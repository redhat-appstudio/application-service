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
	"path"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	cdqanalysis "github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	"github.com/redhat-appstudio/application-service/pkg/github"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
	"github.com/redhat-appstudio/application-service/pkg/metrics"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	"github.com/spf13/afero"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ComponentDetectionQueryReconciler reconciles a ComponentDetectionQuery object
type ComponentDetectionQueryReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	Log                logr.Logger
	GitHubTokenClient  github.GitHubToken
	DevfileRegistryURL string
	AppFS              afero.Afero
}

const cdqName = "ComponentDetectionQuery"

// CDQReconcileTimeout is the default timeout, 5 mins, for the context of cdq reconcile
const CDQReconcileTimeout = 5 * time.Minute

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=componentdetectionqueries,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=componentdetectionqueries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=componentdetectionqueries/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ComponentDetectionQuery object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ComponentDetectionQueryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// set 5 mins timeout for cdq reconcile
	ctx, cancel := context.WithTimeout(ctx, CDQReconcileTimeout)
	defer cancel()

	// Fetch the ComponentDetectionQuery instance
	var componentDetectionQuery appstudiov1alpha1.ComponentDetectionQuery
	err := r.Get(ctx, req.NamespacedName, &componentDetectionQuery)
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

	// If there are no conditions attached to the CDQ, the resource was just created
	if len(componentDetectionQuery.Status.Conditions) == 0 {
		// Start the ComponentDetectionQuery, and update its status condition accordingly
		log.Info(fmt.Sprintf("Checking to see if a component can be detected %v", req.NamespacedName))
		r.SetDetectingConditionAndUpdateCR(ctx, req, &componentDetectionQuery)

		// Create a copy of the CDQ, to use as the base when setting the CDQ status via mergepatch later
		copiedCDQ := componentDetectionQuery.DeepCopy()

		var gitToken string
		if componentDetectionQuery.Spec.Secret != "" {
			gitSecret := corev1.Secret{}
			namespacedName := types.NamespacedName{
				Name:      componentDetectionQuery.Spec.Secret,
				Namespace: componentDetectionQuery.Namespace,
			}
			err = r.Client.Get(ctx, namespacedName, &gitSecret)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to retrieve Git secret %v, exiting reconcile loop %v", componentDetectionQuery.Spec.Secret, req.NamespacedName))
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
			gitToken = string(gitSecret.Data["password"])
		}

		// Create a Go-GitHub client for checking the default branch
		ghClient, err := r.GitHubTokenClient.GetNewGitHubClient(gitToken)
		if err != nil {
			log.Error(err, "Unable to create Go-GitHub client due to error")
			return reconcile.Result{}, err
		}

		// Add the Go-GitHub client name to the context
		ctx = context.WithValue(ctx, github.GHClientKey, ghClient.TokenName)

		source := componentDetectionQuery.Spec.GitSource
		var clonePath, devfilePath string
		devfilesMap := make(map[string][]byte)
		devfilesURLMap := make(map[string]string)
		dockerfileContextMap := make(map[string]string)
		componentPortsMap := make(map[string][]int)
		context := source.Context

		if context == "" {
			context = "./"
		}
		// remove leading and trailing spaces of the repo URL
		source.URL = strings.TrimSpace(source.URL)
		sourceURL := source.URL
		// If the repository URL ends in a forward slash, remove it to avoid issues with default branch lookup
		if string(sourceURL[len(sourceURL)-1]) == "/" {
			sourceURL = sourceURL[0 : len(sourceURL)-1]
		}
		err = util.ValidateEndpoint(sourceURL)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to validate the source URL %v... %v", source.URL, req.NamespacedName))
			r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
			return ctrl.Result{}, nil
		}

		if source.DevfileURL == "" {
			log.Info(fmt.Sprintf("Attempting to read a devfile from the URL %s... %v", source.URL, req.NamespacedName))

			k8sInfoClient := cdqanalysis.K8sInfoClient{
				Log:          log,
				CreateK8sJob: false,
			}

			cdqInfo := &cdqanalysis.CDQInfoClient{
				DevfileRegistryURL: r.DevfileRegistryURL,
				GitURL:             cdqanalysis.GitURL{RepoURL: source.URL, Revision: source.Revision, Token: gitToken},
			}

			devfilesMapReturned, devfilesURLMapReturned, dockerfileContextMapReturned, componentPortsMapReturned, branch, err := cdqanalysis.CloneAndAnalyze(k8sInfoClient, req.Namespace, req.Name, context, cdqInfo)
			componentDetectionQuery.Spec.GitSource.Revision = branch
			if err != nil {
				log.Error(err, fmt.Sprintf("Error running cdq analysis... %v", req.NamespacedName))
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}

			clonePath = cdqInfo.ClonedRepo.ClonedPath
			devfilePath, _ = cdqanalysis.GetDevfileAndDockerFilePaths(*cdqInfo)

			maps.Copy(devfilesMap, devfilesMapReturned)
			maps.Copy(dockerfileContextMap, dockerfileContextMapReturned)
			maps.Copy(devfilesURLMap, devfilesURLMapReturned)
			maps.Copy(componentPortsMap, componentPortsMapReturned)

		} else {
			log.Info(fmt.Sprintf("devfile was explicitly specified at %s %v", source.DevfileURL, req.NamespacedName))

			// For scenarios where a devfile is passed in, we still need to use the GH API for branch detection as we do not clone.
			if source.Revision == "" {
				log.Info(fmt.Sprintf("Look for default branch of repo %s... %v", source.URL, req.NamespacedName))
				metricsLabel := prometheus.Labels{"controller": cdqName, "tokenName": ghClient.TokenName, "operation": "GetDefaultBranchFromURL"}
				metrics.ControllerGitRequest.With(metricsLabel).Inc()
				source.Revision, err = ghClient.GetDefaultBranchFromURL(sourceURL, ctx)
				metrics.HandleRateLimitMetrics(err, metricsLabel)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to get default branch of Github Repo %v, try to fall back to main branch... %v", source.URL, req.NamespacedName))
					metricsLabel := prometheus.Labels{"controller": cdqName, "tokenName": ghClient.TokenName, "operation": "GetBranchFromURL"}
					metrics.ControllerGitRequest.With(metricsLabel).Inc()
					_, err := ghClient.GetBranchFromURL(sourceURL, ctx, "main")
					if err != nil {
						metrics.HandleRateLimitMetrics(err, metricsLabel)
						log.Error(err, fmt.Sprintf("Unable to get main branch of Github Repo %v ... %v", source.URL, req.NamespacedName))
						retErr := fmt.Errorf("unable to get default branch of Github Repo %v, try to fall back to main branch, failed to get main branch... %v", source.URL, req.NamespacedName)
						r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, retErr)
						return ctrl.Result{}, nil
					} else {
						source.Revision = "main"
					}
				}
			}

			// set in the CDQ spec
			componentDetectionQuery.Spec.GitSource.Revision = source.Revision

			shouldIgnoreDevfile, devfileBytes, err := cdqanalysis.ValidateDevfile(log, source.DevfileURL, gitToken)
			if err != nil {
				// if a direct devfileURL is provided and errors out, we dont do an alizer detection
				log.Error(err, fmt.Sprintf("Unable to GET %s, exiting reconcile loop %v", source.DevfileURL, req.NamespacedName))
				err := fmt.Errorf("unable to GET from %s", source.DevfileURL)
				ioutils.RemoveFolderAndLogError(log, r.AppFS, clonePath)
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
			if shouldIgnoreDevfile {
				// if a direct devfileURL is provided and errors out, we dont do an alizer detection
				log.Error(err, fmt.Sprintf("the provided devfileURL %s does not contain a valid outerloop definition, exiting reconcile loop %v", source.DevfileURL, req.NamespacedName))
				err := fmt.Errorf("the provided devfileURL %s does not contain a valid outerloop definition", source.DevfileURL)
				ioutils.RemoveFolderAndLogError(log, r.AppFS, clonePath)
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
			devfilesMap[context] = devfileBytes
			devfilesURLMap[context] = source.DevfileURL
		}

		// Remove the cloned path if present
		if isExist, _ := ioutils.IsExisting(r.AppFS, clonePath); isExist {
			if err := r.AppFS.RemoveAll(clonePath); err != nil {
				log.Error(err, fmt.Sprintf("Unable to remove the clonepath %s %v", clonePath, req.NamespacedName))
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
		}

		for context := range devfilesMap {
			if _, ok := devfilesURLMap[context]; !ok {
				updatedLink, err := cdqanalysis.UpdateGitLink(source.URL, source.Revision, path.Join(context, devfilePath))
				if err != nil {
					log.Error(err, fmt.Sprintf(
						"Unable to update the devfile link %v", req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
					return ctrl.Result{}, nil
				}
				devfilesURLMap[context] = updatedLink
			}
		}
		// only update the componentStub when a component has been detected
		if len(devfilesMap) != 0 || len(devfilesURLMap) != 0 || len(dockerfileContextMap) != 0 {
			err = r.updateComponentStub(req, ctx, &componentDetectionQuery, devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, gitToken)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to update the component stub %v", req.NamespacedName))
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
		}

		r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, nil)
	} else {
		// CDQ resource has been requeued after it originally ran
		// Delete the resource as it's no longer needed and can be cleaned up
		log.Info(fmt.Sprintf("Deleting finished ComponentDetectionQuery resource %v", req.NamespacedName))
		if err = r.Delete(ctx, &componentDetectionQuery); err != nil {
			// Delete failed. Log the error but don't bother modifying the resource's status
			logutil.LogAPIResourceChangeEvent(log, componentDetectionQuery.Name, "ComponentDetectionQuery", logutil.ResourceDelete, err)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentDetectionQueryReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	log := ctrl.LoggerFrom(ctx).WithName("controllers").WithName("ComponentDetectionQuery")

	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.ComponentDetectionQuery{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).WithEventFilter(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			log := log.WithValues("namespace", e.Object.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "ComponentDetectionQuery", logutil.ResourceCreate, nil)
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := log.WithValues("namespace", e.ObjectNew.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "ComponentDetectionQuery", logutil.ResourceUpdate, nil)
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log := log.WithValues("namespace", e.Object.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "ComponentDetectionQuery", logutil.ResourceDelete, nil)
			return false
		},
	}).
		Complete(r)
}
