/*
Copyright 2021-2022 Red Hat, Inc.

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
	"path/filepath"
	"reflect"
	"time"

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
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/yaml"

	"github.com/devfile/api/v2/pkg/attributes"
	data "github.com/devfile/library/pkg/devfile/parser/data"
	"github.com/go-logr/logr"
	kcpclient "github.com/kcp-dev/apimachinery/pkg/client"
	"github.com/kcp-dev/logicalcluster"
	routev1 "github.com/openshift/api/route/v1"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	appservicegitops "github.com/redhat-appstudio/application-service/gitops"
	"github.com/redhat-appstudio/application-service/gitops/prepare"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	"github.com/redhat-appstudio/application-service/pkg/spi"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	gitopsgen "github.com/redhat-developer/gitops-generator/pkg"

	"github.com/spf13/afero"
)

// ComponentReconciler reconciles a Component object
type ComponentReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Log             logr.Logger
	GitToken        string
	GitHubOrg       string
	ImageRepository string
	Executor        gitopsgen.Executor
	AppFS           afero.Afero
	SPIClient       spi.SPI
}

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
	log := r.Log.WithValues("Component", req.NamespacedName).WithValues("clusterName", req.ClusterName)
	ctx = kcpclient.WithCluster(ctx, logicalcluster.New(req.ClusterName))

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
		log.Error(err, fmt.Sprintf("Unable to get the Application %s, requeueing %v", component.Spec.Application, req.NamespacedName))
		r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
		return ctrl.Result{}, err
	}

	// If the Application CR devfile status is empty, requeue
	if hasApplication.Status.Devfile == "" && !containsString(component.GetFinalizers(), compFinalizerName) {
		log.Error(err, fmt.Sprintf("Application devfile model is empty. Before creating a Component, an instance of Application should be created. Requeueing %v", req.NamespacedName))
		err := fmt.Errorf("application devfile model is empty")
		r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
		return ctrl.Result{}, err
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
			if err := r.Finalize(ctx, &component, &hasApplication); err != nil {
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

	if component.Spec.ContainerImage == "" {
		component.Spec.ContainerImage = r.ImageRepository + ":" + component.Namespace + "-" + component.Name
		if err := r.Client.Update(ctx, &component); err != nil {
			log.Error(err, fmt.Sprintf("Failed to set default component image: %s", component.Spec.ContainerImage))
			return ctrl.Result{}, err
		}
		log.Info(fmt.Sprintf("Set component image to default value: %s", component.Spec.ContainerImage))
		return ctrl.Result{Requeue: true}, nil
	}

	// If the devfile hasn't been populated, the CR was just created
	var gitToken string
	if component.Status.Devfile == "" {

		source := component.Spec.Source

		var compDevfileData data.DevfileData
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

			var devfileBytes []byte
			var gitURL string
			if source.GitSource.DevfileURL == "" && source.GitSource.DockerfileURL == "" {
				if gitToken == "" {
					gitURL, err = util.ConvertGitHubURL(source.GitSource.URL, source.GitSource.Revision)
					if err != nil {
						log.Error(err, fmt.Sprintf("Unable to convert Github URL to raw format, exiting reconcile loop %v", req.NamespacedName))
						r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
						return ctrl.Result{}, err
					}

					// append context to the path if present
					// context is usually set when the git repo is a multi-component repo (example - contains both frontend & backend)
					var devfileDir string
					if context == "" {
						devfileDir = gitURL
					} else {
						devfileDir = gitURL + "/" + context
					}

					devfileBytes, err = devfile.DownloadDevfile(devfileDir)
					if err != nil {
						log.Error(err, fmt.Sprintf("Unable to read the devfile from dir %s %v", devfileDir, req.NamespacedName))
						r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
						return ctrl.Result{}, err
					}
				} else {
					// Use SPI to retrieve the devfile from the private repository
					devfileBytes, err = spi.DownloadDevfileUsingSPI(r.SPIClient, ctx, component.Namespace, source.GitSource.URL, "main", context)
					if err != nil {
						log.Error(err, fmt.Sprintf("Unable to download from any known devfile locations from %s %v", source.GitSource.URL, req.NamespacedName))
						r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
						return ctrl.Result{}, err
					}
				}

			} else if source.GitSource.DockerfileURL != "" {
				devfileData, err := devfile.CreateDevfileForDockerfileBuild(source.GitSource.DockerfileURL, context)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to create devfile for dockerfile build %v", req.NamespacedName))
					r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
					return ctrl.Result{}, err
				}

				devfileBytes, err = yaml.Marshal(devfileData)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to marshall devfile, exiting reconcile loop %v", req.NamespacedName))
					r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
					return ctrl.Result{}, nil
				}
			} else if source.GitSource.DevfileURL != "" {
				devfileBytes, err = util.CurlEndpoint(source.GitSource.DevfileURL)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to GET %s, exiting reconcile loop %v", source.GitSource.DevfileURL, req.NamespacedName))
					err := fmt.Errorf("unable to GET from %s", source.GitSource.DevfileURL)
					r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
					return ctrl.Result{}, err
				}
			}

			// Parse the Component Devfile
			compDevfileData, err = devfile.ParseDevfileModel(string(devfileBytes))
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component, exiting reconcile loop %v", req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}
		} else {
			// An image component was specified
			// Generate a stub devfile for the component
			compDevfileData, err = devfile.ConvertImageComponentToDevfile(component)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to convert the Image Component to a devfile %v", req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}
			component.Status.ContainerImage = component.Spec.ContainerImage
		}

		err = r.updateComponentDevfileModel(req, compDevfileData, component)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to update the Component Devfile model %v", req.NamespacedName))
			r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
			return ctrl.Result{}, err
		}

		if hasApplication.Status.Devfile != "" {
			// Get the devfile of the hasApp CR
			hasAppDevfileData, err := devfile.ParseDevfileModel(hasApplication.Status.Devfile)
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
				if err := r.generateGitops(ctx, req, &component); err != nil {
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
		hasCompDevfileData, err := devfile.ParseDevfileModel(component.Status.Devfile)
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
		oldCompDevfileData, err := devfile.ParseDevfileModel(component.Status.Devfile)
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

			// Generate and push the gitops resources, if necessary.
			component.Status.ContainerImage = component.Spec.ContainerImage
			if !component.Spec.SkipGitOpsResourceGeneration {
				if err := r.generateGitops(ctx, req, &component); err != nil {
					errMsg := fmt.Sprintf("Unable to generate gitops resources for component %v", req.NamespacedName)
					log.Error(err, errMsg)
					r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, &component, fmt.Errorf("%v: %v", errMsg, err))
					r.SetUpdateConditionAndUpdateCR(ctx, req, &component, fmt.Errorf("%v: %v", errMsg, err))
					return ctrl.Result{}, err
				} else {
					r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, &component, nil)
				}
			}

			component.Status.Devfile = string(yamlHASCompData)
			r.SetUpdateConditionAndUpdateCR(ctx, req, &component, nil)

		} else {
			log.Info(fmt.Sprintf("The Component devfile data was not updated %v", req.NamespacedName))
		}
	}

	// Get the Webhook from the event listener route and update it
	// Only attempt to get it if the build generation succeeded, otherwise the route won't exist
	if len(component.Status.Conditions) > 0 && component.Status.Conditions[len(component.Status.Conditions)-1].Status == metav1.ConditionTrue &&
		component.Spec.Source.GitSource != nil && component.Spec.Source.GitSource.URL != "" &&
		(component.ObjectMeta.Annotations == nil || component.ObjectMeta.Annotations[appservicegitops.PaCAnnotation] != "1") {
		createdWebhook := &routev1.Route{}
		err = r.Client.Get(ctx, types.NamespacedName{Name: "el" + component.Name, Namespace: component.Namespace}, createdWebhook)
		if err != nil {
			if errors.IsNotFound(err) {
				log.Error(err, fmt.Sprintf("Unable to fetch the created webhook %v, retrying", "el-"+component.Name))
				return ctrl.Result{Requeue: true}, nil
			} else {
				return ctrl.Result{}, err
			}
		}

		// Get the ingress url from the status of the route, if it exists
		if len(createdWebhook.Status.Ingress) != 0 {
			component.Status.Webhook = createdWebhook.Status.Ingress[0].Host
			r.Client.Status().Update(ctx, &component)
		}
	}

	log.Info(fmt.Sprintf("Finished reconcile loop for %v", req.NamespacedName))
	return ctrl.Result{}, nil
}

// generateGitops retrieves the necessary information about a Component's gitops repository (URL, branch, context)
// and attempts to use the GitOps package to generate gitops resources based on that component
func (r *ComponentReconciler) generateGitops(ctx context.Context, req ctrl.Request, component *appstudiov1alpha1.Component) error {
	log := r.Log.WithValues("Component", req.NamespacedName).WithValues("clusterName", req.ClusterName)

	gitOpsURL, gitOpsBranch, gitOpsContext, err := util.ProcessGitOpsStatus(component.Status.GitOps, r.GitToken)
	if err != nil {
		return err
	}

	// Create a temp folder to create the gitops resources in
	tempDir, err := ioutils.CreateTempPath(component.Name, r.AppFS)
	if err != nil {
		log.Error(err, "unable to create temp directory for gitops resources due to error")
		return fmt.Errorf("unable to create temp directory for gitops resources due to error: %v", err)
	}

	// Generate and push the gitops resources
	gitopsConfig := prepare.PrepareGitopsConfig(ctx, r.Client, *component)
	mappedGitOpsComponent := util.GetMappedGitOpsComponent(*component)
	err = gitopsgen.CloneGenerateAndPush(tempDir, gitOpsURL, mappedGitOpsComponent, r.Executor, r.AppFS, gitOpsBranch, gitOpsContext, false)
	if err != nil {
		gitOpsErr := util.SanitizeErrorMessage(err)
		log.Error(gitOpsErr, "unable to generate gitops resources due to error")
		return gitOpsErr
	}

	err = appservicegitops.GenerateTektonBuild(tempDir, *component, r.AppFS, gitOpsContext, gitopsConfig)
	if err != nil {
		gitOpsErr := util.SanitizeErrorMessage(err)
		log.Error(gitOpsErr, "unable to generate gitops build resources due to error")
		return gitOpsErr
	}
	err = gitopsgen.CommitAndPush(tempDir, "", gitOpsURL, mappedGitOpsComponent.Name, r.Executor, gitOpsBranch, "Generating Tekton resources")
	if err != nil {
		gitOpsErr := util.SanitizeErrorMessage(err)
		log.Error(gitOpsErr, "unable to commit and push gitops resources due to error")
		return gitOpsErr
	}

	// Get the commit ID for the gitops repository
	var commitID string
	repoPath := filepath.Join(tempDir, component.Name)
	if commitID, err = gitopsgen.GetCommitIDFromRepo(r.AppFS, r.Executor, repoPath); err != nil {
		gitOpsErr := util.SanitizeErrorMessage(err)
		log.Error(gitOpsErr, "unable to retrieve gitops repository commit id due to error")
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.Component{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(time.Duration(500*time.Millisecond), time.Duration(60*time.Second)),
		}).
		Complete(r)
}
