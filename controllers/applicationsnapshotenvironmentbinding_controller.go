/*
Copyright 2022 Red Hat, Inc.

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

	gh "github.com/google/go-github/v41/github"
	gitopsgen "github.com/redhat-developer/gitops-generator/pkg"
	"go.uber.org/zap/zapcore"

	"github.com/go-logr/logr"
	logicalcluster "github.com/kcp-dev/logicalcluster/v2"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/gitopsjob"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// SnapshotEnvironmentBindingReconciler reconciles a SnapshotEnvironmentBinding object
type SnapshotEnvironmentBindingReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Log          logr.Logger
	AppFS        afero.Afero
	Generator    gitopsgen.Generator
	GitHubClient *gh.Client
	GitToken     string
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
	log := r.Log.WithValues("SnapshotEnvironmentBinding", req.NamespacedName).WithValues("clusterName", req.ClusterName)

	// if we're running on kcp, we need to include workspace in context
	if req.ClusterName != "" {
		ctx = logicalcluster.WithCluster(ctx, logicalcluster.New(req.ClusterName))
	}

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

	patch := client.MergeFrom(appSnapshotEnvBinding.DeepCopy())

	applicationName := appSnapshotEnvBinding.Spec.Application
	environmentName := appSnapshotEnvBinding.Spec.Environment
	snapshotName := appSnapshotEnvBinding.Spec.Snapshot

	// Check if the labels have been applied to the binding
	bindingLabels := appSnapshotEnvBinding.GetLabels()
	if bindingLabels["appstudio.application"] == "" || bindingLabels["appstudio.environment"] == "" {
		appSnapshotEnvBinding.SetLabels(map[string]string{
			"appstudio.application": applicationName,
			"appstudio.environment": environmentName,
		})

		if err := r.Client.Update(ctx, &appSnapshotEnvBinding); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Get the Environment CR
	environment := appstudiov1alpha1.Environment{}
	err = r.Get(ctx, types.NamespacedName{Name: environmentName, Namespace: appSnapshotEnvBinding.Namespace}, &environment)
	if err != nil {
		log.Error(err, fmt.Sprintf("unable to get the Environment %s %v", environmentName, req.NamespacedName))
		r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, patch, err)
		return ctrl.Result{}, err
	}

	// Get the Snapshot CR
	appSnapshot := appstudiov1alpha1.Snapshot{}
	err = r.Get(ctx, types.NamespacedName{Name: snapshotName, Namespace: appSnapshotEnvBinding.Namespace}, &appSnapshot)
	if err != nil {
		log.Error(err, fmt.Sprintf("unable to get the Application Snapshot %s %v", snapshotName, req.NamespacedName))
		r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, patch, err)
		return ctrl.Result{}, err
	}

	if appSnapshot.Spec.Application != applicationName {
		err := fmt.Errorf("application snapshot %s does not belong to the application %s", snapshotName, applicationName)
		log.Error(err, "")
		r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, patch, err)
		return ctrl.Result{}, err
	}

	// Launch the Kubernetes Job
	gitopsjobConfig := gitopsjob.GitOpsJobConfig{
		OperationType: "generate-overlays",
		ResourceName:  appSnapshotEnvBinding.GetName(),
	}

	// Generate a random name for the Job
	jobName := appSnapshotEnvBinding.GetName()
	if len(jobName) > 57 {
		jobName = appSnapshotEnvBinding.GetName()[0:56]
	}

	jobName = jobName + util.GetRandomString(5, true)
	err = gitopsjob.CreateGitOpsJob(ctx, r.Client, r.GitToken, jobName, appSnapshotEnvBinding.Namespace, gitopsjobConfig)
	if err != nil {
		r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, patch, err)
		return ctrl.Result{}, err
	}

	// Wait for the Job to succeed
	err = gitopsjob.WaitForJob(ctx, r.Client, jobName, 5*time.Minute)
	if err != nil {
		r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, patch, err)
		return ctrl.Result{}, err
	}

	r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, patch, nil)

	log.Info(fmt.Sprintf("Finished reconcile loop for %v", req.NamespacedName))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SnapshotEnvironmentBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := zap.Options{
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log.WithName("controllers").WithName("Environment").WithValues("appstudio-component", "HAS")
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.SnapshotEnvironmentBinding{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		// Watch for Environment CR updates and reconcile all the Bindings that reference the Environment
		Watches(&source.Kind{Type: &appstudiov1alpha1.Environment{}},
			handler.EnqueueRequestsFromMapFunc(MapToBindingByBoundObjectName(r.Client, "Environment", "appstudio.environment")), builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					log := log.WithValues("Namespace", e.Object.GetNamespace()).WithValues("clusterName", logicalcluster.From(e.Object).String())
					logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "Environment", logutil.ResourceCreate, nil)
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					log := log.WithValues("Namespace", e.ObjectNew.GetNamespace()).WithValues("clusterName", logicalcluster.From(e.ObjectNew).String())
					logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "Environment", logutil.ResourceUpdate, nil)
					return true
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					log := log.WithValues("Namespace", e.Object.GetNamespace()).WithValues("clusterName", logicalcluster.From(e.Object).String())
					logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "Environment", logutil.ResourceDelete, nil)
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			})).WithEventFilter(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			log := log.WithValues("Namespace", e.Object.GetNamespace()).WithValues("clusterName", logicalcluster.From(e.Object).String())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "SnapshotEnvironmentBinding", logutil.ResourceCreate, nil)
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := log.WithValues("Namespace", e.ObjectNew.GetNamespace()).WithValues("clusterName", logicalcluster.From(e.ObjectNew).String())
			logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "SnapshotEnvironmentBinding", logutil.ResourceUpdate, nil)
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log := log.WithValues("Namespace", e.Object.GetNamespace()).WithValues("clusterName", logicalcluster.From(e.Object).String())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "SnapshotEnvironmentBinding", logutil.ResourceDelete, nil)
			return false
		},
	}).
		Complete(r)
}
