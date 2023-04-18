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
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redhat-appstudio/application-service/pkg/metrics"
	gitopsgenv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
	gitopsgen "github.com/redhat-developer/gitops-generator/pkg"
	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	github "github.com/redhat-appstudio/application-service/pkg/github"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
	"github.com/redhat-appstudio/application-service/pkg/util"
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
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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
}

const asebName = "SnapshotEnvironmentBinding"

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
	log := r.Log.WithValues(asebName, req.NamespacedName)

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

	applicationName := appSnapshotEnvBinding.Spec.Application
	environmentName := appSnapshotEnvBinding.Spec.Environment
	snapshotName := appSnapshotEnvBinding.Spec.Snapshot
	components := appSnapshotEnvBinding.Spec.Components

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

	componentGeneratedResources := make(map[string][]string)
	var tempDir string
	clone := true

	for _, component := range components {
		componentName := component.Name

		// Get the Component CR
		hasComponent := appstudiov1alpha1.Component{}
		err = r.Get(ctx, types.NamespacedName{Name: componentName, Namespace: appSnapshotEnvBinding.Namespace}, &hasComponent)
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to get the Component %s %v", componentName, req.NamespacedName))
			r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, err)
			return ctrl.Result{}, err
		}

		if hasComponent.Spec.SkipGitOpsResourceGeneration {
			continue
		}

		// Sanity check to make sure the binding component has referenced the correct application
		if hasComponent.Spec.Application != applicationName {
			err := fmt.Errorf("component %s does not belong to the application %s", componentName, applicationName)
			log.Error(err, "")
			r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, err)
			return ctrl.Result{}, err
		}

		var imageName string

		for _, snapshotComponent := range appSnapshot.Spec.Components {
			if snapshotComponent.Name == componentName {
				imageName = snapshotComponent.ContainerImage
				break
			}
		}

		if imageName == "" {
			err := fmt.Errorf("application snapshot %s did not reference component %s", snapshotName, componentName)
			log.Error(err, "")
			r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, err)
			return ctrl.Result{}, err
		}

		gitOpsRemoteURL, gitOpsBranch, gitOpsContext, err := util.ProcessGitOpsStatus(hasComponent.Status.GitOps, ghClient.Token)
		if err != nil {
			r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, err)
			return ctrl.Result{}, err
		}

		isStatusUpdated := false
		for _, bindingStatusComponent := range appSnapshotEnvBinding.Status.Components {
			if bindingStatusComponent.Name == componentName {
				isStatusUpdated = true
				break
			}
		}

		if clone {
			// Create a temp folder to create the gitops resources in
			tempDir, err = ioutils.CreateTempPath(appSnapshotEnvBinding.Name, r.AppFS)
			if err != nil {
				log.Error(err, "unable to create temp directory for gitops resources due to error")
				r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, err)
				return ctrl.Result{}, fmt.Errorf("unable to create temp directory for gitops resources due to error: %v", err)
			}
		}

		envVars := make([]corev1.EnvVar, 0)
		for _, env := range component.Configuration.Env {
			envVars = append(envVars, corev1.EnvVar{
				Name:  env.Name,
				Value: env.Value,
			})
		}

		environmentConfigEnvVars := make([]corev1.EnvVar, 0)
		for _, env := range environment.Spec.Configuration.Env {
			environmentConfigEnvVars = append(environmentConfigEnvVars, corev1.EnvVar{
				Name:  env.Name,
				Value: env.Value,
			})
		}
		componentResources := corev1.ResourceRequirements{}
		if component.Configuration.Resources != nil {
			componentResources = *component.Configuration.Resources
		}

		kubeLabels := map[string]string{
			"app.kubernetes.io/name":       componentName,
			"app.kubernetes.io/instance":   component.Name,
			"app.kubernetes.io/part-of":    applicationName,
			"app.kubernetes.io/managed-by": "kustomize",
			"app.kubernetes.io/created-by": "application-service",
		}
		genOptions := gitopsgenv1alpha1.GeneratorOptions{
			Name:          component.Name,
			Replicas:      component.Configuration.Replicas,
			Resources:     componentResources,
			BaseEnvVar:    envVars,
			OverlayEnvVar: environmentConfigEnvVars,
			K8sLabels:     kubeLabels,
		}
		//Gitops functions return sanitized error messages
		metrics.ControllerGitRequest.With(prometheus.Labels{"controller": asebName, "tokenName": ghClient.TokenName, "operation": "GenerateOverlaysAndPush"}).Inc()
		err = r.Generator.GenerateOverlaysAndPush(tempDir, clone, gitOpsRemoteURL, genOptions, applicationName, environmentName, imageName, appSnapshotEnvBinding.Namespace, r.AppFS, gitOpsBranch, gitOpsContext, true, componentGeneratedResources)
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to get generate gitops resources for %s %v", componentName, req.NamespacedName))
			_ = r.AppFS.RemoveAll(tempDir) // not worried with an err, its a best case attempt to delete the temp clone dir
			r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, err)
			return ctrl.Result{}, err
		}

		// Retrieve the commit ID
		var commitID string
		repoName, orgName, err := github.GetRepoAndOrgFromURL(gitOpsRemoteURL)
		if err != nil {
			gitOpsErr := &GitOpsParseRepoError{gitOpsRemoteURL, err}
			log.Error(gitOpsErr, "")
			r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, gitOpsErr)
			return ctrl.Result{}, gitOpsErr
		}

		metricsLabel := prometheus.Labels{"controller": asebName, "tokenName": ghClient.TokenName, "operation": "GetLatestCommitSHAFromRepository"}
		metrics.ControllerGitRequest.With(metricsLabel).Inc()
		commitID, err = ghClient.GetLatestCommitSHAFromRepository(ctx, repoName, orgName, gitOpsBranch)
		if err != nil {
			metrics.HandleRateLimitMetrics(err, metricsLabel)
			gitOpsErr := &GitOpsCommitIdError{err}
			log.Error(gitOpsErr, "")
			r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, gitOpsErr)
			return ctrl.Result{}, gitOpsErr
		}

		if !isStatusUpdated {
			componentStatus := appstudiov1alpha1.BindingComponentStatus{
				Name: componentName,
				GitOpsRepository: appstudiov1alpha1.BindingComponentGitOpsRepository{
					URL:      hasComponent.Status.GitOps.RepositoryURL,
					Branch:   gitOpsBranch,
					Path:     filepath.Join(gitOpsContext, "components", componentName, "overlays", environmentName),
					CommitID: commitID,
				},
			}

			if _, ok := componentGeneratedResources[componentName]; ok {
				componentStatus.GitOpsRepository.GeneratedResources = componentGeneratedResources[componentName]
			}

			appSnapshotEnvBinding.Status.Components = append(appSnapshotEnvBinding.Status.Components, componentStatus)
		}

		// Set the clone to false, since we dont want to clone the repo again for the other components
		clone = false
	}

	// Remove the cloned path
	err = r.AppFS.RemoveAll(tempDir)
	if err != nil {
		log.Error(err, "Unable to remove the clone dir")
	}

	// Update the binding status to reflect the GitOps data
	err = r.Client.Status().Update(ctx, &appSnapshotEnvBinding)
	if err != nil {
		log.Error(err, "Unable to update App Snapshot Env Binding")
		r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, err)
		return ctrl.Result{}, err
	}

	r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, nil)

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
					log := log.WithValues("Namespace", e.Object.GetNamespace())
					logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "Environment", logutil.ResourceCreate, nil)
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					log := log.WithValues("Namespace", e.ObjectNew.GetNamespace())
					logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "Environment", logutil.ResourceUpdate, nil)
					return true
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					log := log.WithValues("Namespace", e.Object.GetNamespace())
					logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "Environment", logutil.ResourceDelete, nil)
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			})).WithEventFilter(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			log := log.WithValues("Namespace", e.Object.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "SnapshotEnvironmentBinding", logutil.ResourceCreate, nil)
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := log.WithValues("Namespace", e.ObjectNew.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "SnapshotEnvironmentBinding", logutil.ResourceUpdate, nil)
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log := log.WithValues("Namespace", e.Object.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "SnapshotEnvironmentBinding", logutil.ResourceDelete, nil)
			return false
		},
	}).
		Complete(r)
}
