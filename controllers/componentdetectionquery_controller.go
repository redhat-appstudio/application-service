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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
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
	log := r.Log.WithValues("ComponentDetectionQuery", req.NamespacedName)

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
		r.SetDetectingConditionAndUpdateCR(ctx, &componentDetectionQuery)

		log.Info(fmt.Sprintf("Checking to see if a component can be detected %v", req.NamespacedName))
		var gitToken string
		if componentDetectionQuery.Spec.GitSource.Secret != "" {
			gitSecret := corev1.Secret{}
			namespacedName := types.NamespacedName{
				Name:      componentDetectionQuery.Spec.GitSource.Secret,
				Namespace: componentDetectionQuery.Namespace,
			}

			err = r.Client.Get(ctx, namespacedName, &gitSecret)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to retrieve Git secret %v, exiting reconcile loop %v", componentDetectionQuery.Spec.GitSource.Secret, req.NamespacedName))
				r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
				return ctrl.Result{}, nil
			}
			gitToken = string(gitSecret.Data["password"])
		}

		source := componentDetectionQuery.Spec.GitSource
		var devfileBytes []byte
		var clonePath string
		devfilesMap := make(map[string][]byte)
		devfilesURLMap := make(map[string]string)

		if source.DevfileURL == "" {
			log.Info(fmt.Sprintf("Attempting to read a devfile from the URL %s... %v", source.URL, req.NamespacedName))
			// Logic to read multiple components in from git
			if componentDetectionQuery.Spec.IsMultiComponent {
				log.Info(fmt.Sprintf("Since this is a multi-component, attempt will be made to read only level 1 dir for devfiles... %v", req.NamespacedName))

				clonePath, err = ioutils.CreateTempPath(componentDetectionQuery.Name, r.AppFS)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to create a temp path %s for cloning %v", clonePath, req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
					return ctrl.Result{}, nil
				}

				err = util.CloneRepo(clonePath, source.URL, gitToken)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to clone repo %s to path %s, exiting reconcile loop %v", source.URL, clonePath, req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
					return ctrl.Result{}, nil
				}
				log.Info(fmt.Sprintf("cloned from %s to path %s... %v", source.URL, clonePath, req.NamespacedName))

				devfilesMap, devfilesURLMap, err = devfile.ReadDevfilesFromRepo(r.AlizerClient, clonePath, maxDevfileDiscoveryDepth, r.DevfileRegistryURL)
				if err != nil {
					if _, ok := err.(*devfile.NoDevfileFound); !ok {
						log.Error(err, fmt.Sprintf("Unable to find devfile(s) in repo %s due to an error %s, exiting reconcile loop %v", source.URL, err.Error(), req.NamespacedName))
						r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
						return ctrl.Result{}, nil
					}
				}
			} else {
				log.Info(fmt.Sprintf("Since this is not a multi-component, attempt will be made to read devfile at the root dir... %v", req.NamespacedName))
				var gitURL string
				if gitToken == "" {
					gitURL, err = util.ConvertGitHubURL(source.URL)
					if err != nil {
						log.Error(err, fmt.Sprintf("Unable to convert Github URL to raw format, exiting reconcile loop %v", req.NamespacedName))
						r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
						return ctrl.Result{}, nil
					}

					devfileBytes, err = devfile.DownloadDevfile(gitURL)
					if err != nil {
						log.Info(fmt.Sprintf("Unable to curl for any known devfile locations from %s due to %v, repo will be cloned and analyzed %v", source.URL, err, req.NamespacedName))
					}
				} else {
					// Use SPI to retrieve the devfile from the private repository
					devfileBytes, err = spi.DownloadDevfileUsingSPI(r.SPIClient, ctx, componentDetectionQuery.Namespace, source.URL, "main", "")
					if err != nil {
						log.Error(err, fmt.Sprintf("Unable to curl for any known devfile locations from %s %v", source.URL, req.NamespacedName))
						r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
						return ctrl.Result{}, nil
					}
				}

				if len(devfileBytes) != 0 {
					devfilesMap["./"] = devfileBytes
				} else {
					clonePath, err = ioutils.CreateTempPath(componentDetectionQuery.Name, r.AppFS)
					if err != nil {
						log.Error(err, fmt.Sprintf("Unable to create a temp path %s for cloning %v", clonePath, req.NamespacedName))
						r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
						return ctrl.Result{}, nil
					}

					err = util.CloneRepo(clonePath, source.URL, gitToken)
					if err != nil {
						log.Error(err, fmt.Sprintf("Unable to clone repo %s to path %s, exiting reconcile loop %v", source.URL, clonePath, req.NamespacedName))
						r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
						return ctrl.Result{}, nil
					}

					log.Info(fmt.Sprintf("cloned from %s to path %s... %v", source.URL, clonePath, req.NamespacedName))
					log.Info(fmt.Sprintf("analyzing path %s", clonePath))

					// if we didnt find any devfile upto our desired depth, then use alizer
					var detectedDevfileEndpoint string
					devfileBytes, detectedDevfileEndpoint, err = devfile.AnalyzeAndDetectDevfile(r.AlizerClient, clonePath, r.DevfileRegistryURL)
					if err != nil {
						if _, ok := err.(*devfile.NoDevfileFound); !ok {
							log.Error(err, fmt.Sprintf("unable to detect devfile in path %s %v", clonePath, req.NamespacedName))
							r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
							return ctrl.Result{}, nil
						}
					}

					if len(devfileBytes) > 0 {
						devfilesMap["./"] = devfileBytes
						devfilesURLMap["./"] = detectedDevfileEndpoint
					}
				}
			}

			// Remove the cloned path if present
			if isExist, _ := ioutils.IsExisting(r.AppFS, clonePath); isExist {
				if err := r.AppFS.RemoveAll(clonePath); err != nil {
					log.Error(err, fmt.Sprintf("Unable to remove the clonepath %s %v", clonePath, req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
					return ctrl.Result{}, nil
				}
			}
		} else {
			if componentDetectionQuery.Spec.IsMultiComponent {
				errMsg := fmt.Sprintf("cannot set IsMultiComponent to %v and set DevfileURL to %s as well... %v", componentDetectionQuery.Spec.IsMultiComponent, source.DevfileURL, req.NamespacedName)
				log.Error(err, errMsg)
				err := fmt.Errorf(errMsg)
				r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
				return ctrl.Result{}, nil
			}

			log.Info(fmt.Sprintf("devfile was explicitly specified at %s %v", source.DevfileURL, req.NamespacedName))
			devfileBytes, err = util.CurlEndpoint(source.DevfileURL)
			if err != nil {
				// if a direct devfileURL is provided and errors out, we dont do an alizer detection
				log.Error(err, fmt.Sprintf("Unable to GET %s, exiting reconcile loop %v", source.DevfileURL, req.NamespacedName))
				err := fmt.Errorf("unable to GET from %s", source.DevfileURL)
				r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
				return ctrl.Result{}, nil
			}
			devfilesMap["./"] = devfileBytes
		}

		err = r.updateComponentStub(&componentDetectionQuery, devfilesMap, devfilesURLMap)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to update the component stub %v", req.NamespacedName))
			r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
			return ctrl.Result{}, nil
		}

		r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, nil)
	} else {
		// CDQ resource has been requeued after it originally ran
		// Delete the resource as it's no longer needed and can be cleaned up
		log.Info("Deleting finished ComponentDetectionQuery resource %v", req.NamespacedName)
		if err = r.Delete(ctx, &componentDetectionQuery); err != nil {
			// Delete failed. Log the error but don't bother modifying the resource's status
			log.Error(err, fmt.Sprintf("Unable to delete the ComponentDetectionQuery resource %v", req.NamespacedName))
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentDetectionQueryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.ComponentDetectionQuery{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
