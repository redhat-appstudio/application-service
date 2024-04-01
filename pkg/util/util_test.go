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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/devfile/library/v2/pkg/devfile/parser"
	gitopsgenv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeDisplayName(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		want        string
	}{
		{
			name:        "Simple display name, no spaces",
			displayName: "PetClinic",
			want:        "petclinic",
		},
		{
			name:        "Simple display name, with space",
			displayName: "PetClinic App",
			want:        "petclinic-app",
		},
		{
			name:        "Longer display name, multiple spaces",
			displayName: "Pet Clinic Application",
			want:        "pet-clinic-application",
		},
		{
			name:        "Very long display name",
			displayName: "Pet Clinic Application Super Super Long Display name",
			want:        "pet-clinic-application-super-super-long-display-na",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitizedName := SanitizeName(tt.displayName)
			// Unexpected error
			if sanitizedName != tt.want {
				t.Errorf("SanitizeName() error: expected %v got %v", tt.want, sanitizedName)
			}
		})
	}
}

func TestProcessGitOpsStatus(t *testing.T) {
	tests := []struct {
		name         string
		gitopsStatus appstudiov1alpha1.GitOpsStatus
		gitToken     string
		wantURL      string
		wantBranch   string
		wantContext  string
		wantErr      bool
	}{
		{
			name: "gitops status processed as expected",
			gitopsStatus: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "https://github.com/myrepo",
				Branch:        "notmain",
				Context:       "context",
			},
			gitToken:    "token",
			wantURL:     "https://token@github.com/myrepo",
			wantBranch:  "notmain",
			wantContext: "context",
		},
		{
			name: "gitops url is empty",
			gitopsStatus: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "",
			},
			wantErr: true,
		},
		{
			name: "gitops branch and context not set",
			gitopsStatus: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "https://github.com/myrepo",
			},
			gitToken:    "token",
			wantURL:     "https://token@github.com/myrepo",
			wantBranch:  "main",
			wantContext: "/",
		},
		{
			name: "gitops url parse err",
			gitopsStatus: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "http://foo.com/?foo\nbar",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitopsURL, gitopsBranch, gitopsContext, err := ProcessGitOpsStatus(tt.gitopsStatus, tt.gitToken)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else {
				assert.Equal(t, tt.wantURL, gitopsURL, "should be equal")
				assert.Equal(t, tt.wantBranch, gitopsBranch, "should be equal")
				assert.Equal(t, tt.wantContext, gitopsContext, "should be equal")
			}
		})
	}
}

func TestValidateEndpoint(t *testing.T) {
	parseFail := "failed to parse the url"

	tests := []struct {
		name    string
		url     string
		wantErr *string
	}{
		{
			name: "Valid Endpoint",
			url:  "https://google.ca",
		},
		{
			name: "Valid private repo",
			url:  "https://github.com/devfile-resources/multi-components-private",
		},
		{
			name:    "Invalid URL failed to be parsed",
			url:     "\000x",
			wantErr: &parseFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEndpoint(tt.url)
			if tt.wantErr != nil && (err == nil) {
				t.Error("wanted error but got nil")
				return
			} else if tt.wantErr == nil && err != nil {
				t.Errorf("got unexpected error %v", err)
				return
			}
			if tt.wantErr != nil {
				assert.Regexp(t, *tt.wantErr, err.Error(), "TestValidateEndpoint: Error message does not match")
			}
		})
	}
}

func TestCheckWithRegex(t *testing.T) {
	tests := []struct {
		name      string
		test      string
		pattern   string
		wantMatch bool
	}{
		{
			name:      "matching string",
			test:      "hi-00-HI",
			pattern:   "^[a-z]([-a-z0-9]*[a-z0-9])?",
			wantMatch: true,
		},
		{
			name:      "not a matching string",
			test:      "1-hi",
			pattern:   "^[a-z]([-a-z0-9]*[a-z0-9])?",
			wantMatch: false,
		},
		{
			name:      "bad pattern",
			test:      "hi-00-HI",
			pattern:   "(abc",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch := CheckWithRegex(tt.pattern, tt.test)
			assert.Equal(t, tt.wantMatch, gotMatch, "the values should match")
		})
	}
}

