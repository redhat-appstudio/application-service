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

package util

import (
	"fmt"
	"net/url"
	"reflect"

	"github.com/devfile/library/v2/pkg/devfile/parser"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	gitopsgenv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
)

// GetMappedGitOpsComponent gets a mapped GeneratorOptions from the Component for GitOps resource generation
func GetMappedGitOpsComponent(component appstudiov1alpha1.Component, kubernetesResources parser.KubernetesResources) gitopsgenv1alpha1.GeneratorOptions {
	customK8sLabels := map[string]string{
		"app.kubernetes.io/name":       component.Spec.ComponentName,
		"app.kubernetes.io/instance":   component.Name,
		"app.kubernetes.io/part-of":    component.Spec.Application,
		"app.kubernetes.io/managed-by": "kustomize",
		"app.kubernetes.io/created-by": "application-service",
	}
	gitopsMapComponent := gitopsgenv1alpha1.GeneratorOptions{
		Name:           component.ObjectMeta.Name,
		Namespace:      component.ObjectMeta.Namespace,
		Application:    component.Spec.Application,
		Secret:         component.Spec.Secret,
		Resources:      component.Spec.Resources,
		Replicas:       component.Spec.Replicas,
		TargetPort:     component.Spec.TargetPort,
		Route:          component.Spec.Route,
		BaseEnvVar:     component.Spec.Env,
		ContainerImage: component.Spec.ContainerImage,
		K8sLabels:      customK8sLabels,
	}
	if component.Spec.Source.ComponentSourceUnion.GitSource != nil {
		gitopsMapComponent.GitSource = &gitopsgenv1alpha1.GitSource{
			URL: component.Spec.Source.ComponentSourceUnion.GitSource.URL,
		}
	} else {
		gitopsMapComponent.GitSource = &gitopsgenv1alpha1.GitSource{}
	}

	if !reflect.DeepEqual(kubernetesResources, parser.KubernetesResources{}) {
		gitopsMapComponent.KubernetesResources.Deployments = append(gitopsMapComponent.KubernetesResources.Deployments, kubernetesResources.Deployments...)
		gitopsMapComponent.KubernetesResources.Services = append(gitopsMapComponent.KubernetesResources.Services, kubernetesResources.Services...)
		gitopsMapComponent.KubernetesResources.Routes = append(gitopsMapComponent.KubernetesResources.Routes, kubernetesResources.Routes...)
		gitopsMapComponent.KubernetesResources.Ingresses = append(gitopsMapComponent.KubernetesResources.Ingresses, kubernetesResources.Ingresses...)
		gitopsMapComponent.KubernetesResources.Others = append(gitopsMapComponent.KubernetesResources.Others, kubernetesResources.Others...)
	}

	return gitopsMapComponent
}

func GetRemoteURL(gitOpsURL string, gitToken string) (string, error) {
	parsedURL, err := url.Parse(gitOpsURL)
	if err != nil {
		return "", err
	}
	parsedURL.User = url.User(gitToken)
	remoteURL := parsedURL.String()
	return remoteURL, nil
}

// ProcessGitOpsStatus processes the GitOps status and returns the remote url, branch, context and the error
func ProcessGitOpsStatus(gitopsStatus appstudiov1alpha1.GitOpsStatus, gitToken string) (string, string, string, error) {
	var gitOpsURL, gitOpsBranch, gitOpsContext string
	gitOpsURL = gitopsStatus.RepositoryURL
	if gitOpsURL == "" {
		err := fmt.Errorf("unable to process GitOps status, GitOps Repository URL cannot be empty")
		return "", "", "", err
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

	// Construct the remote URL for the gitops repository
	parsedURL, err := url.Parse(gitOpsURL)
	if err != nil {
		return "", "", "", err
	}
	parsedURL.User = url.User(gitToken)
	remoteURL := parsedURL.String()

	return remoteURL, gitOpsBranch, gitOpsContext, nil
}
