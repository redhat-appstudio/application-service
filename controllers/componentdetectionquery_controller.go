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
	"github.com/redhat-appstudio/application-service/pkg/spi"
	util "github.com/redhat-appstudio/application-service/pkg/util"
)

// ComponentDetectionQueryReconciler reconciles a ComponentDetectionQuery object
type ComponentDetectionQueryReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	SPIClient spi.SPI
	Log       logr.Logger
}

const (
	clonePathPrefix = "/tmp/appstudio/has"
)

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
	log.Info(fmt.Sprintf("Starting reconcile loop for %v", req.NamespacedName))

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

	// If there is no component list in the map, the CR was just created
	if len(componentDetectionQuery.Status.ComponentDetected) == 0 {
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
		devfilesMap := make(map[string][]byte)
		devfilesURLMap := make(map[string]string)

		clonePath := path.Join(clonePathPrefix, componentDetectionQuery.Namespace, componentDetectionQuery.Name)

		if source.DevfileURL == "" {
			log.Info(fmt.Sprintf("Attempting to read a devfile from the URL %s... %v", source.URL, req.NamespacedName))
			// Logic to read multiple components in from git
			if componentDetectionQuery.Spec.IsMultiComponent {
				log.Info(fmt.Sprintf("Since this is a multi-component, attempt will be made to read only level 1 dir for devfiles... %v", req.NamespacedName))

				err = util.CloneRepo(clonePath, source.URL, gitToken)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to clone repo %s to path %s, exiting reconcile loop %v", source.URL, clonePath, req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
					return ctrl.Result{}, nil
				}
				log.Info(fmt.Sprintf("cloned from %s to path %s... %v", source.URL, clonePath, req.NamespacedName))

				devfilesMap, devfilesURLMap, err = util.ReadDevfilesFromRepo(clonePath, maxDevfileDiscoveryDepth)
				if err != nil {
					if _, ok := err.(*util.NoDevfileFound); !ok {
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

					devfileBytes, err = util.DownloadDevfile(gitURL)
					if err != nil {
						if _, ok := err.(*util.NoDevfileFound); !ok {
							log.Error(err, fmt.Sprintf("Unable to curl for any known devfile locations from %s due to an error %s,  %v", source.URL, err.Error(), req.NamespacedName))
							r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
							return ctrl.Result{}, nil
						}
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
					err = util.CloneRepo(clonePath, source.URL)
					if err != nil {
						log.Error(err, fmt.Sprintf("Unable to clone repo %s to path %s, exiting reconcile loop %v", source.URL, clonePath, req.NamespacedName))
						r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
						return ctrl.Result{}, nil
					}

					log.Info(fmt.Sprintf("cloned from %s to path %s... %v", source.URL, clonePath, req.NamespacedName))
					log.Info(fmt.Sprintf("analyzing path %s", clonePath))

					// if we didnt find any devfile upto our desired depth, then use alizer
					var detectedDevfileEndpoint string
					devfileBytes, detectedDevfileEndpoint, err = util.AnalyzeAndDetectDevfile(clonePath)
					if err != nil {
						if _, ok := err.(*util.NoDevfileFound); !ok {
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

		err := r.updateComponentStub(&componentDetectionQuery, devfilesMap, devfilesURLMap)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to update the component stub %v", req.NamespacedName))
			r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
			return ctrl.Result{}, nil
		}

		r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, nil)
	} else {
		log.Info(fmt.Sprintf("No update scenario yet... %v", req.NamespacedName))
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentDetectionQueryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.ComponentDetectionQuery{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
