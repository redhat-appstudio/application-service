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
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/prometheus/client_golang/prometheus"
	cdqanalysis "github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	"github.com/redhat-appstudio/application-service/pkg/metrics"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	"github.com/devfile/api/v2/pkg/attributes"
	devfileParser "github.com/devfile/library/v2/pkg/devfile/parser"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	parserErrPkg "github.com/devfile/library/v2/pkg/devfile/parser/errors"
	devfileParserUtil "github.com/devfile/library/v2/pkg/devfile/parser/util"
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
	Scheme             *runtime.Scheme
	Log                logr.Logger
	GitHubOrg          string
	Generator          gitopsgen.Generator
	AppFS              afero.Afero
	SPIClient          spi.SPI
	GitHubTokenClient  github.GitHubToken
	DevfileUtilsClient devfileParserUtil.DevfileUtils
}

const (
	applicationFailCounterAnnotation = "applicationFailCounter"
	maxApplicationFailCount          = 5
	componentName                    = "Component"
	forceGenerationAnnotation        = "forceGitopsGeneration"
)

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=components,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=components/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=components/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spifilecontentrequests,verbs=get;list;create
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spifilecontentrequests/status,verbs=get
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=spifilecontentrequests/finalizers,verbs=update

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
	log := ctrl.LoggerFrom(ctx)

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

	isCreateReconcile, prevErrCondition := checkForCreateReconcile(component)

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
	forceGenerateGitopsResource := getForceGenerateGitopsAnnotation(component)
	log.Info(fmt.Sprintf("forceGenerateGitopsResource is %v", forceGenerateGitopsResource))

	var gitToken string
	//get the token to pass into the parser
	if component.Spec.Secret != "" {
		gitSecret := corev1.Secret{}
		namespacedName := types.NamespacedName{
			Name:      component.Spec.Secret,
			Namespace: component.Namespace,
		}

		err = r.Client.Get(ctx, namespacedName, &gitSecret)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to retrieve Git secret %v, exiting reconcile loop %v", component.Spec.Secret, req.NamespacedName))
			_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
			return ctrl.Result{}, err
		}

		gitToken = string(gitSecret.Data["password"])
	}

	ghClient, err := r.GitHubTokenClient.GetNewGitHubClient(gitToken)
	if err != nil {
		log.Error(err, "Unable to create Go-GitHub client due to error")
		return reconcile.Result{}, err
	}

	// Add the Go-GitHub client name to the context
	ctx = context.WithValue(ctx, github.GHClientKey, ghClient.TokenName)

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
		if hasApplication.Status.Devfile != "" && (forceGenerateGitopsResource || len(component.Status.Conditions) > 0 && component.Status.Conditions[len(component.Status.Conditions)-1].Status == metav1.ConditionTrue && containsString(component.GetFinalizers(), compFinalizerName)) {
			// only attempt to finalize and update the gitops repo if an Application is present & the previous Component status is good
			// A finalizer is present for the Component CR, so make sure we do the necessary cleanup steps
			metrics.ComponentDeletionTotalReqs.Inc()
			if err := r.Finalize(ctx, &component, &hasApplication, ghClient, gitToken); err != nil {
				if errors.IsConflict(err) {
					//conflict means we just retry, we are updating the shared application so conflicts are not unexpected
					return ctrl.Result{}, err
				}
				// if fail to delete the external dependency here, log the error, but don't return error
				// Don't want to get stuck in a cycle of repeatedly trying to update the repository and failing
				log.Error(err, fmt.Sprintf("Unable to update GitOps repository for component %v in namespace %v", component.GetName(), component.GetNamespace()))

				// Increment the Component deletion failed metric as the component delete did not fully succeed
				metrics.ComponentDeletionFailed.Inc()
			}
		}

		// Remove the finalizer if no Application is present or an Application is present at this stage
		if containsString(component.GetFinalizers(), compFinalizerName) {
			// remove the finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&component, compFinalizerName)
			if err := r.Update(ctx, &component); err != nil {
				return ctrl.Result{}, err
			} else {
				metrics.ComponentDeletionSucceeded.Inc()
			}
		}
	}

	log.Info(fmt.Sprintf("Starting reconcile loop for %v", req.NamespacedName))

	// Check if GitOps generation has failed on a reconcile
	// Attempt to generate GitOps and set appropriate conditions accordingly
	isUpdateErrConditionPresent := false
	isGitOpsRegenSuccessful := false
	for _, condition := range component.Status.Conditions {
		if forceGenerateGitopsResource || (condition.Type == "GitOpsResourcesGenerated" && condition.Reason == "GenerateError" && condition.Status == metav1.ConditionFalse) {
			log.Info(fmt.Sprintf("Re-attempting GitOps generation for %s", component.Name))
			// Parse the Component CR Devfile
			// Not necessary to pass in a Token or DevfileUtils client to the parser here since the devfileBytes has:
			// 1. Already been flattened on the create reconcile, so private parents are already expanded
			// 2. Kubernetes Component Uri has already been converted to inlined content with a Token if required by default on the first parse
			compDevfileData, err := cdqanalysis.ParseDevfileWithParserArgs(&devfileParser.ParserArgs{Data: []byte(component.Status.Devfile)})
			if err != nil {
				if err != nil {
					if _, ok := err.(*parserErrPkg.NonCompliantDevfile); ok {
						if isCreateReconcile {
							// Gate it with a Create reconcile flag check since this code is executed by both Create and Update reconciliation
							// user error in devfile, increment success metric
							metrics.IncrementComponentCreationSucceeded(prevErrCondition, err.Error())
						}
					} else {
						if isCreateReconcile {
							// Gate it with a Create reconcile flag check since this code is executed by both Create and Update reconciliation
							// not a user error, increment fail metric
							metrics.IncrementComponentCreationFailed(prevErrCondition, err.Error())
						}
					}
				}
				errMsg := fmt.Sprintf("Unable to parse the devfile from Component status and re-attempt GitOps generation, exiting reconcile loop %v", req.NamespacedName)
				log.Error(err, errMsg)
				_ = r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, req, &component, fmt.Errorf("%v: %v", errMsg, err))
				return ctrl.Result{}, err
			}
			if err := r.generateGitops(ctx, ghClient, &component, compDevfileData); err != nil {
				errMsg := fmt.Sprintf("Unable to generate gitops resources for component %v", req.NamespacedName)
				log.Error(err, errMsg)
				metrics.IncrementComponentCreationSucceeded(prevErrCondition, err.Error()) // We are not tracking Component failures for GitOps due to future planning
				_ = r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, req, &component, fmt.Errorf("%v: %v", errMsg, err))
				return ctrl.Result{}, err
			} else {
				log.Info(fmt.Sprintf("GitOps re-generation successful for %s", component.Name))
				err := r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, req, &component, nil)
				if err != nil {
					return ctrl.Result{}, err
				}
				setForceGenerateGitopsAnnotation(&component, "false")
				isGitOpsRegenSuccessful = true
			}
		} else if condition.Type == "Updated" && condition.Reason == "Error" && condition.Status == metav1.ConditionFalse {
			isUpdateErrConditionPresent = true
		}
	}

	if isGitOpsRegenSuccessful && isUpdateErrConditionPresent {
		err = r.SetUpdateConditionAndUpdateCR(ctx, req, &component, nil)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if isGitOpsRegenSuccessful {
		err = r.SetCreateConditionAndUpdateCR(ctx, req, &component, nil)
		if err != nil {
			return ctrl.Result{}, err
		}
		metrics.IncrementComponentCreationSucceeded("", "")
		return ctrl.Result{}, nil
	}

	// If the devfile hasn't been populated, the CR was just created
	if component.Status.Devfile == "" {

		source := component.Spec.Source

		var compDevfileData data.DevfileData
		var devfileLocation string
		var devfileBytes []byte

		if source.GitSource != nil && source.GitSource.URL != "" {
			if err := validateGithubURL(source.GitSource.URL); err != nil {
				// User error - the git url provided is not from github
				log.Error(err, "unable to validate github url")
				metrics.IncrementComponentCreationSucceeded("", "")
				return ctrl.Result{}, err
			}
			context := source.GitSource.Context
			// If a Git secret was passed in, retrieve it for use in our Git operations
			// The secret needs to be in the same namespace as the Component

			if source.GitSource.Revision == "" {
				sourceURL := source.GitSource.URL
				// If the repository URL ends in a forward slash, remove it to avoid issues with default branch lookup
				if string(sourceURL[len(sourceURL)-1]) == "/" {
					sourceURL = sourceURL[0 : len(sourceURL)-1]
				}
				log.Info(fmt.Sprintf("Looking for the default branch of the repo %s... %v", source.GitSource.URL, req.NamespacedName))
				metricsLabel := prometheus.Labels{"controller": cdqName, "tokenName": ghClient.TokenName, "operation": "GetDefaultBranchFromURL"}
				metrics.ControllerGitRequest.With(metricsLabel).Inc()
				source.GitSource.Revision, err = ghClient.GetDefaultBranchFromURL(sourceURL, ctx)
				metrics.HandleRateLimitMetrics(err, metricsLabel)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to get default branch of Github Repo %v, try to fall back to main branch... %v", source.GitSource.URL, req.NamespacedName))
					metricsLabel := prometheus.Labels{"controller": cdqName, "tokenName": ghClient.TokenName, "operation": "GetBranchFromURL"}
					metrics.ControllerGitRequest.With(metricsLabel).Inc()
					_, err := ghClient.GetBranchFromURL(sourceURL, ctx, "main")
					if err != nil {
						metrics.HandleRateLimitMetrics(err, metricsLabel)
						if _, ok := err.(*github.GitHubUserErr); ok {
							// User error, so increment the "success" metric since we're tracking only system errors
							metrics.IncrementComponentCreationSucceeded(prevErrCondition, err.Error())
						} else {
							// Not a user error, increment fail metric
							metrics.IncrementComponentCreationFailed(prevErrCondition, err.Error())
						}
						log.Error(err, fmt.Sprintf("Unable to get main branch of Github Repo %v ... %v", source.GitSource.URL, req.NamespacedName))
						retErr := fmt.Errorf("unable to get default branch of Github Repo %v, try to fall back to main branch, failed to get main branch... %v", source.GitSource.URL, req.NamespacedName)
						_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, retErr)
						return ctrl.Result{}, retErr
					} else {
						source.GitSource.Revision = "main"
					}
				}
			}

			var gitURL string
			if source.GitSource.DevfileURL == "" && source.GitSource.DockerfileURL == "" {
				metrics.ImportGitRepoTotalReqs.Inc()

				if gitToken == "" {
					gitURL, err = cdqanalysis.ConvertGitHubURL(source.GitSource.URL, source.GitSource.Revision, context)
					if err != nil {
						// ConvertGitHubURL only returns user error
						metrics.ImportGitRepoSucceeded.Inc()
						metrics.IncrementComponentCreationSucceeded(prevErrCondition, err.Error())
						log.Error(err, fmt.Sprintf("Unable to convert Github URL to raw format, exiting reconcile loop %v", req.NamespacedName))
						_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
						return ctrl.Result{}, err
					}

					devfileBytes, devfileLocation, err = devfile.FindAndDownloadDevfile(gitURL, gitToken)
					// FindAndDownloadDevfile only returns user error
					metrics.ImportGitRepoSucceeded.Inc()
					if err != nil {
						metrics.IncrementComponentCreationSucceeded(prevErrCondition, err.Error())
						log.Error(err, fmt.Sprintf("Unable to read the devfile from dir %s %v", gitURL, req.NamespacedName))
						_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
						return ctrl.Result{}, err
					}

					devfileLocation = gitURL + string(os.PathSeparator) + devfileLocation
				} else {
					//cannot use converted URLs in SPI because it's not supported.  Need to convert later for parsing
					gitURL = source.GitSource.URL
					// Use SPI to retrieve the devfile from the private repository
					devfileBytes, devfileLocation, err = spi.DownloadDevfileUsingSPI(r.SPIClient, ctx, component, gitURL, source.GitSource.Revision, context)
					if err != nil {
						// Increment the import git repo and component create failed metric on non-user errors
						if _, ok := err.(*cdqanalysis.NoDevfileFound); !ok {
							metrics.ImportGitRepoFailed.Inc()
							metrics.IncrementComponentCreationFailed(prevErrCondition, err.Error())
						} else {
							metrics.ImportGitRepoSucceeded.Inc()
							metrics.IncrementComponentCreationSucceeded(prevErrCondition, err.Error())
						}
						log.Error(err, fmt.Sprintf("Unable to download from any known devfile locations from %s %v", gitURL, req.NamespacedName))
						_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
						return ctrl.Result{}, err
					}
					metrics.ImportGitRepoSucceeded.Inc()

					gitURL, err := cdqanalysis.ConvertGitHubURL(source.GitSource.URL, source.GitSource.Revision, context)
					if err != nil {
						// User error - so increment the "success" metric - since we're tracking only system errors
						metrics.IncrementComponentCreationSucceeded(prevErrCondition, err.Error())
						log.Error(err, fmt.Sprintf("Unable to convert Github URL to raw format, exiting reconcile loop %v", req.NamespacedName))
						_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
						return ctrl.Result{}, err
					}
					devfileLocation = gitURL + string(os.PathSeparator) + devfileLocation
				}

			} else if source.GitSource.DevfileURL != "" {
				devfileLocation = source.GitSource.DevfileURL
			} else if source.GitSource.DockerfileURL != "" {
				devfileData, err := devfile.CreateDevfileForDockerfileBuild(source.GitSource.DockerfileURL, "./", component.Name, component.Spec.Application)
				if err != nil {
					metrics.IncrementComponentCreationFailed(prevErrCondition, err.Error())
					log.Error(err, fmt.Sprintf("Unable to create devfile for Dockerfile build %v", req.NamespacedName))
					_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
					return ctrl.Result{}, err
				}

				devfileBytes, err = yaml.Marshal(devfileData)
				if err != nil {
					metrics.IncrementComponentCreationFailed(prevErrCondition, err.Error())
					log.Error(err, fmt.Sprintf("Unable to marshal devfile, exiting reconcile loop %v", req.NamespacedName))
					err = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
					if err != nil {
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, nil
				}
			}
		} else {
			// An image component was specified
			// Generate a stub devfile for the component
			devfileData, err := devfile.ConvertImageComponentToDevfile(component)
			if err != nil {
				metrics.IncrementComponentCreationFailed(prevErrCondition, err.Error())
				log.Error(err, fmt.Sprintf("Unable to convert the Image Component to a devfile %v", req.NamespacedName))
				_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}
			component.Status.ContainerImage = component.Spec.ContainerImage

			devfileBytes, err = yaml.Marshal(devfileData)
			if err != nil {
				metrics.IncrementComponentCreationFailed(prevErrCondition, err.Error())
				log.Error(err, fmt.Sprintf("Unable to marshal devfile, exiting reconcile loop %v", req.NamespacedName))
				err = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				if err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
		}

		if devfileLocation != "" {
			log.Info(fmt.Sprintf("Parsing Devfile from the Devfile location %s... %v", devfileLocation, req.NamespacedName))
			// Parse the Component CR Devfile
			// Pass in a Token and a DevfileUtils client because we need to
			// 1. Flatten the Devfile and access a private parent if necessary
			// 2. Convert the Kubernetes Uri to Inline by default
			// 3. Provide a way to mock output for Component controller tests
			compDevfileData, err = cdqanalysis.ParseDevfileWithParserArgs(&devfileParser.ParserArgs{URL: devfileLocation, Token: gitToken, DevfileUtilsClient: r.DevfileUtilsClient})
			if err != nil {
				if _, ok := err.(*parserErrPkg.NonCompliantDevfile); ok {
					// user error in devfile, increment success metric
					metrics.IncrementComponentCreationSucceeded(prevErrCondition, err.Error())
				} else {
					// not a user error, increment fail metric
					metrics.IncrementComponentCreationFailed(prevErrCondition, err.Error())
				}
				log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component devfile location, exiting reconcile loop %v", req.NamespacedName))
				_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}
		} else {
			log.Info(fmt.Sprintf("Parsing Devfile from the Devfile bytes %v... %v", len(devfileBytes), req.NamespacedName))
			// Parse the Component CR Devfile
			// Not necessary to pass in a Token or a DevfileUtils client to the parser here on devfileBytes, since:
			// 1. devfileBytes are only used from a DockerfileURL or an Image, Component CR source (check if conditions above on Component CR sources)
			// 2. We dont access any resources for either of these cases in devfile/library
			compDevfileData, err = cdqanalysis.ParseDevfileWithParserArgs(&devfileParser.ParserArgs{Data: devfileBytes})
			if err != nil {
				if _, ok := err.(*parserErrPkg.NonCompliantDevfile); ok {
					// user error in devfile, increment success metric
					metrics.IncrementComponentCreationSucceeded(prevErrCondition, err.Error())
				} else {
					// not a user error, increment fail metric
					metrics.IncrementComponentCreationFailed(prevErrCondition, err.Error())
				}
				log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component, exiting reconcile loop %v", req.NamespacedName))
				_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}
		}

		err = r.updateComponentDevfileModel(req, compDevfileData, component)
		if err != nil {
			// Increment the Component create failed metric only on non-user errors
			if _, ok := err.(*NotSupported); ok {
				metrics.IncrementComponentCreationSucceeded(prevErrCondition, err.Error())
			} else if _, ok := err.(*devfile.DevfileAttributeParse); ok {
				metrics.IncrementComponentCreationSucceeded(prevErrCondition, err.Error())
			} else if _, ok := err.(*parserErrPkg.NonCompliantDevfile); ok {
				metrics.IncrementComponentCreationSucceeded(prevErrCondition, err.Error())
			} else {
				metrics.IncrementComponentCreationFailed(prevErrCondition, err.Error())
			}
			log.Error(err, fmt.Sprintf("Unable to update the Component Devfile model %v", req.NamespacedName))
			_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
			return ctrl.Result{}, err
		}

		if hasApplication.Status.Devfile != "" {
			// Parse the Application CR Devfile
			// No need to invoke devfile/library parser with a Token or a DevfileUtils client because the Application CR Devfile model:
			// 1. Is constructed in the Application controller and there is no need for a Token
			// 2. Only consists of Devfile metadata attributes and projects to store the Component CR information
			hasAppDevfileData, err := cdqanalysis.ParseDevfileWithParserArgs(&devfileParser.ParserArgs{Data: []byte(hasApplication.Status.Devfile)})
			if err != nil {
				// not a user error, increment fail metric
				metrics.IncrementComponentCreationFailed(prevErrCondition, err.Error())
				log.Error(err, fmt.Sprintf("Unable to parse the devfile from Application, exiting reconcile loop %v", req.NamespacedName))
				_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}

			yamlHASCompData, err := yaml.Marshal(compDevfileData)
			if err != nil {
				metrics.IncrementComponentCreationFailed(prevErrCondition, err.Error())
				log.Error(err, fmt.Sprintf("Unable to marshall the Component devfile, exiting reconcile loop %v", req.NamespacedName))
				_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}

			component.Status.Devfile = string(yamlHASCompData)

			// Set the container image in the status
			component.Status.ContainerImage = component.Spec.ContainerImage

			log.Info(fmt.Sprintf("Adding the GitOps repository information to the status for component %v", req.NamespacedName))
			err = setGitopsStatus(&component, hasAppDevfileData)
			if err != nil {
				metrics.IncrementComponentCreationFailed(prevErrCondition, err.Error())
				log.Error(err, fmt.Sprintf("Unable to retrieve gitops repository information for resource %v", req.NamespacedName))
				_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}

			// Generate and push the gitops resources
			if !component.Spec.SkipGitOpsResourceGeneration {
				if err := r.generateGitops(ctx, ghClient, &component, compDevfileData); err != nil {
					errMsg := fmt.Sprintf("Unable to generate gitops resources for component %v", req.NamespacedName)
					log.Error(err, errMsg)
					metrics.IncrementComponentCreationSucceeded(prevErrCondition, err.Error()) // We are not tracking Component failures for GitOps due to future planning
					_ = r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, req, &component, fmt.Errorf("%v: %v", errMsg, err))
					_ = r.SetCreateConditionAndUpdateCR(ctx, req, &component, fmt.Errorf("%v: %v", errMsg, err))
					return ctrl.Result{}, err
				} else {
					err = r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, req, &component, nil)
					if err != nil {
						return ctrl.Result{}, err
					}
				}
			}

			err = r.SetCreateConditionAndUpdateCR(ctx, req, &component, nil)
			if err != nil {
				return ctrl.Result{}, err
			}
			metrics.IncrementComponentCreationSucceeded("", "")
		}
	} else {

		// If the model already exists, see if fields have been updated
		log.Info(fmt.Sprintf("Checking if the Component has been updated %v", req.NamespacedName))

		// Parse the Component CR Devfile
		// Not necessary to pass in a Token or DevfileUtils client to the parser here since the devfileBytes has:
		// 1. Already been flattened on the create reconcile, so private parents are already expanded
		// 2. Kubernetes Component Uri has already been converted to inlined content with a Token if required by default on the first parse
		hasCompDevfileData, err := cdqanalysis.ParseDevfileWithParserArgs(&devfileParser.ParserArgs{Data: []byte(component.Status.Devfile)})

		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component status, exiting reconcile loop %v", req.NamespacedName))
			_ = r.SetUpdateConditionAndUpdateCR(ctx, req, &component, err)
			return ctrl.Result{}, err
		}

		err = r.updateComponentDevfileModel(req, hasCompDevfileData, component)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to update the Component Devfile model %v", req.NamespacedName))
			_ = r.SetUpdateConditionAndUpdateCR(ctx, req, &component, err)
			return ctrl.Result{}, err
		}

		// Parse the Component CR Devfile again to compare it with any updates
		// Not necessary to pass in a Token or DevfileUtils client to the parser here since the devfileBytes has:
		// 1. Already been flattened on the create reconcile, so private parents are already expanded
		// 2. Kubernetes Component Uri has already been converted to inlined content with a Token if required by default on the first parse
		oldCompDevfileData, err := cdqanalysis.ParseDevfileWithParserArgs(&devfileParser.ParserArgs{Data: []byte(component.Status.Devfile)})

		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component status, exiting reconcile loop %v", req.NamespacedName))
			_ = r.SetUpdateConditionAndUpdateCR(ctx, req, &component, err)
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
				_ = r.SetUpdateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}

			component.Status.ContainerImage = component.Spec.ContainerImage
			component.Status.Devfile = string(yamlHASCompData)
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var currentComponent appstudiov1alpha1.Component
				err := r.Get(ctx, req.NamespacedName, &currentComponent)
				if err != nil {
					return err
				}
				currentComponent.Status.Devfile = component.Status.Devfile
				currentComponent.Status.ContainerImage = component.Status.ContainerImage
				currentComponent.Status.GitOps = component.Status.GitOps
				currentComponent.Status.Conditions = component.Status.Conditions
				err = r.Client.Status().Update(ctx, &currentComponent)
				return err
			})
			if err != nil {
				log.Error(err, "Unable to update Component status")
				// if we're unable to update the Component CR status, then we need to err out
				// since we need the reference of the devfile in Component to be always accessible
				_ = r.SetUpdateConditionAndUpdateCR(ctx, req, &component, err)
				return ctrl.Result{}, err
			}

			// Generate and push the gitops resources, if necessary.
			if !component.Spec.SkipGitOpsResourceGeneration {
				if err := r.generateGitops(ctx, ghClient, &component, hasCompDevfileData); err != nil {
					errMsg := fmt.Sprintf("Unable to generate gitops resources for component %v", req.NamespacedName)
					log.Error(err, errMsg)
					_ = r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, req, &component, fmt.Errorf("%v: %v", errMsg, err))
					_ = r.SetUpdateConditionAndUpdateCR(ctx, req, &component, fmt.Errorf("%v: %v", errMsg, err))
					return ctrl.Result{}, err
				} else {
					err = r.SetGitOpsGeneratedConditionAndUpdateCR(ctx, req, &component, nil)
					if err != nil {
						return ctrl.Result{}, err
					}
				}
			}
			err = r.SetUpdateConditionAndUpdateCR(ctx, req, &component, nil)
			if err != nil {
				return ctrl.Result{}, err
			}

		} else {
			log.Info(fmt.Sprintf("The Component devfile data was not updated %v", req.NamespacedName))
		}
	}

	log.Info(fmt.Sprintf("Finished reconcile loop for %v", req.NamespacedName))
	return ctrl.Result{}, nil
}

