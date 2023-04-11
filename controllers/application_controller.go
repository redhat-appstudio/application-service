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

	gofakeit "github.com/brianvoe/gofakeit/v6"
	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	appAdapter "github.com/redhat-appstudio/application-service/controllers/adapters/application"
	github "github.com/redhat-appstudio/application-service/pkg/github"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
	util "github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/redhat-appstudio/operator-goodies/reconciler"
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Log               logr.Logger
	GitHubTokenClient github.GitHubToken
	GitHubOrg         string
}

const applicationName = "Application"

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
	log := r.Log.WithValues("Application", req.NamespacedName)

	ghClient, err := r.GitHubTokenClient.GetNewGitHubClient()
	if err != nil {
		log.Error(err, "Unable to create Go-GitHub client due to error")
		return reconcile.Result{}, err
	}

	application, components, err := r.GetApplicationAndComponents(ctx, req)
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

	// Check if the Application CR is under deletion
	// If so: Remove the GitOps repo (if generated) and remove the finalizer.
	if application.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(application.GetFinalizers(), appFinalizerName) {
			// Attach the finalizer and return to reset the reconciler loop
			err := r.AddFinalizer(ctx, &application)
			if err == nil {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
	} else {
		if util.ContainsString(application.GetFinalizers(), appFinalizerName) {
			// A finalizer is present for the Application CR, so make sure we do the necessary cleanup steps
			if err := r.Finalize(&application, ghClient); err != nil {
				// if fail to delete the external dependency here, log the error, but don't return error
				// Don't want to get stuck in a cycle of repeatedly trying to delete the repository and failing
				log.Error(err, "Unable to delete GitOps repository for application %v in namespace %v", application.GetName(), application.GetNamespace())
			}

			// remove the finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&application, appFinalizerName)
			if err := r.Update(ctx, &application); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	adapter := appAdapter.Adapter{
		Application:    &application,
		NamespacedName: req.NamespacedName,
		Components:     components,
		GithubOrg:      r.GitHubOrg,
		GitHubClient:   ghClient,
		Client:         r.Client,
		Ctx:            ctx,
		Log:            log,
	}

	// Reconcile the Application
	return reconciler.ReconcileHandler([]reconciler.ReconcileOperation{
		adapter.EnsureGitOpsRepoExists,
		adapter.EnsureApplicationDevfile,
		adapter.EnsureApplicationStatus,
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	gofakeit.New(0)
	opts := zap.Options{
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	mapComponentToApplication := handler.EnqueueRequestsFromMapFunc(MapComponentToApplication())

	log := ctrl.Log.WithName("controllers").WithName("Application").WithValues("appstudio-component", "HAS")
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.Application{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				log := log.WithValues("Namespace", e.Object.GetNamespace())
				logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "Application", logutil.ResourceCreate, nil)
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				log := log.WithValues("Namespace", e.ObjectNew.GetNamespace())
				logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "Application", logutil.ResourceUpdate, nil)
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				log := log.WithValues("Namespace", e.Object.GetNamespace())
				logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "Application", logutil.ResourceDelete, nil)
				return false
			},
		}).
		Watches(&source.Kind{Type: &appstudiov1alpha1.Component{}}, mapComponentToApplication).
		Complete(r)
}

// GetApplicationAndComponents returns the application associated with the current reconcile, along with any Components that belong to it
func (r *ApplicationReconciler) GetApplicationAndComponents(ctx context.Context, req ctrl.Request) (appstudiov1alpha1.Application, []appstudiov1alpha1.Component, error) {
	// Get the Application
	var application appstudiov1alpha1.Application
	err := r.Get(ctx, req.NamespacedName, &application)
	if err != nil {
		return appstudiov1alpha1.Application{}, []appstudiov1alpha1.Component{}, err
	}

	// Get all of the Components that belong to the Application
	var components []appstudiov1alpha1.Component
	var componentList appstudiov1alpha1.ComponentList
	err = r.Client.List(ctx, &componentList, &client.ListOptions{
		Namespace: req.NamespacedName.Namespace,
	})
	if err != nil {
		return appstudiov1alpha1.Application{}, []appstudiov1alpha1.Component{}, err
	}
	for _, component := range componentList.Items {
		if component.Spec.Application == application.GetName() {
			components = append(components, component)
		}
	}

	return application, components, nil
}

// MapComponentToApplication returns an event handler will convert events on a Component CR to events on its parent Application
func MapComponentToApplication() func(object client.Object) []reconcile.Request {
	return func(obj client.Object) []reconcile.Request {
		component := obj.(*appstudiov1alpha1.Component)
		if component != nil && component.Spec.Application != "" {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: component.Namespace,
						Name:      component.Spec.Application,
					},
				},
			}
		}
		// the obj was not a namespace or it did not have the required label.
		return []reconcile.Request{}
	}
}
