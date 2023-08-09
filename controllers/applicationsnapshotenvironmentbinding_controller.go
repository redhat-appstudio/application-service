/*
Copyright 2022-2023 Red Hat, Inc.

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

	gitopsjoblib "github.com/redhat-appstudio/application-service/gitops-generator/pkg/generate"

	gitopsgen "github.com/redhat-developer/gitops-generator/pkg"
	"golang.org/x/exp/maps"

	"github.com/go-logr/logr"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	github "github.com/redhat-appstudio/application-service/pkg/github"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// SnapshotEnvironmentBindingReconciler reconciles a SnapshotEnvironmentBinding object
type SnapshotEnvironmentBindingReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Log               logr.Logger
	AppFS             afero.Afero
	Generator         gitopsgen.Generator
	GitHubTokenClient github.GitHubToken

	// DoLocalGitOpsGen determines whether or not to only spawn off gitops generation jobs, or to run them locally inside HAS. Defaults to false
	DoGitOpsJob bool

	// AllowLocalGitopsGen allows for certain resources to generate gitops resources locally, *if* an annotation is present on the resource. Defaults to false
	AllowLocalGitopsGen bool
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=snapshotenvironmentbindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=snapshotenvironmentbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=snapshotenvironmentbindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=snapshots,verbs=get;list;watch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=environments,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SnapshotEnvironmentBinding object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *SnapshotEnvironmentBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the SnapshotEnvironmentBinding instance
	var appSnapshotEnvBinding appstudiov1alpha1.SnapshotEnvironmentBinding
	err := r.Get(ctx, req.NamespacedName, &appSnapshotEnvBinding)
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

	log.Info(fmt.Sprintf("Starting reconcile loop for %v %v", appSnapshotEnvBinding.Name, req.NamespacedName))

	ghClient, err := r.GitHubTokenClient.GetNewGitHubClient("")
	if err != nil {
		log.Error(err, "Unable to create Go-GitHub client due to error")
		return reconcile.Result{}, err
	}

	// Add the Go-GitHub client name to the context
	ctx = context.WithValue(ctx, github.GHClientKey, ghClient.TokenName)

	applicationName := appSnapshotEnvBinding.Spec.Application
	environmentName := appSnapshotEnvBinding.Spec.Environment
	snapshotName := appSnapshotEnvBinding.Spec.Snapshot

	// Check if the labels have been applied to the binding
	requiredLabels := map[string]string{
		"appstudio.application": applicationName,
		"appstudio.environment": environmentName,
	}
	bindingLabels := appSnapshotEnvBinding.GetLabels()
	if bindingLabels["appstudio.application"] == "" || bindingLabels["appstudio.environment"] == "" {
		if bindingLabels != nil {
			maps.Copy(bindingLabels, requiredLabels)
		} else {
			bindingLabels = requiredLabels
		}
		appSnapshotEnvBinding.SetLabels(bindingLabels)
		if err := r.Client.Update(ctx, &appSnapshotEnvBinding); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Get the Environment CR
	environment := appstudiov1alpha1.Environment{}
	err = r.Get(ctx, types.NamespacedName{Name: environmentName, Namespace: appSnapshotEnvBinding.Namespace}, &environment)
	if err != nil {
		log.Error(err, fmt.Sprintf("unable to get the Environment %s %v", environmentName, req.NamespacedName))
		r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, err)
		return ctrl.Result{}, err
	}

	// Get the Snapshot CR
	appSnapshot := appstudiov1alpha1.Snapshot{}
	err = r.Get(ctx, types.NamespacedName{Name: snapshotName, Namespace: appSnapshotEnvBinding.Namespace}, &appSnapshot)
	if err != nil {
		log.Error(err, fmt.Sprintf("unable to get the Application Snapshot %s %v", snapshotName, req.NamespacedName))
		r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, err)
		return ctrl.Result{}, err
	}

	if appSnapshot.Spec.Application != applicationName {
		err := fmt.Errorf("application snapshot %s does not belong to the application %s", snapshotName, applicationName)
		log.Error(err, "")
		r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, err)
		return ctrl.Result{}, err
	}

	localGitopsGen := (!r.DoGitOpsJob) || (r.AllowLocalGitopsGen && appSnapshotEnvBinding.Annotations["allowLocalGitopsGen"] == "true")

	if localGitopsGen {
		err = gitopsjoblib.GenerateGitopsOverlays(context.Background(), log, r.Client, appSnapshotEnvBinding, ioutils.NewFilesystem(), gitopsjoblib.GitOpsGenParams{
			Generator: r.Generator,
			Token:     ghClient.Token,
		})
		if err != nil {
			r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, err)
			return ctrl.Result{}, err
		}
	}

	r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, nil)

	log.Info(fmt.Sprintf("Finished reconcile loop for %v", req.NamespacedName))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SnapshotEnvironmentBindingReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	log := ctrl.LoggerFrom(ctx).WithName("controllers").WithName("Environment")
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.SnapshotEnvironmentBinding{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		// Watch for Environment CR updates and reconcile all the Bindings that reference the Environment
		Watches(&source.Kind{Type: &appstudiov1alpha1.Environment{}},
			handler.EnqueueRequestsFromMapFunc(MapToBindingByBoundObjectName(r.Client, "Environment", "appstudio.environment")), builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					log := log.WithValues("namespace", e.Object.GetNamespace())
					logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "Environment", logutil.ResourceCreate, nil)
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					log := log.WithValues("namespace", e.ObjectNew.GetNamespace())
					logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "Environment", logutil.ResourceUpdate, nil)
					return true
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					log := log.WithValues("namespace", e.Object.GetNamespace())
					logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "Environment", logutil.ResourceDelete, nil)
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			})).WithEventFilter(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			log := log.WithValues("namespace", e.Object.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "SnapshotEnvironmentBinding", logutil.ResourceCreate, nil)
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := log.WithValues("namespace", e.ObjectNew.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "SnapshotEnvironmentBinding", logutil.ResourceUpdate, nil)
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log := log.WithValues("namespace", e.Object.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "SnapshotEnvironmentBinding", logutil.ResourceDelete, nil)
			return false
		},
	}).
		Complete(r)
}
