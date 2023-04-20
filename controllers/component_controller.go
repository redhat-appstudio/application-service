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
	"time"

	"go.uber.org/zap/zapcore"
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

	"github.com/go-logr/logr"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	compAdapter "github.com/redhat-appstudio/application-service/controllers/adapters/component"
	github "github.com/redhat-appstudio/application-service/pkg/github"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"

	"github.com/redhat-appstudio/application-service/pkg/spi"
	"github.com/redhat-appstudio/operator-goodies/reconciler"
	gitopsgen "github.com/redhat-developer/gitops-generator/pkg"
	"github.com/spf13/afero"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

// ComponentReconciler reconciles a Component object
type ComponentReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Log               logr.Logger
	GitToken          string
	GitHubOrg         string
	ImageRepository   string
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
//+kubebuilder:rbac:groups=apis.kcp.dev,resources=apibindings,verbs=get;list;watch

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
	log := r.Log.WithValues("Component", req.NamespacedName)

	ghClient, err := r.GitHubTokenClient.GetNewGitHubClient()
	if err != nil {
		log.Error(err, "Unable to create Go-GitHub client due to error")
		return reconcile.Result{}, err
	}

	// Get the Component (and parent Application) associated with this reconcile
	component, application, compErr, appErr := r.GetComponentAndApplication(ctx, req)
	if compErr != nil {
		if k8sErrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Check if the Component CR is under deletion
	// If so: Remove the project from the Application devfile, remove the component dir from the Gitops repo and remove the finalizer.
	if appErr == nil && component.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(component.GetFinalizers(), compFinalizerName) {
			ownerReference := metav1.OwnerReference{
				APIVersion: application.APIVersion,
				Kind:       application.Kind,
				Name:       application.Name,
				UID:        application.UID,
			}
			component.SetOwnerReferences(append(component.GetOwnerReferences(), ownerReference))

			// Attach the finalizer and return to reset the reconciler loop
			err := r.AddFinalizer(ctx, &component)
			if err == nil {
				log.Info(fmt.Sprintf("added the finalizer %v", req.NamespacedName))
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err

		}
	} else {
		if application.Status.Devfile != "" && len(component.Status.Conditions) > 0 && component.Status.Conditions[len(component.Status.Conditions)-1].Status == metav1.ConditionTrue && containsString(component.GetFinalizers(), compFinalizerName) {
			// only attempt to finalize and update the gitops repo if an Application is present & the previous Component status is good
			// A finalizer is present for the Component CR, so make sure we do the necessary cleanup steps
			if err := r.Finalize(ctx, &component, &application, ghClient); err != nil {
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

	// Check that the application is ready before proceeding
	if appErr != nil || (application.Status.Devfile == "" && !containsString(component.GetFinalizers(), compFinalizerName)) {
		err = fmt.Errorf("application devfile model is empty. Before creating a Component, an instance of Application should be created %v", req.NamespacedName)
		return r.incrementCounterAndRequeue(log, ctx, req, &component, err)
	}
	setCounterAnnotation(applicationFailCounterAnnotation, &component, 0)

	log.Info(fmt.Sprintf("Starting reconcile loop for %v", req.NamespacedName))

	adapter := compAdapter.Adapter{
		Component:      &component,
		Application:    &application,
		NamespacedName: req.NamespacedName,
		Generator:      r.Generator,
		GitHubClient:   ghClient,
		Client:         r.Client,
		Ctx:            ctx,
		AppFS:          r.AppFS,
		SPIClient:      r.SPIClient,
		Log:            log,
	}

	// Reconcile the Application
	// Note: `adapter.EnsureApplicationIsReady` must be called before anything else requiring the Application resource
	return reconciler.ReconcileHandler([]reconciler.ReconcileOperation{
		adapter.EnsureComponentDevfile,
		adapter.EnsureComponentGitOpsResources,
		adapter.EnsureComponentStatus,
	})

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
			RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(time.Duration(500*time.Millisecond), time.Duration(60*time.Second)),
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
			return true
		},
	}).
		Complete(r)
}

// GetComponentAndApplication returns the component associated with the current reconcile, along with its parent application
func (r *ComponentReconciler) GetComponentAndApplication(ctx context.Context, req ctrl.Request) (appstudiov1alpha1.Component, appstudiov1alpha1.Application, error, error) {
	// Get the Component
	var component appstudiov1alpha1.Component
	err := r.Client.Get(ctx, req.NamespacedName, &component)
	if err != nil {
		return appstudiov1alpha1.Component{}, appstudiov1alpha1.Application{}, err, nil
	}

	// Get the Application
	var application appstudiov1alpha1.Application
	err = r.Get(ctx, types.NamespacedName{Name: component.Spec.Application, Namespace: req.Namespace}, &application)
	if err != nil {
		return component, appstudiov1alpha1.Application{}, nil, err
	}

	return component, application, nil, nil
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