func TestGetRandomString(t *testing.T) {
	tests := []struct {
		name   string
		length int
		lower  bool
	}{
		{
			name:   "all lower case string",
			length: 5,
			lower:  true,
		},
		{
			name:   "contain upper case string",
			length: 10,
			lower:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotString := GetRandomString(tt.length, tt.lower)
			assert.Equal(t, tt.length, len(gotString), "the values should match")

			if tt.lower == true {
				assert.Equal(t, strings.ToLower(gotString), gotString, "the values should match")
			}

			gotString2 := GetRandomString(tt.length, tt.lower)
			assert.NotEqual(t, gotString, gotString2, "the two random string should not be the same")
		})
	}
}

func TestGetMappedComponent(t *testing.T) {

	other := make([]interface{}, 1)
	other[0] = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deployment1",
		},
	}

	tests := []struct {
		name                string
		component           appstudiov1alpha1.Component
		kubernetesResources parser.KubernetesResources
		want                gitopsgenv1alpha1.GeneratorOptions
	}{
		{
			name: "Test01ComponentSpecFilledIn",
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testcomponent",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest001",
					Secret:        "Secret",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceLimitsCPU: resource.MustParse("1"),
							corev1.ResourceMemory:    resource.MustParse("1Gi"),
						},
					},
					TargetPort: 8080,
					Route:      "https://testroute",
					Env: []corev1.EnvVar{
						{
							Name:  "env1",
							Value: "env1Value",
						},
					},
					ContainerImage:               "myimage:image",
					SkipGitOpsResourceGeneration: false,
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL:           "https://host/git-repo.git",
								Revision:      "1.0",
								Context:       "/context",
								DevfileURL:    "https://mydevfileurl",
								DockerfileURL: "https://mydockerfileurl",
							},
						},
					},
				},
			},
			want: gitopsgenv1alpha1.GeneratorOptions{
				Name:        "testcomponent",
				Application: "AppTest001",
				Secret:      "Secret",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceLimitsCPU: resource.MustParse("1"),
						corev1.ResourceMemory:    resource.MustParse("1Gi"),
					},
				},
				TargetPort: 8080,
				Route:      "https://testroute",
				BaseEnvVar: []corev1.EnvVar{
					{
						Name:  "env1",
						Value: "env1Value",
					},
				},
				ContainerImage: "myimage:image",
				GitSource: &gitopsgenv1alpha1.GitSource{
					URL: "https://host/git-repo.git",
				},
			},
		},
		{
			name: "Test02EmptyComponentSource",
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testcomponent",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest002",
					Source:        appstudiov1alpha1.ComponentSource{},
				},
			},
			want: gitopsgenv1alpha1.GeneratorOptions{
				Name:        "testcomponent",
				Application: "AppTest002",
				GitSource:   &gitopsgenv1alpha1.GitSource{},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			},
		},
		{
			name: "Test03NoSource",
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testcomponent",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest003",
				},
			},
			want: gitopsgenv1alpha1.GeneratorOptions{
				Name:        "testcomponent",
				Application: "AppTest003",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			},
		},
		{
			name: "Test04EmptyComponentSourceUnion",
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testcomponent",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest004",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{},
					},
				},
			},
			want: gitopsgenv1alpha1.GeneratorOptions{
				Name:        "testcomponent",
				Application: "AppTest004",
				GitSource:   &gitopsgenv1alpha1.GitSource{},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			},
		},
		{
			name: "Test05EmptyGitSource",
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testcomponent",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest005",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{},
						},
					},
				},
			},
			want: gitopsgenv1alpha1.GeneratorOptions{
				Name:        "testcomponent",
				Application: "AppTest005",
				GitSource:   &gitopsgenv1alpha1.GitSource{},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			},
		},
		{
			name: "Test06KubernetesResources",
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testcomponent",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "frontEnd",
					Application:   "AppTest005",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "url",
							},
						},
					},
				},
			},
			kubernetesResources: parser.KubernetesResources{
				Deployments: []appsv1.Deployment{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "deployment1",
						},
					},
				},
				Services: []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "service1",
						},
					},
				},
				Others: other,
			},
			want: gitopsgenv1alpha1.GeneratorOptions{
				Name:        "testcomponent",
				Application: "AppTest005",
				GitSource: &gitopsgenv1alpha1.GitSource{
					URL: "url",
				},
				KubernetesResources: gitopsgenv1alpha1.KubernetesResources{
					Deployments: []appsv1.Deployment{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "deployment1",
							},
						},
					},
					Services: []corev1.Service{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "service1",
							},
						},
					},
					Others: other,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappedComponent := GetMappedGitOpsComponent(tt.component, tt.kubernetesResources)
			assert.True(t, tt.want.Name == mappedComponent.Name, "Expected ObjectMeta.Name: %s, is different than actual: %s", tt.want.Name, mappedComponent.Name)
			assert.True(t, tt.want.Application == mappedComponent.Application, "Expected Spec.Application: %s, is different than actual: %s", tt.want.Application, mappedComponent.Application)
			assert.True(t, tt.want.Secret == mappedComponent.Secret, "Expected Spec.Secret: %s, is different than actual: %s", tt.want.Secret, mappedComponent.Secret)
			assert.True(t, reflect.DeepEqual(tt.want.Resources, mappedComponent.Resources), "Expected Spec.Resources: %s, is different than actual: %s", tt.want.Resources, mappedComponent.Resources)
			assert.True(t, tt.want.Route == mappedComponent.Route, "Expected Spec.Route: %s, is different than actual: %s", tt.want.Route, mappedComponent.Route)
			assert.True(t, reflect.DeepEqual(tt.want.BaseEnvVar, mappedComponent.BaseEnvVar), "Expected Spec.Env: %s, is different than actual: %s", tt.want.BaseEnvVar, mappedComponent.BaseEnvVar)
			assert.True(t, tt.want.ContainerImage == mappedComponent.ContainerImage, "Expected Spec.ContainerImage: %s, is different than actual: %s", tt.want.ContainerImage, mappedComponent.ContainerImage)

			if tt.want.GitSource != nil {
				assert.True(t, tt.want.GitSource.URL == mappedComponent.GitSource.URL, "Expected GitSource URL: %s, is different than actual: %s", tt.want.GitSource.URL, mappedComponent.GitSource.URL)
			}

			if !reflect.DeepEqual(tt.want.KubernetesResources, gitopsgenv1alpha1.KubernetesResources{}) {
				for _, wantDeployment := range tt.want.KubernetesResources.Deployments {
					matched := false
					for _, gotDeployment := range mappedComponent.KubernetesResources.Deployments {
						if wantDeployment.Name == gotDeployment.Name {
							matched = true
							break
						}
					}
					assert.True(t, matched, "Expected Deployment: %s, but didnt find in actual", wantDeployment.Name)
				}

				for _, wantService := range tt.want.KubernetesResources.Services {
					matched := false
					for _, gotService := range mappedComponent.KubernetesResources.Services {
						if wantService.Name == gotService.Name {
							matched = true
							break
						}
					}
					assert.True(t, matched, "Expected Service: %s, but didnt find in actual", wantService.Name)
				}

				for _, wantRoute := range tt.want.KubernetesResources.Routes {
					matched := false
					for _, gotRoute := range mappedComponent.KubernetesResources.Routes {
						if wantRoute.Name == gotRoute.Name {
							matched = true
							break
						}
					}
					assert.True(t, matched, "Expected Route: %s, but didnt find in actual", wantRoute.Name)
				}

				for _, wantIngress := range tt.want.KubernetesResources.Ingresses {
					matched := false
					for _, gotIngress := range mappedComponent.KubernetesResources.Ingresses {
						if wantIngress.Name == gotIngress.Name {
							matched = true
							break
						}
					}
					assert.True(t, matched, "Expected Ingress: %s, but didnt find in actual", wantIngress.Name)
				}

				for _, wantOther := range tt.want.KubernetesResources.Others {
					matched := false
					wantDeployment := wantOther.(appsv1.Deployment)

					for _, gotOther := range mappedComponent.KubernetesResources.Others {
						gotDeployment := gotOther.(appsv1.Deployment)
						if wantDeployment.Name == gotDeployment.Name {
							matched = true
							break
						}
					}
					assert.True(t, matched, "Expected Other: %s, but didnt find in actual", wantDeployment.Name)
				}
			}
		})
	}
}

