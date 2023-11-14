//
// Copyright 2021-2023 Red Hat, Inc.
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
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	gitopsgenv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/devfile/library/v2/pkg/devfile/parser"
)

var RevisionHistoryLimit = int32(0)

func SanitizeName(name string) string {
	sanitizedName := strings.ToLower(strings.Replace(strings.Replace(name, " ", "-", -1), "'", "", -1))
	if len(sanitizedName) > 50 {
		sanitizedName = sanitizedName[0:50]
	}

	return sanitizedName
}

// GetIntValue returns the value of an int pointer, with the default of 0 if nil
func GetIntValue(intPtr *int) int {
	if intPtr != nil {
		return *intPtr
	}

	return 0
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

// ValidateEndpoint validates if the endpoint url can be parsed and if it has a host and a scheme
func ValidateEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse the url: %v, err: %v", endpoint, err)
	}

	if len(u.Host) == 0 || len(u.Scheme) == 0 {
		return fmt.Errorf("url %v is invalid", endpoint)
	}

	return nil
}

// CheckWithRegex checks if a name matches the pattern.
// If a pattern fails to compile, it returns false
func CheckWithRegex(pattern, name string) bool {
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	return reg.MatchString(name)
}

const schemaBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// GetRandomString returns a random string which is n characters long.
// If lower is set to true a lower case string is returned.
func GetRandomString(n int, lower bool) string {
	b := make([]byte, n)
	for i := range b {
		/* #nosec G404 -- not used for cryptographic purposes*/
		b[i] = schemaBytes[rand.Intn(len(schemaBytes)-1)]
	}
	randomString := string(b)
	if lower {
		randomString = strings.ToLower(randomString)
	}
	return randomString
}

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
		Name:                 component.ObjectMeta.Name,
		Application:          component.Spec.Application,
		Secret:               component.Spec.Secret,
		Resources:            component.Spec.Resources,
		Replicas:             GetIntValue(component.Spec.Replicas),
		TargetPort:           component.Spec.TargetPort,
		Route:                component.Spec.Route,
		BaseEnvVar:           component.Spec.Env,
		ContainerImage:       component.Spec.ContainerImage,
		K8sLabels:            customK8sLabels,
		RevisionHistoryLimit: &RevisionHistoryLimit,
	}
	if component.Spec.Source.ComponentSourceUnion.GitSource != nil {
		gitopsMapComponent.GitSource = &gitopsgenv1alpha1.GitSource{
			URL: component.Spec.Source.ComponentSourceUnion.GitSource.URL,
		}
	} else {
		gitopsMapComponent.GitSource = &gitopsgenv1alpha1.GitSource{}
	}

	// If the resource requests or limits were unset, set default values
	if gitopsMapComponent.Resources.Requests == nil {
		gitopsMapComponent.Resources.Requests = v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse("10m"),
			v1.ResourceMemory: resource.MustParse("50Mi"),
		}
	}
	if gitopsMapComponent.Resources.Limits == nil {
		gitopsMapComponent.Resources.Limits = v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse("1"),
			v1.ResourceMemory: resource.MustParse("512Mi"),
		}
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

// GenerateUniqueHashForWorkloadImageTag generates a unique hash from the namespace
// in order to not expose user's username via namespace of a particular resource potentially.
func GenerateUniqueHashForWorkloadImageTag(namespace string) string {
	h := sha256.New()
	h.Write([]byte(namespace))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))[0:5]
}

// GenerateRandomRouteName returns a random, trimmed route name based on the Component name based on the following criteria
// 1. Under 30 characters
// 2. Contains 4 random characters
func GenerateRandomRouteName(componentName string) string {
	routeName := componentName
	if len(componentName) > 25 {
		routeName = componentName[0:25]
	} else {
		routeName = componentName
	}

	// Append random characters to the route name
	routeName = routeName + GetRandomString(4, true)
	return routeName
}

// StrInList returns true if the given string is present in strList
func StrInList(str string, strList []string) bool {
	for _, val := range strList {
		if str == val {
			return true
		}
	}
	return false
}
