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
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	github "github.com/redhat-appstudio/application-service/pkg/github"
	"github.com/redhat-appstudio/application-service/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const appFinalizerName = "application.appstudio.redhat.com/finalizer"
const finalizeCount = "finalizeCount"

// AddFinalizer adds the finalizer to the Application CR and initiates the finalize count on the annotation
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
func (r *ApplicationReconciler) Finalize(application *appstudiov1alpha1.Application, ghClient *github.GitHubClient) error {
	// Get the GitOps repository URL
	devfileSrc := devfile.DevfileSrc{
		Data: application.Status.Devfile,
	}
	devfileObj, err := devfile.ParseDevfile(devfileSrc)
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

		metricsLabel := prometheus.Labels{"controller": applicationName, "tokenName": ghClient.TokenName, "operation": "DeleteRepository"}
		metrics.ControllerGitRequest.With(metricsLabel).Inc()
		err = ghClient.DeleteRepository(context.Background(), r.GitHubOrg, repoName)
		metrics.HandleRateLimitMetrics(err, metricsLabel)
		return err

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

// getApplicationFailCount gets the given counter annotation on the resource (defaults to 0 if unset)
func getCounterAnnotation(annotation string, obj client.Object) (int, error) {
	objAnnotations := obj.GetAnnotations()
	if objAnnotations == nil || objAnnotations[annotation] == "" {
		objAnnotations = make(map[string]string)
		objAnnotations[annotation] = "0"
	}
	counterAnnotation := objAnnotations[annotation]
	return strconv.Atoi(counterAnnotation)
}

// setApplicationFailCount sets the given counter annotation on the resource to the specified value
func setCounterAnnotation(annotation string, obj client.Object, count int) {
	objAnnotations := obj.GetAnnotations()
	if objAnnotations == nil {
		objAnnotations = make(map[string]string)
	}
	objAnnotations[annotation] = strconv.Itoa(count)
}