func TestGetIntValue(t *testing.T) {

	value := 7

	tests := []struct {
		name      string
		replica   *int
		wantValue int
		wantErr   bool
	}{
		{
			name:      "Unset value, expect default 0",
			replica:   nil,
			wantValue: 0,
		},
		{
			name:      "set value, expect set number",
			replica:   &value,
			wantValue: 7,
		},
	}

	for _, tt := range tests {
		val := GetIntValue(tt.replica)
		assert.True(t, val == tt.wantValue, "Expected int value %d got %d", tt.wantValue, val)
	}
}

func TestStrInList(t *testing.T) {
	tests := []struct {
		name string
		str  string
		list []string
		want bool
	}{
		{
			name: "str not in list",
			str:  "test",
			list: []string{"some", "words"},
			want: false,
		},
		{
			name: "str in list",
			str:  "test",
			list: []string{"some", "test", "words"},
			want: true,
		},
	}

	for _, tt := range tests {
		val := StrInList(tt.str, tt.list)
		assert.True(t, val == tt.want, "Expected bool value %v got %v", tt.want, val)
	}
}

func TestRemoveStrFromList(t *testing.T) {
	tests := []struct {
		name string
		str  string
		list []string
		want []string
	}{
		{
			name: "single string in list",
			str:  "test",
			list: []string{"some", "test", "words"},
			want: []string{"some", "words"},
		},
		{
			name: "string not in list",
			str:  "test",
			list: []string{"some", "words"},
			want: []string{"some", "words"},
		},
		{
			name: "multiple occurence of string in list",
			str:  "test",
			list: []string{"some", "test", "words", "test", "again"},
			want: []string{"some", "words", "test", "again"},
		},
	}

	for _, tt := range tests {
		strList := RemoveStrFromList(tt.str, tt.list)
		if len(strList) != len(tt.want) {
			t.Errorf("TestRemoveStrFromList(): unexpected error. expected string list %v, got %v", tt.want, strList)
		}
		for i := range strList {
			if strList[i] != tt.want[i] {
				t.Errorf("TestRemoveStrFromList(): unexpected error. expected string %v at index %v, got %v", tt.want[i], i, strList[i])
			}
		}
	}
}

