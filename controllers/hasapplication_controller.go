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

	gofakeit "github.com/brianvoe/gofakeit/v6"
	"github.com/go-logr/logr"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	util "github.com/redhat-appstudio/application-service/pkg/util"
)

// HASApplicationReconciler reconciles a HASApplication object
type HASApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=hasapplications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=hasapplications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=hasapplications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HASApplication object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *HASApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Get the HASApplication resource
	var hasApplication appstudiov1alpha1.HASApplication
	err := r.Get(ctx, req.NamespacedName, &hasApplication)
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

	// If devfile hasn't been generated yet, generate it
	// If the devfile hasn't been generated, the CR was just created.
	if hasApplication.Status.Devfile == "" {
		// See if a gitops/appModel repo(s) were passed in. If not, generate them.
		gitOpsRepo := hasApplication.Spec.GitOpsRepository.URL
		appModelRepo := hasApplication.Spec.AppModelRepository.URL
		if gitOpsRepo == "" && appModelRepo == "" {
			// If both repositories are blank, just generate a single shared repository
			repoName := util.GenerateNewRepository(hasApplication.Spec.DisplayName)
			gitOpsRepo = repoName
			appModelRepo = repoName
		} else if gitOpsRepo == "" {
			repoName := util.GenerateNewRepository(hasApplication.Spec.DisplayName)
			gitOpsRepo = repoName
		} else if appModelRepo == "" {
			repoName := util.GenerateNewRepository(hasApplication.Spec.DisplayName)
			appModelRepo = repoName
		}

		// Convert the devfile string to a devfile object
		devfileData, err := devfile.ConvertHASApplicationToDevfile(hasApplication, gitOpsRepo, appModelRepo)
		if err != nil {
			r.SetCreateConditionAndUpdateCR(ctx, &hasApplication, err)
			return reconcile.Result{}, err
		}
		yamlData, err := yaml.Marshal(devfileData)
		if err != nil {
			r.SetCreateConditionAndUpdateCR(ctx, &hasApplication, err)
			return reconcile.Result{}, err
		}

		hasApplication.Status.Devfile = string(yamlData)
		// Update the status of the CR
		r.SetCreateConditionAndUpdateCR(ctx, &hasApplication, nil)
	} else {
		// If the model already exists, see if either the displayname or description need updating
		// Get the devfile of the hasApp CR
		devfileData, err := devfile.ParseDevfileModel(hasApplication.Status.Devfile)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Update any specific fields that changed
		displayName := util.SanitizeDisplayName(hasApplication.Spec.DisplayName)
		description := hasApplication.Spec.Description
		devfileMeta := devfileData.GetMetadata()
		updateRequired := false
		if devfileMeta.Name != displayName {
			devfileMeta.Name = displayName
			updateRequired = true
		}
		if devfileMeta.Description != description {
			devfileMeta.Description = description
			updateRequired = true
		}
		if updateRequired {
			devfileData.SetMetadata(devfileMeta)

			// Update the hasApp CR with the new devfile
			yamlData, err := yaml.Marshal(devfileData)
			if err != nil {
				r.SetUpdateConditionAndUpdateCR(ctx, &hasApplication, err)
				return reconcile.Result{}, err
			}

			hasApplication.Status.Devfile = string(yamlData)
			r.SetUpdateConditionAndUpdateCR(ctx, &hasApplication, err)
		}
	}

	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *HASApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	gofakeit.New(0)

	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.HASApplication{}).
		Complete(r)
}