// generateGitops retrieves the necessary information about a Component's gitops repository (URL, branch, context)
// and attempts to use the GitOps package to generate gitops resources based on that component
func (r *ComponentReconciler) generateGitops(ctx context.Context, ghClient *github.GitHubClient, component *appstudiov1alpha1.Component, compDevfileData data.DevfileData) error {
	log := ctrl.LoggerFrom(ctx)

	gitOpsURL, gitOpsBranch, gitOpsContext, err := util.ProcessGitOpsStatus(component.Status.GitOps, ghClient.Token)
	if err != nil {
		return err
	}

	// Create a temp folder to create the gitops resources in
	tempDir, err := ioutils.CreateTempPath(component.Name, r.AppFS)
	if err != nil {
		log.Error(err, "unable to create temp directory for GitOps resources due to error")
		ioutils.RemoveFolderAndLogError(log, r.AppFS, tempDir)
		return fmt.Errorf("unable to create temp directory for GitOps resources due to error: %v", err)
	}

	deployAssociatedComponents, err := devfileParser.GetDeployComponents(compDevfileData)
	if err != nil {
		log.Error(err, "unable to get deploy components")
		ioutils.RemoveFolderAndLogError(log, r.AppFS, tempDir)
		return err
	}

	kubernetesResources, err := devfile.GetResourceFromDevfile(log, compDevfileData, deployAssociatedComponents, component.Name, component.Spec.Application, component.Spec.ContainerImage, "")
	if err != nil {
		log.Error(err, "unable to get kubernetes resources from the devfile outerloop components")
		ioutils.RemoveFolderAndLogError(log, r.AppFS, tempDir)
		return err
	}

	// Generate and push the gitops resources
	mappedGitOpsComponent := util.GetMappedGitOpsComponent(*component, kubernetesResources)

	//add the token name to the metrics.  When we add more tokens and rotate, we can determine how evenly distributed the requests are
	metrics.ControllerGitRequest.With(prometheus.Labels{"controller": componentName, "tokenName": ghClient.TokenName, "operation": "CloneGenerateAndPush"}).Inc()
	err = r.Generator.CloneGenerateAndPush(tempDir, gitOpsURL, mappedGitOpsComponent, r.AppFS, gitOpsBranch, gitOpsContext, false)
	if err != nil {
		retErr := err
		if strings.Contains(strings.ToLower(err.Error()), "github push protection") {
			retErr = fmt.Errorf("potential secret leak caught by github push protection")
			// to get the URL token
			// e.g. <GitURL>/security/secret-scanning/unblock-secret/2WlUv72plUf05tgshlpRLzSlH4R        \n
			var unblockURL string
			splited := strings.Split(strings.ToLower(err.Error()), "unblock-secret/")
			if len(splited) > 1 {
				token := strings.Split(splited[1], " ")[0]
				unblockURL = fmt.Sprintf("%v/security/secret-scanning/unblock-secret/%v", component.Status.GitOps.RepositoryURL, token)
				log.Error(retErr, fmt.Sprintf("unable to generate gitops resources due to git push protecton error, follow the link to unblock the secret: %v", unblockURL))
			}
		} else {
			log.Error(retErr, "unable to generate gitops resources due to error")
		}
		ioutils.RemoveFolderAndLogError(log, r.AppFS, tempDir)
		return retErr
	}

	//Gitops functions return sanitized error messages
	metrics.ControllerGitRequest.With(prometheus.Labels{"controller": componentName, "tokenName": ghClient.TokenName, "operation": "CommitAndPush"}).Inc()
	err = r.Generator.CommitAndPush(tempDir, "", gitOpsURL, mappedGitOpsComponent.Name, gitOpsBranch, "Generating GitOps resources")
	if err != nil {
		retErr := err
		if strings.Contains(strings.ToLower(err.Error()), "github push protection") {
			retErr = fmt.Errorf("potential secret leak caught by github push protection")
			// to get the URL token
			// e.g. <GitURL>/security/secret-scanning/unblock-secret/2WlUv72plUf05tgshlpRLzSlH4R        \n
			var unblockURL string
			splited := strings.Split(strings.ToLower(err.Error()), "unblock-secret/")
			if len(splited) > 1 {
				token := strings.Split(splited[1], " ")[0]
				unblockURL = fmt.Sprintf("%v/security/secret-scanning/unblock-secret/%v", component.Status.GitOps.RepositoryURL, token)
				log.Error(retErr, fmt.Sprintf("unable to commit and push gitops resources due to git push protecton error, follow the link to unblock the secret: %v", unblockURL))
			}
		} else {
			log.Error(retErr, "unable to commit and push gitops resources due to error")
		}
		ioutils.RemoveFolderAndLogError(log, r.AppFS, tempDir)
		return retErr
	}

	// Get the commit ID for the gitops repository
	var commitID string
	repoPath := filepath.Join(tempDir, component.Name)
	metricsLabel := prometheus.Labels{"controller": componentName, "tokenName": ghClient.TokenName, "operation": "GetCommitIDFromRepo"}
	metrics.ControllerGitRequest.With(metricsLabel).Inc()
	if commitID, err = r.Generator.GetCommitIDFromRepo(r.AppFS, repoPath); err != nil {
		log.Error(err, "")
		ioutils.RemoveFolderAndLogError(log, r.AppFS, tempDir)
		return err
	}

	component.Status.GitOps.CommitID = commitID

	// Remove the temp folder that was created
	err = r.AppFS.RemoveAll(tempDir)
	if err != nil {
		log.Error(err, "unable to remove temp dir")
		return err
	}

	return nil
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
func (r *ComponentReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	log := ctrl.LoggerFrom(ctx).WithName("controllers").WithName("Component")
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.Component{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(time.Duration(500*time.Millisecond), time.Duration(1000*time.Second)),
		}).WithEventFilter(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			log := log.WithValues("namespace", e.Object.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "Component", logutil.ResourceCreate, nil)
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := log.WithValues("namespace", e.ObjectNew.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "Component", logutil.ResourceUpdate, nil)
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log := log.WithValues("namespace", e.Object.GetNamespace())
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

// getForceGenerateGitopsAnnotation gets the internal annotation on the component whether to force generate the gitops resource
func getForceGenerateGitopsAnnotation(component appstudiov1alpha1.Component) bool {
	compAnnotations := component.GetAnnotations()
	if compAnnotations != nil && compAnnotations[forceGenerationAnnotation] == "true" {
		return true
	}
	return false
}

func setForceGenerateGitopsAnnotation(component *appstudiov1alpha1.Component, value string) {
	compAnnotations := component.GetAnnotations()
	if compAnnotations == nil {
		component.Annotations = make(map[string]string)
	}
	component.Annotations[forceGenerationAnnotation] = value
}

// checkForCreateReconcile checks if the Component is in Create state or an Update state.
// The err condition message is returned if it is in Create state.
func checkForCreateReconcile(component appstudiov1alpha1.Component) (bool, string) {
	var errCondition string
	// Determine if this is a Create reconcile or an Update reconcile based on Conditions
	for _, condition := range component.Status.Conditions {
		if condition.Type == "Updated" {
			// If an Updated Condition is present, it means this is an Update reconcile
			return false, ""
		} else if condition.Type == "Created" && condition.Reason == "Error" && condition.Status == metav1.ConditionFalse {
			errCondition = condition.Message
		}
	}

	// If there are no Conditions or no Updated Condition, then this is a Create reconcile
	return true, errCondition
}

// isGithubURL checks if the given url includes github in hostname
// In case of invalid url (not able to parse) returns false.
func validateGithubURL(URL string) error {
	parsedURL, err := url.Parse(URL)
	if err != nil {
		return err
	}

	if strings.Contains(parsedURL.Host, "github") {
		return nil
	}
	return fmt.Errorf("source git url %v is not from github", URL)
}
