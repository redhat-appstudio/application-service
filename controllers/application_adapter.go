//
// Copyright 2023 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"context"
	"fmt"

	"github.com/devfile/api/v2/pkg/attributes"
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

type ApplicationAdapter struct {
	Application        *appstudiov1alpha1.Application
	NamespacedName     types.NamespacedName
	Components         []appstudiov1alpha1.Component
	GithubOrg          string
	GitHubClient       github.GitHubClient
	Client             client.Client
	Ctx                context.Context
	Log                logr.Logger
	GitOpsRepository   appstudiov1alpha1.ApplicationGitRepository
	AppModelRepository appstudiov1alpha1.ApplicationGitRepository
}

func (a *ApplicationAdapter) EnsureGitOpsRepoExists() (reconciler.OperationResult, error) {
	ghClient := a.GitHubClient
	log := a.Log

	// See if a gitops/appModel repo(s) were passed in. If not, generate them.
	gitOpsRepo := a.Application.Spec.GitOpsRepository.URL
	appModelRepo := a.Application.Spec.AppModelRepository.URL

	if a.Application.Status.Devfile != "" {
		gitOpsRepo, err := getRepoInfo(a.Application, "gitOpsRepository")
		if err != nil {
			log.Error(err, "Unable to get gitops repository from devfile model")
			a.SetConditionAndUpdateCR(err)
			return reconciler.RequeueWithError(err)
		}
		appModelRepo, err := getRepoInfo(a.Application, "appModelRepository")
		if err != nil {
			log.Error(err, "Unable to get appmodel repository from devfile model")
			a.SetConditionAndUpdateCR(err)
			return reconciler.RequeueWithError(err)
		}
		a.GitOpsRepository = gitOpsRepo
		a.AppModelRepository = appModelRepo
	} else {
		if gitOpsRepo == "" {
			// If both repositories are blank, just generate a single shared repository
			uniqueHash := util.GenerateUniqueHashForWorkloadImageTag(a.Application.Namespace)
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

			a.GitOpsRepository.URL = repoUrl
		} else {
			a.GitOpsRepository = appstudiov1alpha1.ApplicationGitRepository{
				URL:     gitOpsRepo,
				Context: a.Application.Spec.GitOpsRepository.Context,
				Branch:  a.Application.Spec.GitOpsRepository.Branch,
			}
		}
		if appModelRepo == "" {
			a.AppModelRepository.URL = a.GitOpsRepository.URL
		} else {
			a.AppModelRepository = appstudiov1alpha1.ApplicationGitRepository{
				URL:     appModelRepo,
				Context: a.Application.Spec.AppModelRepository.Context,
				Branch:  a.Application.Spec.AppModelRepository.Branch,
			}
		}
	}

	return reconciler.ContinueProcessing()
}

// EnsureApplicationDevfile is reponsible for ensuring the devfile in the Application's status is up to date
func (a *ApplicationAdapter) EnsureApplicationDevfile() (reconciler.OperationResult, error) {
	log := a.Log
	namespacedName := a.NamespacedName
	a.Application.Status.Devfile = ""

	// Convert the devfile string to a devfile object
	devfileData, err := devfile.ConvertApplicationToDevfile(*a.Application, a.GitOpsRepository, a.AppModelRepository)
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
func (a *ApplicationAdapter) EnsureApplicationStatus() (reconciler.OperationResult, error) {
	a.SetConditionAndUpdateCR(nil)
	return reconciler.ContinueProcessing()
}

// getRepoInfo retrieves the necessary information (repo url, branch and context) about the gitops repository
// Return values are: repository url, repository branch, repository context, and an error value
func getRepoInfo(application *appstudiov1alpha1.Application, repositoryLabel string) (appstudiov1alpha1.ApplicationGitRepository, error) {
	var repoInfo appstudiov1alpha1.ApplicationGitRepository
	// Get the devfile of the hasApp CR
	devfileSrc := devfile.DevfileSrc{
		Data: application.Status.Devfile,
	}
	devfileData, err := devfile.ParseDevfile(devfileSrc)
	if err != nil {
		return appstudiov1alpha1.ApplicationGitRepository{}, err
	}
	devfileAttributes := devfileData.GetMetadata().Attributes

	// Get the GitOps repository URL
	repoInfo.URL = devfileAttributes.GetString(repositoryLabel+".url", &err)
	if err != nil {
		return appstudiov1alpha1.ApplicationGitRepository{}, fmt.Errorf("unable to retrieve GitOps repository URL from Application CR devfile: %v", err)
	}

	// Get the GitOps repository branch
	repoInfo.Branch = devfileAttributes.GetString(repositoryLabel+".branch", &err)
	if err != nil {
		if _, ok := err.(*attributes.KeyNotFoundError); !ok {
			return appstudiov1alpha1.ApplicationGitRepository{}, err
		}
	}

	// Get the GitOps repository context
	repoInfo.Context = devfileAttributes.GetString(repositoryLabel+".context", &err)
	if err != nil {
		if _, ok := err.(*attributes.KeyNotFoundError); !ok {
			return appstudiov1alpha1.ApplicationGitRepository{}, err
		}
	}
	return repoInfo, nil
}
