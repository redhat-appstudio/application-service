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
	"path/filepath"

	gitopsgenv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
	gitopsgen "github.com/redhat-developer/gitops-generator/pkg"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	kcpclient "github.com/kcp-dev/apimachinery/pkg/client"
	"github.com/kcp-dev/logicalcluster"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
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
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ApplicationSnapshotEnvironmentBindingReconciler reconciles a ApplicationSnapshotEnvironmentBinding object
type ApplicationSnapshotEnvironmentBindingReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Log      logr.Logger
	AppFS    afero.Afero
	Executor gitopsgen.Executor
	GitToken string
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applicationsnapshotenvironmentbindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applicationsnapshotenvironmentbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applicationsnapshotenvironmentbindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applicationsnapshots,verbs=get;list;watch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=environments,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ApplicationSnapshotEnvironmentBinding object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ApplicationSnapshotEnvironmentBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("ApplicationSnapshotEnvironmentBinding", req.NamespacedName).WithValues("clusterName", req.ClusterName)
	ctx = kcpclient.WithCluster(ctx, logicalcluster.New(req.ClusterName))

	// Fetch the ApplicationSnapshotEnvironmentBinding instance
	var appSnapshotEnvBinding appstudioshared.ApplicationSnapshotEnvironmentBinding
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

	applicationName := appSnapshotEnvBinding.Spec.Application
	environmentName := appSnapshotEnvBinding.Spec.Environment
	snapshotName := appSnapshotEnvBinding.Spec.Snapshot
	components := appSnapshotEnvBinding.Spec.Components

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
	environment := appstudioshared.Environment{}
	err = r.Get(ctx, types.NamespacedName{Name: environmentName, Namespace: appSnapshotEnvBinding.Namespace}, &environment)
	if err != nil {
		log.Error(err, fmt.Sprintf("unable to get the Environment %s %v", environmentName, req.NamespacedName))
		r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, err)
		return ctrl.Result{}, err
	}

	// Get the Snapshot CR
	appSnapshot := appstudioshared.ApplicationSnapshot{}
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

		gitOpsRemoteURL, gitOpsBranch, gitOpsContext, err := util.ProcessGitOpsStatus(hasComponent.Status.GitOps, r.GitToken)
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
		gitopsgenBinding := gitopsgenv1alpha1.BindingComponentConfiguration{
			Name:      component.Name,
			Replicas:  component.Configuration.Replicas,
			Resources: component.Configuration.Resources,
			Env:       envVars,
		}

		environmentConfigEnvVars := make([]corev1.EnvVar, 0)
		for _, env := range environment.Spec.Configuration.Env {
			environmentConfigEnvVars = append(environmentConfigEnvVars, corev1.EnvVar{
				Name:  env.Name,
				Value: env.Value,
			})
		}

		gitopsgenEnv := gitopsgenv1alpha1.Environment{
			Spec: gitopsgenv1alpha1.EnvironmentSpec{
				Configuration: gitopsgenv1alpha1.EnvironmentConfiguration{
					Env: environmentConfigEnvVars,
				},
			},
		}
		err = gitopsgen.GenerateOverlaysAndPush(tempDir, clone, gitOpsRemoteURL, gitopsgenBinding, gitopsgenEnv, applicationName, environmentName, imageName, appSnapshotEnvBinding.Namespace, r.Executor, r.AppFS, gitOpsBranch, gitOpsContext, true, componentGeneratedResources)
		if err != nil {
			gitOpsErr := util.SanitizeErrorMessage(err)
			log.Error(gitOpsErr, fmt.Sprintf("unable to get generate gitops resources for %s %v", componentName, req.NamespacedName))
			r.AppFS.RemoveAll(tempDir) // not worried with an err, its a best case attempt to delete the temp clone dir
			r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, gitOpsErr)
			return ctrl.Result{}, gitOpsErr
		}

		// Retrieve the commit ID
		var commitID string
		repoPath := filepath.Join(tempDir, applicationName)
		if commitID, err = gitopsgen.GetCommitIDFromRepo(r.AppFS, r.Executor, repoPath); err != nil {
			gitOpsErr := util.SanitizeErrorMessage(err)
			log.Error(gitOpsErr, "unable to retrieve gitops repository commit id due to error")
			r.SetConditionAndUpdateCR(ctx, req, &appSnapshotEnvBinding, gitOpsErr)
			return ctrl.Result{}, gitOpsErr
		}

		if !isStatusUpdated {
			componentStatus := appstudioshared.ComponentStatus{
				Name: componentName,
				GitOpsRepository: appstudioshared.BindingComponentGitOpsRepository{
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
func (r *ApplicationSnapshotEnvironmentBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudioshared.ApplicationSnapshotEnvironmentBinding{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		// Watch for Environment CR updates and reconcile all the Bindings that reference the Environment
		Watches(&source.Kind{Type: &appstudioshared.Environment{}},
			handler.EnqueueRequestsFromMapFunc(MapToBindingByBoundObjectName(r.Client, "Environment", "appstudio.environment")), builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					return true
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			})).
		Complete(r)
}
