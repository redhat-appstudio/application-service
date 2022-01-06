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
	"strconv"
	"strings"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	github "github.com/redhat-appstudio/application-service/pkg/github"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const appFinalizerName = "application.appstudio.redhat.com/finalizer"
const finalizeCount = "finalizeCount"

func (r *ApplicationReconciler) AddFinalizer(ctx context.Context, application *appstudiov1alpha1.Application) error {
	controllerutil.AddFinalizer(application, appFinalizerName)

	// Initialize the finalizer counter
	appAnnotations := application.ObjectMeta.GetAnnotations()
	if appAnnotations == nil {
		appAnnotations = make(map[string]string)
	}
	appAnnotations[finalizeCount] = "0"
	application.SetAnnotations(appAnnotations)
	return r.Update(ctx, application)
}

// Finalize deletes the corresponding GitOps repo for the given Application CR.
func (r *ApplicationReconciler) Finalize(application *appstudiov1alpha1.Application) error {
	// Get the GitOps repository URL
	devfileObj, err := devfile.ParseDevfileModel(application.Status.Devfile)
	if err != nil {
		return err
	}
	devfileGitOps := devfileObj.GetMetadata().Attributes.Get("gitOpsRepository.url", &err)
	if err != nil {
		return err
	}
	gitOpsURL := devfileGitOps.(string)

	// Only delete the GitOps repo if we created it.
	if strings.Contains(gitOpsURL, r.GitHubOrg) {
		repoName, err := github.GetRepoNameFromURL(gitOpsURL, r.GitHubOrg)
		if err != nil {
			return err
		}
		return github.DeleteRepository(r.GitHubClient, context.Background(), r.GitHubOrg, repoName)
	}
	return nil
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func getFinalizeCount(application *appstudiov1alpha1.Application) (int, error) {
	applicationAnnotations := application.GetAnnotations()
	if applicationAnnotations == nil {
		applicationAnnotations = make(map[string]string)
		applicationAnnotations[finalizeCount] = "0"
	}
	finalizeCountAnnotation := applicationAnnotations[finalizeCount]
	return strconv.Atoi(finalizeCountAnnotation)
}

func setFinalizeCount(application *appstudiov1alpha1.Application, count int) {
	applicationAnnotations := application.GetAnnotations()
	if applicationAnnotations == nil {
		applicationAnnotations = make(map[string]string)
	}
	applicationAnnotations[finalizeCount] = strconv.Itoa(count)
}
