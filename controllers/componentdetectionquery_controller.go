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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	util "github.com/redhat-appstudio/application-service/pkg/util"
)

// ComponentDetectionQueryReconciler reconciles a ComponentDetectionQuery object
type ComponentDetectionQueryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
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

		source := componentDetectionQuery.Spec.GitSource
		var devfileBytes []byte
		devfilesMap := make(map[string][]byte)

		if source.DevfileURL == "" {
			log.Info(fmt.Sprintf("Attempting to read a devfile from the URL %s... %v", source.URL, req.NamespacedName))
			// Logic to read multiple components in from git
			if componentDetectionQuery.Spec.IsMultiComponent {
				log.Info(fmt.Sprintf("Since this is a multi-component, attempt will be made to read only level 1 dir for devfiles... %v", req.NamespacedName))

				clonePath := path.Join(clonePathPrefix, componentDetectionQuery.Namespace, componentDetectionQuery.Name)

				err = util.CloneRepo(clonePath, source.URL)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to clone repo %s and read devfile(s) in path %s, exiting reconcile loop %v", source.URL, clonePath, req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
					return ctrl.Result{}, err
				}
				log.Info(fmt.Sprintf("cloned from %s to path %s... %v", source.URL, clonePath, req.NamespacedName))

				devfilesMap, err = util.ReadDevfilesFromRepo(clonePath, 1)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to find devfile(s) in repo %s, exiting reconcile loop %v", source.URL, req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
					return ctrl.Result{}, err
				}
			} else {
				log.Info(fmt.Sprintf("Since this is not a multi-component, attempt will be made to read devfile at the root dir... %v", req.NamespacedName))
				rawURL, err := util.ConvertGitHubURL(source.URL)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to convert Github URL to raw format, exiting reconcile loop %v", req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
					return ctrl.Result{}, err
				}

				devfileBytes, err = util.DownloadDevfile(rawURL)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to curl for any known devfile locations from %s %v", source.URL, req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
					return ctrl.Result{}, err
				}

				devfilesMap["./"] = devfileBytes
			}
		} else {
			if componentDetectionQuery.Spec.IsMultiComponent {
				errMsg := fmt.Sprintf("cannot set IsMultiComponent to %v and set DevfileURL to %s as well... %v", componentDetectionQuery.Spec.IsMultiComponent, source.DevfileURL, req.NamespacedName)
				log.Error(err, errMsg)
				err := fmt.Errorf(errMsg)
				r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
				return ctrl.Result{}, err
			}

			log.Info(fmt.Sprintf("devfile was explicitly specified at %s %v", source.DevfileURL, req.NamespacedName))
			devfileBytes, err = util.CurlEndpoint(source.DevfileURL)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to GET %s, exiting reconcile loop %v", source.DevfileURL, req.NamespacedName))
				err := fmt.Errorf("unable to GET from %s", source.DevfileURL)
				r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
				return ctrl.Result{}, err
			}
			devfilesMap["./"] = devfileBytes
		}

		err := r.updateComponentStub(&componentDetectionQuery, devfilesMap)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to update the component stub %v", req.NamespacedName))
			r.SetCompleteConditionAndUpdateCR(ctx, &componentDetectionQuery, err)
			return ctrl.Result{}, err
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
