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
	"reflect"
	"time"

	"github.com/devfile/library/v2/pkg/devfile/parser"

	cdqanalysis "github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	"github.com/redhat-appstudio/application-service/pkg/metrics"

	gofakeit "github.com/brianvoe/gofakeit/v6"
	"github.com/go-logr/logr"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
)

const appFinalizerName = "application.appstudio.redhat.com/finalizer"

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Application object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Get the Application resource
	var application appstudiov1alpha1.Application
	err := r.Get(ctx, req.NamespacedName, &application)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// If the resource still has the finalizer attached to it, just remove it so deletion doesn't get blocked
	if containsString(application.GetFinalizers(), appFinalizerName) {
		// remove the finalizer from the list and update it.
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var currentApplication appstudiov1alpha1.Application
			err := r.Get(ctx, req.NamespacedName, &currentApplication)
			if err != nil {
				return err
			}

			controllerutil.RemoveFinalizer(&currentApplication, appFinalizerName)

			err = r.Update(ctx, &currentApplication)
			return err
		})
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	log.Info(fmt.Sprintf("Starting reconcile loop for %v", req.NamespacedName))
	// If devfile hasn't been generated yet, generate it
	// If the devfile hasn't been generated, the CR was just created.
	if application.Status.Devfile == "" {
		metrics.ApplicationCreationTotalReqs.Inc()

		// Convert the devfile string to a devfile object
		devfileData, err := devfile.ConvertApplicationToDevfile(application)
		if err != nil {
			metrics.ApplicationCreationFailed.Inc()
			log.Error(err, fmt.Sprintf("Unable to convert Application CR to devfile, exiting reconcile loop %v", req.NamespacedName))
			r.SetCreateConditionAndUpdateCR(ctx, req, &application, err)
			return reconcile.Result{}, err
		}

		// Find all components owned by the application
		err = r.getAndAddComponentApplicationsToModel(log, req, application.Name, devfileData.GetDevfileWorkspaceSpec())
		if err != nil {
			r.SetCreateConditionAndUpdateCR(ctx, req, &application, err)
			log.Error(err, fmt.Sprintf("Unable to add components to application model for %v", req.NamespacedName))
			return ctrl.Result{}, err
		}

		yamlData, err := yaml.Marshal(devfileData)
		if err != nil {
			metrics.ApplicationCreationFailed.Inc()
			log.Error(err, fmt.Sprintf("Unable to marshall Application devfile, exiting reconcile loop %v", req.NamespacedName))
			r.SetCreateConditionAndUpdateCR(ctx, req, &application, err)
			return reconcile.Result{}, err
		}

		application.Status.Devfile = string(yamlData)

		// Update the status of the CR
		metrics.ApplicationCreationSucceeded.Inc()
		r.SetCreateConditionAndUpdateCR(ctx, req, &application, nil)
	} else {
		// If the model already exists, see if either the displayname or description need updating
		// Get the devfile of the hasApp CR

		// Token can be empty since we are passing in generated devfile data, so we won't be dealing with private repos
		devfileData, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(application.Status.Devfile)})
		if err != nil {
			r.SetUpdateConditionAndUpdateCR(ctx, req, &application, err)
			log.Error(err, fmt.Sprintf("Unable to parse devfile model, exiting reconcile loop %v", req.NamespacedName))
			return ctrl.Result{}, err
		}

		updateRequired := false
		// nil out the attributes and projects for the application devfile
		// The Attributes contain any image components for the application
		// And the projects contains any git components for the application
		devWorkspacesSpec := devfileData.GetDevfileWorkspaceSpec().DeepCopy()
		devWorkspacesSpec.Attributes = nil
		devWorkspacesSpec.Projects = nil

		err = r.getAndAddComponentApplicationsToModel(log, req, application.Name, devWorkspacesSpec)
		if err != nil {
			r.SetUpdateConditionAndUpdateCR(ctx, req, &application, err)
			log.Error(err, fmt.Sprintf("Unable to add components to application model for %v", req.NamespacedName))
			return ctrl.Result{}, err
		}
		// Update any specific fields that changed
		displayName := application.Spec.DisplayName
		description := application.Spec.Description
		devfileMeta := devfileData.GetMetadata()
		if devfileMeta.Name != displayName {
			devfileMeta.Name = displayName
			updateRequired = true
		}
		if devfileMeta.Description != description {
			devfileMeta.Description = description
			updateRequired = true
		}

		oldDevSpec := devfileData.GetDevfileWorkspaceSpec()
		if !reflect.DeepEqual(oldDevSpec.Attributes, devWorkspacesSpec.Attributes) || !reflect.DeepEqual(oldDevSpec.Projects, devWorkspacesSpec.Projects) {
			devfileData.SetDevfileWorkspaceSpec(*devWorkspacesSpec)
			updateRequired = true
		}

		if updateRequired {
			devfileData.SetMetadata(devfileMeta)

			// Update the Application CR with the new devfile
			yamlData, err := yaml.Marshal(devfileData)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to marshall Application devfile, exiting reconcile loop %v", req.NamespacedName))
				r.SetUpdateConditionAndUpdateCR(ctx, req, &application, err)
				return reconcile.Result{}, err
			}

			application.Status.Devfile = string(yamlData)
			r.SetUpdateConditionAndUpdateCR(ctx, req, &application, nil)
		}

	}

	log.Info(fmt.Sprintf("Finished reconcile loop for %v", req.NamespacedName))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	gofakeit.New(0)
	log := ctrl.LoggerFrom(ctx).WithName("controllers").WithName("Application")

	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.Application{}).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(time.Duration(1*time.Second), time.Duration(1000*time.Second)),
		}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				log := log.WithValues("namespace", e.Object.GetNamespace())
				logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "Application", logutil.ResourceCreate, nil)
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				log := log.WithValues("namespace", e.ObjectNew.GetNamespace())
				logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "Application", logutil.ResourceUpdate, nil)
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				log := log.WithValues("namespace", e.Object.GetNamespace())
				logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "Application", logutil.ResourceDelete, nil)
				return false
			},
		}).
		// Watch Components (Create and Delete events only) as a secondary resource
		Watches(&source.Kind{Type: &appstudiov1alpha1.Component{}}, handler.EnqueueRequestsFromMapFunc(MapComponentToApplication()), builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return true
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		})).
		Complete(r)
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
