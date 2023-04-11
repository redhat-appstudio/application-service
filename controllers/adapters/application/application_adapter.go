package application

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/devfile"
	"github.com/redhat-appstudio/application-service/pkg/github"
	"github.com/redhat-appstudio/application-service/pkg/metrics"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/redhat-appstudio/operator-goodies/reconciler"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type Adapter struct {
	Application    *appstudiov1alpha1.Application
	NamespacedName types.NamespacedName
	Components     []appstudiov1alpha1.Component
	GithubOrg      string
	GitHubClient   github.GitHubClient
	Client         client.Client
	Ctx            context.Context
	Log            logr.Logger
}

func (a *Adapter) EnsureGitOpsRepoExists() (reconciler.OperationResult, error) {
	ghClient := a.GitHubClient
	log := a.Log

	// See if a gitops/appModel repo(s) were passed in. If not, generate them.
	gitOpsRepo := a.Application.Spec.GitOpsRepository.URL
	appModelRepo := a.Application.Spec.AppModelRepository.URL

	// ToDo: replaced with an API call to the GH API for the GitOps repository's existence
	if a.Application.Status.Devfile == "" {
		if gitOpsRepo == "" {
			// If both repositories are blank, just generate a single shared repository
			uniqueHash := util.GenerateUniqueHashForWorkloadImageTag("", a.Application.Namespace)
			repoName := github.GenerateNewRepositoryName(a.Application.Name, uniqueHash)

			// Generate the git repo in the redhat-appstudio-appdata org
			// Not an SLI metric.  Used for determining the number of git operation requests
			metricsLabel := prometheus.Labels{"controller": a.Application.Name, "tokenName": ghClient.TokenName, "operation": "GenerateNewRepository"}
			metrics.ControllerGitRequest.With(metricsLabel).Inc()
			repoUrl, err := ghClient.GenerateNewRepository(a.Ctx, a.GithubOrg, repoName, "GitOps Repository")
			if err != nil {
				metrics.HandleRateLimitMetrics(err, metricsLabel)
				log.Error(err, fmt.Sprintf("Unable to create repository %v", repoUrl))
				a.SetConditionAndUpdateCR(err)
				return reconciler.RequeueWithError(err)
			}

			a.Application.Spec.GitOpsRepository.URL = repoUrl
		}
		if appModelRepo == "" {
			a.Application.Spec.AppModelRepository.URL = a.Application.Spec.GitOpsRepository.URL
		}
	}

	return reconciler.ContinueProcessing()
}

// EnsureApplicationDevfile is reponsible for ensuring the devfile in the Application's status is up to date
func (a *Adapter) EnsureApplicationDevfile() (reconciler.OperationResult, error) {
	log := a.Log
	namespacedName := a.NamespacedName
	a.Application.Status.Devfile = ""

	// Convert the devfile string to a devfile object
	devfileData, err := devfile.ConvertApplicationToDevfile(*a.Application, a.Application.Spec.GitOpsRepository.URL, a.Application.Spec.AppModelRepository.URL)
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to convert Application CR to devfile, exiting reconcile loop %v", namespacedName))
		a.SetConditionAndUpdateCR(err)
		return reconciler.RequeueWithError(err)
	}

	// Add entries for the child Components that belong to the Application
	for _, component := range a.Components {
		err = updateApplicationDevfileModel(devfileData, component)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to add Component %q to the Application devfile model. %v", component.Name, namespacedName))
			a.SetConditionAndUpdateCR(err)
			return reconciler.RequeueWithError(err)
		}
	}

	// Marshal the Application's devfile data and set it in the Application status
	yamlData, err := yaml.Marshal(devfileData)
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to marshall Application devfile, exiting reconcile loop %v", namespacedName))
		a.SetConditionAndUpdateCR(err)
		return reconciler.RequeueWithError(err)
	}
	a.Application.Status.Devfile = string(yamlData)

	return reconciler.ContinueProcessing()
}

// EnsureApplicationStatus ensures that the status of the Application gets updated to 'Created/Updated'
func (a *Adapter) EnsureApplicationStatus() (reconciler.OperationResult, error) {
	a.SetConditionAndUpdateCR(nil)
	return reconciler.ContinueProcessing()
}
