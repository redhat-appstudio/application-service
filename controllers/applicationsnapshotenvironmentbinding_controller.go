/*
Copyright 2021-2022 Red Hat, Inc.

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
	"net/url"
	"path/filepath"

	"github.com/go-logr/logr"
	appstudioshared "github.com/maysunfaisal/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ApplicationSnapshotEnvironmentBindingReconciler reconciles a ApplicationSnapshotEnvironmentBinding object
type ApplicationSnapshotEnvironmentBindingReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Log      logr.Logger
	AppFS    afero.Afero
	Executor gitops.Executor
	GitToken string
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applicationsnapshotenvironmentbindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applicationsnapshotenvironmentbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applicationsnapshotenvironmentbindings/finalizers,verbs=update

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
	log := r.Log.WithValues("Component", req.NamespacedName)

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

	log.Info(fmt.Sprintf("Starting reconcile loop for %v - %v", appSnapshotEnvBinding.Name, req.NamespacedName))

	applicationName := appSnapshotEnvBinding.Spec.Application
	environmentName := appSnapshotEnvBinding.Spec.Environment
	snapshotName := appSnapshotEnvBinding.Spec.Snapshot
	components := appSnapshotEnvBinding.Spec.Components

	// Create a temp folder to create the gitops resources in
	tempDir, err := ioutils.CreateTempPath(appSnapshotEnvBinding.Name, r.AppFS)
	if err != nil {
		log.Error(err, "unable to create temp directory for gitops resources due to error")
		r.SetCreateConditionAndUpdateCR(ctx, &appSnapshotEnvBinding, err)
		return ctrl.Result{}, fmt.Errorf("unable to create temp directory for gitops resources due to error: %v", err)
	}

	// Get the Snapshot CR
	appSnapshot := appstudioshared.ApplicationSnapshot{}
	err = r.Get(ctx, types.NamespacedName{Name: snapshotName, Namespace: appSnapshotEnvBinding.Namespace}, &appSnapshot)
	if err != nil {
		log.Error(err, fmt.Sprintf("unable to get the Application Snapshot %s %v", snapshotName, req.NamespacedName))
		return ctrl.Result{}, nil
	}

	if appSnapshot.Spec.Application != applicationName {
		err := fmt.Errorf("application snapshot %s does not belong to the application %s", snapshotName, applicationName)
		log.Error(err, "")
		r.SetCreateConditionAndUpdateCR(ctx, &appSnapshotEnvBinding, err)
		return ctrl.Result{}, nil
	}

	// Get the Application CR
	// hasApplication := appstudiov1alpha1.Application{}
	// err = r.Get(ctx, types.NamespacedName{Name: applicationName, Namespace: appSnapshotEnvBinding.Namespace}, &hasApplication)
	// if err != nil {
	// 	log.Error(err, fmt.Sprintf("unable to get the Application %s %v", applicationName, req.NamespacedName))
	// 	return ctrl.Result{}, nil
	// }

	// hasAppDevfileData, err := devfile.ParseDevfileModel(hasApplication.Status.Devfile)
	// if err != nil {
	// 	log.Error(err, fmt.Sprintf("Unable to parse the devfile from Application, exiting reconcile loop %v", req.NamespacedName))
	// 	return ctrl.Result{}, err
	// }

	// applicationDevfileMetadata := hasAppDevfileData.GetMetadata()
	// gitopsRepo := applicationDevfileMetadata.Attributes.GetString("gitOpsRepository.url", &err)
	// if err != nil {
	// 	log.Error(err, fmt.Sprintf("unable to get the gitops repo from Application %s %v", applicationName, req.NamespacedName))
	// 	return ctrl.Result{}, nil
	// }
	// gitopsBranch := applicationDevfileMetadata.Attributes.GetString("gitOpsRepository.branch", &err)
	// if err != nil {
	// 	log.Error(err, fmt.Sprintf("unable to get the gitops branch from Application %s %v", applicationName, req.NamespacedName))
	// }

	// if gitopsBranch == "" {
	// 	gitopsBranch = "main"
	// }

	// err = cloneAndCheckout(tempDir, gitopsRepo, gitopsBranch, applicationName, r.Executor)
	// if err != nil {
	// 	log.Error(err, "unable to clone and checkout gitops repo %s", "")
	// }

	var remoteURL, gitOpsURL, gitOpsBranch, gitOpsContext string

	for _, component := range components {
		componentName := component.Name

		// Get the Component CR
		hasComponent := appstudiov1alpha1.Component{}
		err = r.Get(ctx, types.NamespacedName{Name: componentName, Namespace: appSnapshotEnvBinding.Namespace}, &hasComponent)
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to get the Component %s %v", componentName, req.NamespacedName))
			r.SetCreateConditionAndUpdateCR(ctx, &appSnapshotEnvBinding, err)
			return ctrl.Result{}, nil
		}

		// Sanity check to make sure the binding component has referenced the correct application
		if hasComponent.Spec.Application != applicationName {
			err := fmt.Errorf("component %s does not belong to the application %s", componentName, applicationName)
			log.Error(err, "")
			r.SetCreateConditionAndUpdateCR(ctx, &appSnapshotEnvBinding, err)
			return ctrl.Result{}, nil
		}

		var clone bool
		var imageName string

		for _, snapshotComponent := range appSnapshot.Spec.Components {
			if snapshotComponent.Name == componentName {
				imageName = snapshotComponent.ContainerImage
			}
		}

		if imageName == "" {
			err := fmt.Errorf("application snapshot %s did not reference component %s", snapshotName, componentName)
			log.Error(err, "")
			r.SetCreateConditionAndUpdateCR(ctx, &appSnapshotEnvBinding, err)
			return ctrl.Result{}, nil
		}

		gitopsStatus := hasComponent.Status.GitOps
		// Get the information about the gitops repository from the Component resource
		gitOpsURL = gitopsStatus.RepositoryURL
		if gitOpsURL == "" {
			err := fmt.Errorf("unable to create gitops resource, GitOps Repository not set on component status")
			log.Error(err, "")
			r.SetCreateConditionAndUpdateCR(ctx, &appSnapshotEnvBinding, err)
			return ctrl.Result{}, nil
		}
		if gitopsStatus.Branch != "" {
			gitOpsBranch = gitopsStatus.Branch
		} else {
			gitOpsBranch = "main"
		}
		if gitopsStatus.Context != "" {
			gitOpsContext = gitopsStatus.Context
		} else {
			gitOpsContext = "/"
		}

		if remoteURL == "" {
			// Construct the remote URL for the gitops repository
			parsedURL, err := url.Parse(gitOpsURL)
			if err != nil {
				log.Error(err, "unable to parse gitops URL due to error")
				r.SetCreateConditionAndUpdateCR(ctx, &appSnapshotEnvBinding, err)
				return ctrl.Result{}, nil
			}
			parsedURL.User = url.User(r.GitToken)
			remoteURL = parsedURL.String()
			clone = true
		}

		isStatusUpdated := false
		for _, bindingStatusComponent := range appSnapshotEnvBinding.Status.Components {
			if bindingStatusComponent.Name == componentName {
				isStatusUpdated = true
			}
		}

		if !isStatusUpdated {
			appSnapshotEnvBinding.Status.Components = append(appSnapshotEnvBinding.Status.Components, appstudioshared.ComponentStatus{
				Name: componentName,
				GitOpsRepository: appstudioshared.BindingComponentGitOpsRepository{
					URL:    gitOpsURL,
					Branch: gitOpsBranch,
					Path:   filepath.Join(gitOpsContext, "components", componentName, "overlays", environmentName),
				},
			})
		}

		err = gitops.GenerateOverlaysAndPush(tempDir, clone, remoteURL, component, applicationName, environmentName, imageName, appSnapshotEnvBinding.Namespace, r.Executor, r.AppFS, gitOpsBranch, gitOpsContext)
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to get generate gitops resources for %s %v", componentName, req.NamespacedName))
			r.SetCreateConditionAndUpdateCR(ctx, &appSnapshotEnvBinding, err)
			return ctrl.Result{}, nil
		}

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
		r.SetCreateConditionAndUpdateCR(ctx, &appSnapshotEnvBinding, err)
		return ctrl.Result{}, err
	}

	r.SetCreateConditionAndUpdateCR(ctx, &appSnapshotEnvBinding, nil)

	log.Info(fmt.Sprintf("Finished reconcile loop for %v", req.NamespacedName))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationSnapshotEnvironmentBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&appstudioshared.ApplicationSnapshotEnvironmentBinding{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
