/*
Copyright 2021-2023 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redhat-appstudio/application-service/pkg/metrics"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	"github.com/devfile/api/v2/pkg/attributes"
	devfileParser "github.com/devfile/library/v2/pkg/devfile/parser"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	"github.com/go-logr/logr"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	"github.com/redhat-appstudio/application-service/pkg/github"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
	"github.com/redhat-appstudio/application-service/pkg/spi"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	gitopsgen "github.com/redhat-developer/gitops-generator/pkg"
	"github.com/spf13/afero"
)

// ComponentReconciler reconciles a Component object
type ComponentReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Log               logr.Logger
	GitHubOrg         string
	Generator         gitopsgen.Generator
	AppFS             afero.Afero
	SPIClient         spi.SPI
	GitHubTokenClient github.GitHubToken
}

const (
	applicationFailCounterAnnotation = "applicationFailCounter"
	maxApplicationFailCount          = 5
	componentName                    = "Component"
)

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=components,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=components/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=components/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Component object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues(componentName, req.NamespacedName)

	// Fetch the Component instance
	var component appstudiov1alpha1.Component
	err := r.Get(ctx, req.NamespacedName, &component)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Get the Application CR
	hasApplication := appstudiov1alpha1.Application{}
	err = r.Get(ctx, types.NamespacedName{Name: component.Spec.Application, Namespace: component.Namespace}, &hasApplication)
	if err != nil && !containsString(component.GetFinalizers(), compFinalizerName) {
		// only requeue if there is no finalizer attached ie; first time component create
		err = fmt.Errorf("unable to get the Application %s, requeueing %v", component.Spec.Application, req.NamespacedName)
		return r.incrementCounterAndRequeue(log, ctx, req, &component, err)
	}

	// If the Application CR devfile status is empty, requeue
	if hasApplication.Status.Devfile == "" && !containsString(component.GetFinalizers(), compFinalizerName) {
		err = fmt.Errorf("application devfile model is empty. Before creating a Component, an instance of Application should be created %v", req.NamespacedName)
		return r.incrementCounterAndRequeue(log, ctx, req, &component, err)
	}

	setCounterAnnotation(applicationFailCounterAnnotation, &component, 0)

	ghClient, err := r.GitHubTokenClient.GetNewGitHubClient("")
	if err != nil {
		log.Error(err, "Unable to create Go-GitHub client due to error")
		return reconcile.Result{}, err
	}

	// Check if the Component CR is under deletion
	// If so: Remove the project from the Application devfile, remove the component dir from the Gitops repo and remove the finalizer.
	if component.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(component.GetFinalizers(), compFinalizerName) {
			ownerReference := metav1.OwnerReference{
				APIVersion: hasApplication.APIVersion,
				Kind:       hasApplication.Kind,
				Name:       hasApplication.Name,
				UID:        hasApplication.UID,
			}
			component.SetOwnerReferences(append(component.GetOwnerReferences(), ownerReference))

			// Attach the finalizer and return to reset the reconciler loop
			err := r.AddFinalizer(ctx, &component)
			if err != nil {
				return ctrl.Result{}, err
			}
			log.Info(fmt.Sprintf("added the finalizer %v", req.NamespacedName))
		}
	} else {
		if hasApplication.Status.Devfile != "" && len(component.Status.Conditions) > 0 && component.Status.Conditions[len(component.Status.Conditions)-1].Status == metav1.ConditionTrue && containsString(component.GetFinalizers(), compFinalizerName) {
			// only attempt to finalize and update the gitops repo if an Application is present & the previous Component status is good
			// A finalizer is present for the Component CR, so make sure we do the necessary cleanup steps
			if err := r.Finalize(ctx, &component, &hasApplication, ghClient); err != nil {
				// if fail to delete the external dependency here, log the error, but don't return error
				// Don't want to get stuck in a cycle of repeatedly trying to update the repository and failing
				log.Error(err, "Unable to update GitOps repository for component %v in namespace %v", component.GetName(), component.GetNamespace())
			}
		}

		// Remove the finalizer if no Application is present or an Application is present at this stage
		if containsString(component.GetFinalizers(), compFinalizerName) {
			// remove the finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&component, compFinalizerName)
			if err := r.Update(ctx, &component); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	log.Info(fmt.Sprintf("Starting reconcile loop for %v", req.NamespacedName))

	// Check if GitOps generation has failed on a reconcile
	// Attempt to generate GitOps and set appropriate conditions accordingly
	isUpdateConditionPresent := false
	isGitOpsRegenSuccessful := false
	for _, condition := range component.Status.Conditions {
		if condition.Type == "GitOpsResourcesGenerated" && condition.Reason == "GenerateError" && condition.Status == metav1.ConditionFalse {
			log.Info(fmt.Sprintf("Re-attempting GitOps generation for %s", component.Name))
			// Parse the Component Devfile
			devfileSrc := devfile.DevfileSrc{
				Data: component.Status.Devfile,
			}
			compDevfileData, err := devfile.ParseDevfile(devfileSrc)
			if err != nil {
				errMsg := fmt.Sprintf("Unable to parse the devfile from Component status and re-attempt GitOps generation, exiting reconcile loop %v", req.NamespacedName)
				log.Error(err, errMsg)
				r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, &component, fmt.Errorf("%v: %v", errMsg, err))
				return ctrl.Result{}, err
			}
			if err := r.generateGitops(ctx, ghClient, req, &component, compDevfileData); err != nil {
				errMsg := fmt.Sprintf("Unable to generate gitops resources for component %v", req.NamespacedName)
				log.Error(err, errMsg)
				r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, &component, fmt.Errorf("%v: %v", errMsg, err))
				return ctrl.Result{}, err
			} else {
				log.Info(fmt.Sprintf("GitOps re-generation successful for %s", component.Name))
				r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, &component, nil)
				isGitOpsRegenSuccessful = true
			}
		} else if condition.Type == "Updated" && condition.Reason == "Error" && condition.Status == metav1.ConditionFalse {
			isUpdateConditionPresent = true
		}
	}

	if isGitOpsRegenSuccessful && isUpdateConditionPresent {
		r.SetUpdateConditionAndUpdateCR(ctx, req, &component, nil)
		return ctrl.Result{}, nil
	} else if isGitOpsRegenSuccessful {
		r.SetCreateConditionAndUpdateCR(ctx, req, &component, nil)
		return ctrl.Result{}, nil
	}

	// If the devfile hasn't been populated, the CR was just created
	var gitToken string
	if component.Status.Devfile == "" {

		source := component.Spec.Source

		var compDevfileData data.DevfileData
		var devfileLocation string
		var devfileBytes []byte

		if source.GitSource != nil && source.GitSource.URL != "" {
			context := source.GitSource.Context
			// If a Git secret was passed in, retrieve it for use in our Git operations
			// The secret needs to be in the same namespace as the Component
			if component.Spec.Secret != "" {
				gitSecret := corev1.Secret{}
				namespacedName := types.NamespacedName{
					Name:      component.Spec.Secret,
					Namespace: component.Namespace,
				}

				err = r.Client.Get(ctx, namespacedName, &gitSecret)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to retrieve Git secret %v, exiting reconcile loop %v", component.Spec.Secret, req.NamespacedName))
					r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
					return ctrl.Result{}, err
				}

				gitToken = string(gitSecret.Data["password"])
			}

			var gitURL string
			if source.GitSource.DevfileURL == "" && source.GitSource.DockerfileURL == "" {
				if gitToken == "" {
					gitURL, err = util.ConvertGitHubURL(source.GitSource.URL, source.GitSource.Revision, context)
					if err != nil {
						log.Error(err, fmt.Sprintf("Unable to convert Github URL to raw format, exiting reconcile loop %v", req.NamespacedName))
						r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
						return ctrl.Result{}, err
					}

					devfileBytes, devfileLocation, err = devfile.FindAndDownloadDevfile(gitURL)
					if err != nil {
						log.Error(err, fmt.Sprintf("Unable to read the devfile from dir %s %v", gitURL, req.NamespacedName))
						r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
						return ctrl.Result{}, err
					}

					devfileLocation = gitURL + string(os.PathSeparator) + devfileLocation
				} else {
					// Use SPI to retrieve the devfile from the private repository
					devfileBytes, err = spi.DownloadDevfileUsingSPI(r.SPIClient, ctx, component.Namespace, source.GitSource.URL, "main", context)
					if err != nil {
						log.Error(err, fmt.Sprintf("Unable to download from any known devfile locations from %s %v", source.GitSource.URL, req.NamespacedName))
						r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
						return ctrl.Result{}, err
					}
				}

			} else if source.GitSource.DevfileURL != "" {
				devfileLocation = source.GitSource.DevfileURL
				devfileBytes, err = util.CurlEndpoint(source.GitSource.DevfileURL)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to GET %s, exiting reconcile loop %v", source.GitSource.DevfileURL, req.NamespacedName))
					err := fmt.Errorf("unable to GET from %s", source.GitSource.DevfileURL)
					r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
					return ctrl.Result{}, err
				}
			} else if source.GitSource.DockerfileURL != "" {
				devfileData, err := devfile.CreateDevfileForDockerfileBuild(source.GitSource.DockerfileURL, "./", component.Name, component.Spec.Application, component.Namespace)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to create devfile for dockerfile build %v", req.NamespacedName))
					r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
					return ctrl.Result{}, err
				}

				devfileBytes, err = yaml.Marshal(devfileData)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to marshal devfile, exiting reconcile loop %v", req.NamespacedName))
					r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
					return ctrl.Result{}, nil
				}
			}
		} else {
			// An image component was specified
			// Generate a stub devfile for the component
			devfileData, err := devfile.ConvertImageComponentToDevfile(component)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to convert the Image Component to a devfile %v", req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}
			component.Status.ContainerImage = component.Spec.ContainerImage

			devfileBytes, err = yaml.Marshal(devfileData)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to marshal devfile, exiting reconcile loop %v", req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, nil
			}
		}

		if devfileLocation != "" {
			// Parse the Component Devfile
			devfileSrc := devfile.DevfileSrc{
				URL: devfileLocation,
			}
			compDevfileData, err = devfile.ParseDevfile(devfileSrc)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component devfile location, exiting reconcile loop %v", req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}
		} else {
			// Parse the Component Devfile
			devfileSrc := devfile.DevfileSrc{
				Data: string(devfileBytes),
			}
			compDevfileData, err = devfile.ParseDevfile(devfileSrc)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component, exiting reconcile loop %v", req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}
		}

		err = r.updateComponentDevfileModel(req, compDevfileData, component)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to update the Component Devfile model %v", req.NamespacedName))
			r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
			return ctrl.Result{}, err
		}

		if hasApplication.Status.Devfile != "" {
			// Get the devfile of the hasApp CR
			devfileSrc := devfile.DevfileSrc{
				Data: hasApplication.Status.Devfile,
			}
			hasAppDevfileData, err := devfile.ParseDevfile(devfileSrc)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to parse the devfile from Application, exiting reconcile loop %v", req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}

			err = r.updateApplicationDevfileModel(hasAppDevfileData, component)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to update the HAS Application Devfile model %v", req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}

			yamlHASCompData, err := yaml.Marshal(compDevfileData)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to marshall the Component devfile, exiting reconcile loop %v", req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}

			component.Status.Devfile = string(yamlHASCompData)

			// Update the HASApp CR with the new devfile
			yamlHASAppData, err := yaml.Marshal(hasAppDevfileData)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to marshall the Application devfile, exiting reconcile loop %v", req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}
			hasApplication.Status.Devfile = string(yamlHASAppData)
			err = r.Status().Update(ctx, &hasApplication)
			if err != nil {
				log.Error(err, "Unable to update Application")
				// if we're unable to update the Application CR, then  we need to err out
				// since we need to save a reference of the Component in Application
				r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}

			// Set the container image in the status
			component.Status.ContainerImage = component.Spec.ContainerImage

			log.Info(fmt.Sprintf("Adding the GitOps repository information to the status for component %v", req.NamespacedName))
			err = setGitopsStatus(&component, hasAppDevfileData)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to retrieve gitops repository information for resource %v", req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}

			// Generate and push the gitops resources
			if !component.Spec.SkipGitOpsResourceGeneration {
				if err := r.generateGitops(ctx, ghClient, req, &component, compDevfileData); err != nil {
					errMsg := fmt.Sprintf("Unable to generate gitops resources for component %v", req.NamespacedName)
					log.Error(err, errMsg)
					r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, &component, fmt.Errorf("%v: %v", errMsg, err))
					r.SetCreateConditionAndUpdateCR(ctx, req, &component, fmt.Errorf("%v: %v", errMsg, err))
					return ctrl.Result{}, err
				} else {
					r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, &component, nil)
				}
			}

			r.SetCreateConditionAndUpdateCR(ctx, req, &component, nil)
		}
	} else {

		// If the model already exists, see if fields have been updated
		log.Info(fmt.Sprintf("Checking if the Component has been updated %v", req.NamespacedName))

		// Parse the Component Devfile
		devfileSrc := devfile.DevfileSrc{
			Data: component.Status.Devfile,
		}
		hasCompDevfileData, err := devfile.ParseDevfile(devfileSrc)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component status, exiting reconcile loop %v", req.NamespacedName))
			r.SetUpdateConditionAndUpdateCR(ctx, req, &component, err)
			return ctrl.Result{}, err
		}

		err = r.updateComponentDevfileModel(req, hasCompDevfileData, component)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to update the Component Devfile model %v", req.NamespacedName))
			r.SetUpdateConditionAndUpdateCR(ctx, req, &component, err)
			return ctrl.Result{}, err
		}

		// Read the devfile again to compare it with any updates
		devfileSrc = devfile.DevfileSrc{
			Data: component.Status.Devfile,
		}
		oldCompDevfileData, err := devfile.ParseDevfile(devfileSrc)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component status, exiting reconcile loop %v", req.NamespacedName))
			r.SetUpdateConditionAndUpdateCR(ctx, req, &component, err)
			return ctrl.Result{}, err
		}

		containerImage := component.Spec.ContainerImage
		skipGitOpsGeneration := component.Spec.SkipGitOpsResourceGeneration
		isUpdated := !reflect.DeepEqual(oldCompDevfileData, hasCompDevfileData) || containerImage != component.Status.ContainerImage || skipGitOpsGeneration != component.Status.GitOps.ResourceGenerationSkipped
		if isUpdated {
			log.Info(fmt.Sprintf("The Component was updated %v", req.NamespacedName))
			component.Status.GitOps.ResourceGenerationSkipped = skipGitOpsGeneration
			yamlHASCompData, err := yaml.Marshal(hasCompDevfileData)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to marshall the Component devfile, exiting reconcile loop %v", req.NamespacedName))
				r.SetUpdateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}

			component.Status.ContainerImage = component.Spec.ContainerImage
			component.Status.Devfile = string(yamlHASCompData)
			err = r.Client.Status().Update(ctx, &component)
			if err != nil {
				log.Error(err, "Unable to update Component status")
				// if we're unable to update the Component CR status, then we need to err out
				// since we need the reference of the devfile in Component to be always accessible
				r.SetUpdateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}

			// Generate and push the gitops resources, if necessary.
			if !component.Spec.SkipGitOpsResourceGeneration {
				if err := r.generateGitops(ctx, ghClient, req, &component, hasCompDevfileData); err != nil {
					errMsg := fmt.Sprintf("Unable to generate gitops resources for component %v", req.NamespacedName)
					log.Error(err, errMsg)
					r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, &component, fmt.Errorf("%v: %v", errMsg, err))
					r.SetUpdateConditionAndUpdateCR(ctx, req, &component, fmt.Errorf("%v: %v", errMsg, err))
					return ctrl.Result{}, err
				} else {
					r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, &component, nil)
				}
			}
			r.SetUpdateConditionAndUpdateCR(ctx, req, &component, nil)

		} else {
			log.Info(fmt.Sprintf("The Component devfile data was not updated %v", req.NamespacedName))
		}
	}

	log.Info(fmt.Sprintf("Finished reconcile loop for %v", req.NamespacedName))
	return ctrl.Result{}, nil
}

// generateGitops retrieves the necessary information about a Component's gitops repository (URL, branch, context)
// and attempts to use the GitOps package to generate gitops resources based on that component
func (r *ComponentReconciler) generateGitops(ctx context.Context, ghClient github.GitHubClient, req ctrl.Request, component *appstudiov1alpha1.Component, compDevfileData data.DevfileData) error {
	log := r.Log.WithValues("Component", req.NamespacedName)

	gitOpsURL, gitOpsBranch, gitOpsContext, err := util.ProcessGitOpsStatus(component.Status.GitOps, ghClient.Token)
	if err != nil {
		return err
	}

	// Create a temp folder to create the gitops resources in
	tempDir, err := ioutils.CreateTempPath(component.Name, r.AppFS)
	if err != nil {
		log.Error(err, "unable to create temp directory for GitOps resources due to error")
		return fmt.Errorf("unable to create temp directory for GitOps resources due to error: %v", err)
	}

	deployAssociatedComponents, err := devfileParser.GetDeployComponents(compDevfileData)
	if err != nil {
		log.Error(err, "unable to get deploy components")
		return err
	}

	kubernetesResources, err := devfile.GetResourceFromDevfile(log, compDevfileData, deployAssociatedComponents, component.Name, component.Spec.Application, component.Spec.ContainerImage, component.Namespace)
	if err != nil {
		log.Error(err, "unable to get kubernetes resources from the devfile outerloop components")
		return err
	}

	// Generate and push the gitops resources
	mappedGitOpsComponent := util.GetMappedGitOpsComponent(*component, kubernetesResources)

	//add the token name to the metrics.  When we add more tokens and rotate, we can determine how evenly distributed the requests are
	metrics.ControllerGitRequest.With(prometheus.Labels{"controller": componentName, "tokenName": ghClient.TokenName, "operation": "CloneGenerateAndPush"}).Inc()
	err = r.Generator.CloneGenerateAndPush(tempDir, gitOpsURL, mappedGitOpsComponent, r.AppFS, gitOpsBranch, gitOpsContext, false)
	if err != nil {
		log.Error(err, "unable to generate gitops resources due to error")
		return err
	}

	//Gitops functions return sanitized error messages
	metrics.ControllerGitRequest.With(prometheus.Labels{"controller": componentName, "tokenName": ghClient.TokenName, "operation": "CommitAndPush"}).Inc()
	err = r.Generator.CommitAndPush(tempDir, "", gitOpsURL, mappedGitOpsComponent.Name, gitOpsBranch, "Generating GitOps resources")
	if err != nil {
		log.Error(err, "unable to commit and push gitops resources due to error")
		return err
	}

	// Get the commit ID for the gitops repository
	var commitID string
	repoName, orgName, err := github.GetRepoAndOrgFromURL(gitOpsURL)
	if err != nil {
		gitOpsErr := &GitOpsParseRepoError{gitOpsURL, err}
		log.Error(gitOpsErr, "")
		return gitOpsErr
	}

	metricsLabel := prometheus.Labels{"controller": componentName, "tokenName": ghClient.TokenName, "operation": "GetLatestCommitSHAFromRepository"}
	metrics.ControllerGitRequest.With(metricsLabel).Inc()
	commitID, err = ghClient.GetLatestCommitSHAFromRepository(ctx, repoName, orgName, gitOpsBranch)
	metrics.HandleRateLimitMetrics(err, metricsLabel)
	if err != nil {
		gitOpsErr := &GitOpsCommitIdError{err}
		log.Error(gitOpsErr, "")
		return gitOpsErr
	}
	component.Status.GitOps.CommitID = commitID

	// Remove the temp folder that was created
	return r.AppFS.RemoveAll(tempDir)
}

// setGitopsStatus adds the necessary gitops info (url, branch, context) to the component CR status
func setGitopsStatus(component *appstudiov1alpha1.Component, devfileData data.DevfileData) error {
	var err error
	devfileAttributes := devfileData.GetMetadata().Attributes

	// Get the GitOps repository URL
	gitOpsURL := devfileAttributes.GetString("gitOpsRepository.url", &err)
	if err != nil {
		return fmt.Errorf("unable to retrieve GitOps repository from Application CR devfile: %v", err)
	}
	component.Status.GitOps.RepositoryURL = gitOpsURL

	// Get the GitOps repository branch
	gitOpsBranch := devfileAttributes.GetString("gitOpsRepository.branch", &err)
	if err != nil {
		if _, ok := err.(*attributes.KeyNotFoundError); !ok {
			return err
		}
	}
	if gitOpsBranch != "" {
		component.Status.GitOps.Branch = gitOpsBranch
	}

	// Get the GitOps repository context
	gitOpsContext := devfileAttributes.GetString("gitOpsRepository.context", &err)
	if err != nil {
		if _, ok := err.(*attributes.KeyNotFoundError); !ok {
			return err
		}
	}
	if gitOpsContext != "" {
		component.Status.GitOps.Context = gitOpsContext
	}

	component.Status.GitOps.ResourceGenerationSkipped = component.Spec.SkipGitOpsResourceGeneration
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := zap.Options{
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log.WithName("controllers").WithName("Component").WithValues("appstudio-component", "HAS")
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.Component{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(time.Duration(500*time.Millisecond), time.Duration(1000*time.Second)),
		}).WithEventFilter(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			log := log.WithValues("Namespace", e.Object.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "Component", logutil.ResourceCreate, nil)
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := log.WithValues("Namespace", e.ObjectNew.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "Component", logutil.ResourceUpdate, nil)
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log := log.WithValues("Namespace", e.Object.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "Component", logutil.ResourceDelete, nil)
			return false
		},
	}).
		Complete(r)
}

// incrementCounterAndRequeue will increment the "application error counter" on the Component resource and requeue
// If the counter is less than 3, the Component will be requeued (with a half second delay) without any error message returned
// If the counter is greater than or equal to 3, an error message will be set on the Component's status and it will be requeud
// 3 attemps were chosen along with the half second requeue delay to allow certain transient errors when the application CR isn't ready, to resolve themself.
func (r *ComponentReconciler) incrementCounterAndRequeue(log logr.Logger, ctx context.Context, req ctrl.Request, component *appstudiov1alpha1.Component, componentErr error) (ctrl.Result, error) {
	if component.GetAnnotations() == nil {
		component.ObjectMeta.Annotations = make(map[string]string)
	}
	count, err := getCounterAnnotation(applicationFailCounterAnnotation, component)
	if count > 2 || err != nil {
		log.Error(err, "")
		r.SetCreateConditionAndUpdateCR(ctx, req, component, componentErr)
		return ctrl.Result{}, componentErr
	} else {
		setCounterAnnotation(applicationFailCounterAnnotation, component, count+1)
		err = r.Update(ctx, component)
		if err != nil {
			log.Error(err, "error updating component's counter annotation")
		}
		return ctrl.Result{}, componentErr
	}
}
