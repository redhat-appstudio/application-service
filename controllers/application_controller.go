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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redhat-appstudio/application-service/pkg/metrics"

	gofakeit "github.com/brianvoe/gofakeit/v6"
	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	github "github.com/redhat-appstudio/application-service/pkg/github"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
	util "github.com/redhat-appstudio/application-service/pkg/util"
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
	log := r.Log.WithValues(applicationName, req.NamespacedName)

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

	ghClient, err := r.GitHubTokenClient.GetNewGitHubClient("")
	if err != nil {
		log.Error(err, "Unable to create Go-GitHub client due to error")
		return reconcile.Result{}, err
	}

	// Check if the Application CR is under deletion
	// If so: Remove the GitOps repo (if generated) and remove the finalizer.
	if application.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(application.GetFinalizers(), appFinalizerName) {
			// Attach the finalizer and return to reset the reconciler loop
			err := r.AddFinalizer(ctx, &application)
			return ctrl.Result{}, err
		}
	} else {
		if containsString(application.GetFinalizers(), appFinalizerName) {
			// A finalizer is present for the Application CR, so make sure we do the necessary cleanup steps
			if err := r.Finalize(&application, ghClient); err != nil {
				finalizeCounter, err := getCounterAnnotation(finalizeCount, &application)
				if err == nil && finalizeCounter < 5 {
					// The Finalize function failed, so increment the finalize count and return
					setCounterAnnotation(finalizeCount, &application, finalizeCounter+1)
					err := r.Update(ctx, &application)
					if err != nil {
						log.Error(err, "Error incrementing finalizer count on resource")
					}
					return ctrl.Result{}, nil
				} else {
					// if fail to delete the external dependency here, log the error, but don't return error
					// Don't want to get stuck in a cycle of repeatedly trying to delete the repository and failing
					log.Error(err, "Unable to delete GitOps repository for application %v in namespace %v", application.GetName(), application.GetNamespace())
				}

			}

			// remove the finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&application, appFinalizerName)
			if err := r.Update(ctx, &application); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	log.Info(fmt.Sprintf("Starting reconcile loop for %v", req.NamespacedName))
	// If devfile hasn't been generated yet, generate it
	// If the devfile hasn't been generated, the CR was just created.
	if application.Status.Devfile == "" {
		// See if a gitops/appModel repo(s) were passed in. If not, generate them.
		gitOpsRepo := application.Spec.GitOpsRepository.URL
		appModelRepo := application.Spec.AppModelRepository.URL
		if gitOpsRepo == "" {
			// If both repositories are blank, just generate a single shared repository
			uniqueHash := util.GenerateUniqueHashForWorkloadImageTag(application.Namespace)
			repoName := github.GenerateNewRepositoryName(application.Name, uniqueHash)

			// Generate the git repo in the redhat-appstudio-appdata org
			// Not an SLI metric.  Used for determining the number of git operation requests
			metricsLabel := prometheus.Labels{"controller": applicationName, "tokenName": ghClient.TokenName, "operation": "GenerateNewRepository"}
			metrics.ControllerGitRequest.With(metricsLabel).Inc()
			repoUrl, err := ghClient.GenerateNewRepository(ctx, r.GitHubOrg, repoName, "GitOps Repository")
			if err != nil {
				metrics.HandleRateLimitMetrics(err, metricsLabel)
				log.Error(err, fmt.Sprintf("Unable to create repository %v", repoUrl))
				r.SetCreateConditionAndUpdateCR(ctx, req, &application, err)
				return reconcile.Result{}, err
			}

			gitOpsRepo = repoUrl
		}
		if appModelRepo == "" {
			// If the appModelRepo is unset, just set it to the gitops repo
			appModelRepo = gitOpsRepo
		}

		// Convert the devfile string to a devfile object
		devfileData, err := devfile.ConvertApplicationToDevfile(application, gitOpsRepo, appModelRepo)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to convert Application CR to devfile, exiting reconcile loop %v", req.NamespacedName))
			r.SetCreateConditionAndUpdateCR(ctx, req, &application, err)
			return reconcile.Result{}, err
		}
		yamlData, err := yaml.Marshal(devfileData)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to marshall Application devfile, exiting reconcile loop %v", req.NamespacedName))
			r.SetCreateConditionAndUpdateCR(ctx, req, &application, err)
			return reconcile.Result{}, err
		}

		application.Status.Devfile = string(yamlData)

		// Create GitOps repository
		// Update the status of the CR
		r.SetCreateConditionAndUpdateCR(ctx, req, &application, nil)
	} else {
		// If the model already exists, see if either the displayname or description need updating
		// Get the devfile of the hasApp CR
		devfileSrc := devfile.DevfileSrc{
			Data: application.Status.Devfile,
		}
		devfileData, err := devfile.ParseDevfile(devfileSrc)
		if err != nil {
			r.SetUpdateConditionAndUpdateCR(ctx, req, &application, err)
			log.Error(err, fmt.Sprintf("Unable to parse devfile model, exiting reconcile loop %v", req.NamespacedName))
			return ctrl.Result{}, err
		}

		// Update any specific fields that changed
		displayName := application.Spec.DisplayName
		description := application.Spec.Description
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
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	gofakeit.New(0)
	opts := zap.Options{
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log.WithName("controllers").WithName("Application").WithValues("appstudio-component", "HAS")
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.Application{}).WithEventFilter(predicate.Funcs{
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
		Complete(r)
}
