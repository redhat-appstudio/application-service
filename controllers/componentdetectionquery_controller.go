/*
Copyright 2021 Red Hat, Inc.

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

	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"
	logicalcluster "github.com/kcp-dev/logicalcluster/v2"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
	"github.com/redhat-appstudio/application-service/pkg/spi"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	"github.com/spf13/afero"
)

// ComponentDetectionQueryReconciler reconciles a ComponentDetectionQuery object
type ComponentDetectionQueryReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	SPIClient          spi.SPI
	AlizerClient       devfile.Alizer
	Log                logr.Logger
	DevfileRegistryURL string
	AppFS              afero.Afero
}

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
	log := r.Log.WithValues("ComponentDetectionQuery", req.NamespacedName).WithValues("clusterName", req.ClusterName)

	// if we're running on kcp, we need to include workspace in context
	if req.ClusterName != "" {
		ctx = logicalcluster.WithCluster(ctx, logicalcluster.New(req.ClusterName))
	}

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

		source := componentDetectionQuery.Spec.GitSource
		var devfileBytes, dockerfileBytes []byte
		var clonePath string
		var componentPath string
		devfilesMap := make(map[string][]byte)
		devfilesURLMap := make(map[string]string)
		dockerfileContextMap := make(map[string]string)

		context := source.Context
		if context == "" {
			context = "./"
		}

		if source.DevfileURL == "" {
			isMultiComponent := false
			isDockerfilePresent := false
			isDevfilePresent := false
			log.Info(fmt.Sprintf("Attempting to read a devfile from the URL %s... %v", source.URL, req.NamespacedName))
			// check if the project is multi-component or single-component
			if gitToken == "" {
				gitURL, err := util.ConvertGitHubURL(source.URL, source.Revision, context)
				log.Info(fmt.Sprintf("Look for devfile or dockerfile at the URL %s... %v", gitURL, req.NamespacedName))
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to convert Github URL to raw format, exiting reconcile loop %v", req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
					return ctrl.Result{}, nil
				}
				devfileBytes, dockerfileBytes = devfile.DownloadDevfileAndDockerfile(gitURL)
			} else {
				// Use SPI to retrieve the devfile from the private repository
				// TODO - maysunfaisal also search for Dockerfile
				devfileBytes, err = spi.DownloadDevfileUsingSPI(r.SPIClient, ctx, componentDetectionQuery.Namespace, source.URL, "main", context)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to curl for any known devfile locations from %s %v", source.URL, req.NamespacedName))
				}
			}

			isDevfilePresent = len(devfileBytes) != 0
			isDockerfilePresent = len(dockerfileBytes) != 0

			if isDevfilePresent {
				log.Info(fmt.Sprintf("Found a devfile, devfile to be analyzed to see if a Dockerfile is referenced %v", req.NamespacedName))
				devfilesMap[context] = devfileBytes
			} else if isDockerfilePresent {
				log.Info(fmt.Sprintf("Determined that this is a Dockerfile only component  %v", req.NamespacedName))
				dockerfileContextMap[context] = "./Dockerfile"
			}

			// Clone the repo if no dockerfile present
			if !isDockerfilePresent {
				log.Info(fmt.Sprintf("Unable to find devfile or Dockerfile under root directory, run Alizer to detect components... %v", req.NamespacedName))

				clonePath, err = ioutils.CreateTempPath(componentDetectionQuery.Name, r.AppFS)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to create a temp path %s for cloning %v", clonePath, req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
					return ctrl.Result{}, nil
				}

				err = util.CloneRepo(clonePath, source.URL, gitToken)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to clone repo %s to path %s, exiting reconcile loop %v", source.URL, clonePath, req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
					return ctrl.Result{}, nil
				}
				log.Info(fmt.Sprintf("cloned from %s to path %s... %v", source.URL, clonePath, req.NamespacedName))
				componentPath = clonePath
				if context != "" {
					componentPath = path.Join(clonePath, context)
				}

				if !isDevfilePresent {
					components, err := r.AlizerClient.DetectComponents(componentPath)
					if err != nil {
						log.Error(err, fmt.Sprintf("Unable to detect components using Alizer for repo %v, under path %v... %v ", source.URL, componentPath, req.NamespacedName))
						r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
						return ctrl.Result{}, nil
					}
					log.Info(fmt.Sprintf("components detected %v... %v", components, req.NamespacedName))
					// If no devfile and no dockerfile present in the root
					// case 1: no components been detected by Alizer, might still has subfolders contains dockerfile. Need to scan repo
					// case 2: more than 1 components been detected by Alizer, is certain a multi-component project. Need to scan repo
					// case 3: one or more than 1 compinents been detected by Alizer, and the first one in the list is under sub-folder. Need to scan repo.
					if len(components) != 1 || (len(components) != 0 && path.Clean(components[0].Path) != path.Clean(componentPath)) {
						isMultiComponent = true
					}
				}
			}

			// Logic to read multiple components in from git
			if isMultiComponent {
				log.Info(fmt.Sprintf("Since this is a multi-component, attempt will be made to read only level 1 dir for devfiles... %v", req.NamespacedName))

				devfilesMap, devfilesURLMap, dockerfileContextMap, err = devfile.ScanRepo(log, r.AlizerClient, componentPath, r.DevfileRegistryURL)
				if err != nil {
					if _, ok := err.(*devfile.NoDevfileFound); !ok {
						log.Error(err, fmt.Sprintf("Unable to find devfile(s) in repo %s due to an error %s, exiting reconcile loop %v", source.URL, err.Error(), req.NamespacedName))
						r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
						return ctrl.Result{}, nil
					}
				}
			} else {
				log.Info(fmt.Sprintf("Since this is not a multi-component, attempt will be made to read devfile at the root dir... %v", req.NamespacedName))
				if !isDockerfilePresent {
					err := devfile.AnalyzePath(r.AlizerClient, componentPath, context, r.DevfileRegistryURL, devfilesMap, devfilesURLMap, dockerfileContextMap, isDevfilePresent, isDockerfilePresent)
					if err != nil {
						log.Error(err, fmt.Sprintf("Unable to analyze path %s for a dockerfile/devfile %v", componentPath, req.NamespacedName))
						r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
						return ctrl.Result{}, nil
					}
				}
			}
		} else {
			log.Info(fmt.Sprintf("devfile was explicitly specified at %s %v", source.DevfileURL, req.NamespacedName))
			devfileBytes, err = util.CurlEndpoint(source.DevfileURL)
			if err != nil {
				// if a direct devfileURL is provided and errors out, we dont do an alizer detection
				log.Error(err, fmt.Sprintf("Unable to GET %s, exiting reconcile loop %v", source.DevfileURL, req.NamespacedName))
				err := fmt.Errorf("unable to GET from %s", source.DevfileURL)
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
			devfilesMap[context] = devfileBytes
		}

		// Remove the cloned path if present
		if isExist, _ := ioutils.IsExisting(r.AppFS, clonePath); isExist {
			if err := r.AppFS.RemoveAll(clonePath); err != nil {
				log.Error(err, fmt.Sprintf("Unable to remove the clonepath %s %v", clonePath, req.NamespacedName))
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
		}

		for context, link := range dockerfileContextMap {
			updatedLink, err := devfile.UpdateDockerfileLink(source.URL, source.Revision, link)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to update the dockerfile link %v", req.NamespacedName))
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
			dockerfileContextMap[context] = updatedLink
		}

		err = r.updateComponentStub(req, &componentDetectionQuery, devfilesMap, devfilesURLMap, dockerfileContextMap)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to update the component stub %v", req.NamespacedName))
			r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
			return ctrl.Result{}, nil
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
func (r *ComponentDetectionQueryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := zap.Options{
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log.WithName("controllers").WithName("ComponentDetectionQuery").WithValues("appstudio-component", "HAS")
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.ComponentDetectionQuery{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).WithEventFilter(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			log = log.WithValues("Namespace", e.Object.GetNamespace()).WithValues("clusterName", logicalcluster.From(e.Object).String())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "ComponentDetectionQuery", logutil.ResourceCreate, nil)
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			log = log.WithValues("Namespace", e.ObjectNew.GetNamespace()).WithValues("clusterName", logicalcluster.From(e.ObjectNew).String())
			logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "ComponentDetectionQuery", logutil.ResourceUpdate, nil)
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log = log.WithValues("Namespace", e.Object.GetNamespace()).WithValues("clusterName", logicalcluster.From(e.Object).String())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "ComponentDetectionQuery", logutil.ResourceDelete, nil)
			return false
		},
	}).
		Complete(r)
}