func TestGenerateRandomRouteName(t *testing.T) {

	tests := []struct {
		name          string
		componentName string
	}{
		{
			name:          "Simple component name, less than 25 characters",
			componentName: "test-comp",
		},
		{
			name:          "long component name",
			componentName: "test-test-test-test-test-test-test-test-test",
		},
	}

	for _, tt := range tests {
		routeName := GenerateRandomRouteName(tt.componentName)
		if len(routeName) >= 30 {
			t.Errorf("TestGenerateRandomRouteName() error: expected generated route name %s to be less than 30 chars", routeName)
		}
		if tt.name == "long component name" {
			if !strings.Contains(routeName, tt.componentName[0:25]) {
				t.Errorf("TestGenerateRandomRouteName() error: expected generated route name %s to contain first 25 chars of component name %s", routeName, tt.componentName)
			}
			if routeName == tt.componentName[0:25] {
				t.Errorf("TestGenerateRandomRouteName() error: expected generated route name %s to contain 25 char slice from component name %s", routeName, tt.componentName)
			}
		} else {
			if !strings.Contains(routeName, tt.componentName) {
				t.Errorf("TestGenerateRandomRouteName() error: expected generated route name %s to contain component name %s", routeName, tt.componentName)
			}
			if routeName == tt.componentName {
				t.Errorf("TestGenerateRandomRouteName() error: expected generated route name %s to be unique from component name %s", routeName, tt.componentName)
			}
		}

	}
}

func TestVerifyNoApplicationComponentUnderDeletion(t *testing.T) {
	testTime := time.Time{}.AddDate(2030, 12, 12)
	tests := []struct {
		name            string
		components      appstudiov1alpha1.ComponentList
		applicationName string
		wantErr         bool
	}{
		{
			name: "One component under deletion",
			components: appstudiov1alpha1.ComponentList{
				Items: []appstudiov1alpha1.Component{
					{
						Spec:       appstudiov1alpha1.ComponentSpec{Application: "application-one"},
						ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &metav1.Time{testTime}},
					},
				},
			},
			applicationName: "application-one",
			wantErr:         true,
		},
		{
			name: "No component under deletion",
			components: appstudiov1alpha1.ComponentList{
				Items: []appstudiov1alpha1.Component{
					{
						Spec: appstudiov1alpha1.ComponentSpec{Application: "application-one"},
					},
				},
			},
			applicationName: "application-one",
			wantErr:         false,
		},
		{
			name: "No component found for application",
			components: appstudiov1alpha1.ComponentList{
				Items: []appstudiov1alpha1.Component{
					{
						Spec: appstudiov1alpha1.ComponentSpec{Application: "application-one"},
					},
				},
			},
			applicationName: "application-two",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		err := VerifyNoApplicationComponentUnderDeletion(tt.components, tt.applicationName)
		if tt.wantErr && (err == nil) {
			t.Error("TestVerifyNoApplicationComponentUnderDeletion() error: expected error but got nil")
		} else if !tt.wantErr && err != nil {
			t.Errorf("TestVerifyNoApplicationComponentUnderDeletion() error: got unexpected error %v", err)
		}
	}
}
