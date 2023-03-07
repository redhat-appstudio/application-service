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

package gitops

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/redhat-developer/gitops-generator/pkg/resources"
	"github.com/redhat-developer/gitops-generator/pkg/yaml"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	gitopsprepare "github.com/redhat-appstudio/application-service/gitops/prepare"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	buildTriggerTemplateFileName = "trigger-template.yaml"
	buildEventListenerFileName   = "event-listener.yaml"
	buildWebhookRouteFileName    = "build-webhook-route.yaml"
	buildRepositoryFileName      = "pac-repository.yaml"

	PaCAnnotation                     = "pipelinesascode"
	GitProviderAnnotationName         = "git-provider"
	PipelinesAsCodeWebhooksSecretName = "pipelines-as-code-webhooks-secret"
	PipelinesAsCode_githubAppIdKey    = "github-application-id"
	PipelinesAsCode_githubPrivateKey  = "github-private-key"
)

func GenerateBuild(fs afero.Fs, outputFolder string, component appstudiov1alpha1.Component, gitopsConfig gitopsprepare.GitopsConfig) error {
	repository, err := GeneratePACRepository(component, gitopsConfig.PipelinesAsCodeCredentials)
	if err != nil {
		return err
	}

	buildResources := map[string]interface{}{
		buildRepositoryFileName: repository,
	}

	kustomize := resources.Kustomization{}
	for fileName := range buildResources {
		kustomize.AddResources(fileName)
	}

	buildResources[kustomizeFileName] = kustomize

	if _, err := yaml.WriteResources(fs, outputFolder, buildResources); err != nil {
		return err
	}
	return nil
}

func getBuildCommonLabelsForComponent(component *appstudiov1alpha1.Component) map[string]string {
	labels := map[string]string{
		"pipelines.appstudio.openshift.io/type": "build",
		"build.appstudio.openshift.io/build":    "true",
		"build.appstudio.openshift.io/type":     "build",
		"build.appstudio.openshift.io/version":  "0.1",
		"appstudio.openshift.io/component":      component.Name,
		"appstudio.openshift.io/application":    component.Spec.Application,
	}
	return labels
}

// GeneratePACRepository creates configuration of Pipelines as Code repository object.
func GeneratePACRepository(component appstudiov1alpha1.Component, config map[string][]byte) (*pacv1alpha1.Repository, error) {
	gitProvider, err := GetGitProvider(component)
	if err != nil {
		return nil, err
	}

	isAppUsed := IsPaCApplicationConfigured(gitProvider, config)

	var gitProviderConfig *pacv1alpha1.GitProvider = nil
	if !isAppUsed {
		// Webhook is used
		gitProviderConfig = &pacv1alpha1.GitProvider{
			Secret: &pacv1alpha1.Secret{
				Name: gitopsprepare.PipelinesAsCodeSecretName,
				Key:  GetProviderTokenKey(gitProvider),
			},
			WebhookSecret: &pacv1alpha1.Secret{
				Name: PipelinesAsCodeWebhooksSecretName,
				Key:  GetWebhookSecretKeyForComponent(component),
			},
		}

		if gitProvider == "gitlab" {
			gitProviderConfig.URL = "https://gitlab.com"
		}
	}

	repository := &pacv1alpha1.Repository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Repository",
			APIVersion: "pipelinesascode.tekton.dev/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        component.Name,
			Namespace:   component.Namespace,
			Annotations: getBuildCommonLabelsForComponent(&component),
		},
		Spec: pacv1alpha1.RepositorySpec{
			URL:         strings.TrimSuffix(strings.TrimSuffix(component.Spec.Source.GitSource.URL, ".git"), "/"),
			GitProvider: gitProviderConfig,
		},
	}

	return repository, nil
}

// GetProviderTokenKey returns key (field name) of the given provider access token in the Pipelines as Code k8s secret
func GetProviderTokenKey(gitProvider string) string {
	return gitProvider + ".token"
}

func GetWebhookSecretKeyForComponent(component appstudiov1alpha1.Component) string {
	gitRepoUrl := strings.TrimSuffix(component.Spec.Source.GitSource.URL, ".git")

	notAllowedCharRegex, _ := regexp.Compile("[^-._a-zA-Z0-9]{1}")
	return notAllowedCharRegex.ReplaceAllString(gitRepoUrl, "_")
}

// GetGitProvider returns git provider name based on the repository url, e.g. github, gitlab, etc or git-privider annotation
func GetGitProvider(component appstudiov1alpha1.Component) (string, error) {
	allowedGitProviders := map[string]bool{"github": true, "gitlab": true, "bitbucket": true}
	gitProvider := ""

	sourceUrl := component.Spec.Source.GitSource.URL

	if strings.HasPrefix(sourceUrl, "git@") {
		// git@github.com:redhat-appstudio/application-service.git
		sourceUrl = strings.TrimPrefix(sourceUrl, "git@")
		host := strings.Split(sourceUrl, ":")[0]
		gitProvider = strings.Split(host, ".")[0]
	} else {
		// https://github.com/redhat-appstudio/application-service
		u, err := url.Parse(sourceUrl)
		if err != nil {
			return "", err
		}
		uParts := strings.Split(u.Hostname(), ".")
		if len(uParts) == 1 {
			gitProvider = uParts[0]
		} else {
			gitProvider = uParts[len(uParts)-2]
		}
	}

	var err error
	if !allowedGitProviders[gitProvider] {
		// Self-hosted git provider, check for git-provider annotation on the component
		gitProviderAnnotationValue := component.GetAnnotations()[GitProviderAnnotationName]
		if gitProviderAnnotationValue != "" {
			if allowedGitProviders[gitProviderAnnotationValue] {
				gitProvider = gitProviderAnnotationValue
			} else {
				err = fmt.Errorf("unsupported \"%s\" annotation value: %s", GitProviderAnnotationName, gitProviderAnnotationValue)
			}
		} else {
			err = fmt.Errorf("self-hosted git provider is not specified via \"%s\" annotation in the component", GitProviderAnnotationName)
		}
	}

	return gitProvider, err
}

// IsPaCApplicationConfigured checks if Pipelines as Code credentials configured for given provider.
// Application is preffered over webhook if possible.
func IsPaCApplicationConfigured(gitProvider string, config map[string][]byte) bool {
	isAppUsed := false

	switch gitProvider {
	case "github":
		if len(config[PipelinesAsCode_githubAppIdKey]) != 0 || len(config[PipelinesAsCode_githubPrivateKey]) != 0 {
			isAppUsed = true
		}
	default:
		// Application is not supported
		isAppUsed = false
	}

	return isAppUsed
}
