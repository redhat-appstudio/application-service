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
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	appstudiov1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"
	gitopsprepare "github.com/konflux-ci/application-service/gitops/prepare"
	"github.com/konflux-ci/application-service/pkg/util/ioutils"
	"github.com/mitchellh/go-homedir"
	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/redhat-developer/gitops-generator/pkg/testutils"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateBuild(t *testing.T) {
	outoutFolder := "output"
	emptyGitopsConfig := gitopsprepare.GitopsConfig{}

	tests := []struct {
		name         string
		fs           afero.Afero
		component    appstudiov1alpha1.Component
		gitopsConfig gitopsprepare.GitopsConfig
		want         []string
	}{
		{
			name: "Check pipeline as code resources with annotation",
			fs:   ioutils.NewMemoryFilesystem(),
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "workspace-name",
					Annotations: map[string]string{
						PaCAnnotation: "1",
					},
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "https://github.com/user/git-repo.git",
							},
						},
					},
				},
			},
			gitopsConfig: emptyGitopsConfig,
			want: []string{
				kustomizeFileName,
				buildRepositoryFileName,
			},
		},
		{
			name: "Check pipeline as code resources on HACBS",
			fs:   ioutils.NewMemoryFilesystem(),
			component: appstudiov1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcomponent",
					Namespace: "workspace-name",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "https://github.com/user/git-repo.git",
							},
						},
					},
				},
			},
			gitopsConfig: emptyGitopsConfig,
			want: []string{
				kustomizeFileName,
				buildRepositoryFileName,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := GenerateBuild(tt.fs, outoutFolder, tt.component, tt.gitopsConfig); err != nil {
				t.Errorf("Failed to generate builf gitops resources. Cause: %v", err)
			}

			// Ensure that needed resources generated
			path, err := homedir.Expand(outoutFolder)
			testutils.AssertNoError(t, err)

			for _, item := range tt.want {
				exist, err := tt.fs.Exists(filepath.Join(path, item))
				testutils.AssertNoError(t, err)
				assert.True(t, exist, "Expected file %s missing in gitops", item)
			}
		})
	}
}

func TestGeneratePACRepository(t *testing.T) {
	getComponent := func(repoUrl string, annotations map[string]string) appstudiov1alpha1.Component {
		return appstudiov1alpha1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "testcomponent",
				Namespace:   "workspace-name",
				Annotations: annotations,
			},
			Spec: appstudiov1alpha1.ComponentSpec{
				Source: appstudiov1alpha1.ComponentSource{
					ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
						GitSource: &appstudiov1alpha1.GitSource{
							URL: repoUrl,
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name                      string
		repoUrl                   string
		componentAnnotations      map[string]string
		pacConfig                 map[string][]byte
		expectedGitProviderConfig *pacv1alpha1.GitProvider
	}{
		{
			name:    "should create PaC repository for Github application",
			repoUrl: "https://github.com/user/test-component-repository",
			pacConfig: map[string][]byte{
				PipelinesAsCode_githubAppIdKey:   []byte("12345"),
				PipelinesAsCode_githubPrivateKey: []byte("private-key"),
			},
			expectedGitProviderConfig: nil,
		},
		{
			name:    "should create PaC repository for Github application even if Github webhook configured",
			repoUrl: "https://github.com/user/test-component-repository",
			pacConfig: map[string][]byte{
				PipelinesAsCode_githubAppIdKey:   []byte("12345"),
				PipelinesAsCode_githubPrivateKey: []byte("private-key"),
				"github.token":                   []byte("ghp_token"),
			},
			expectedGitProviderConfig: nil,
		},
		{
			name:    "should create PaC repository for Github webhook",
			repoUrl: "https://github.com/user/test-component-repository",
			pacConfig: map[string][]byte{
				"github.token": []byte("ghp_token"),
				"gitlab.token": []byte("glpat-token"),
			},
			expectedGitProviderConfig: &pacv1alpha1.GitProvider{
				Secret: &pacv1alpha1.Secret{
					Name: gitopsprepare.PipelinesAsCodeSecretName,
					Key:  "github.token",
				},
				WebhookSecret: &pacv1alpha1.Secret{
					Name: PipelinesAsCodeWebhooksSecretName,
					Key:  GetWebhookSecretKeyForComponent(getComponent("https://github.com/user/test-component-repository", nil)),
				},
			},
		},
		{
			name:    "should create PaC repository for GitLab webhook",
			repoUrl: "https://gitlab.com/user/test-component-repository/",
			pacConfig: map[string][]byte{
				"github.token": []byte("ghp_token"),
				"gitlab.token": []byte("glpat-token"),
			},
			expectedGitProviderConfig: &pacv1alpha1.GitProvider{
				Secret: &pacv1alpha1.Secret{
					Name: gitopsprepare.PipelinesAsCodeSecretName,
					Key:  "gitlab.token",
				},
				WebhookSecret: &pacv1alpha1.Secret{
					Name: PipelinesAsCodeWebhooksSecretName,
					Key:  GetWebhookSecretKeyForComponent(getComponent("https://gitlab.com/user/test-component-repository/", nil)),
				},
				URL: "https://gitlab.com",
			},
		},
		{
			name:    "should create PaC repository for GitLab webhook even if GitHub application configured",
			repoUrl: "https://gitlab.com/user/test-component-repository.git",
			pacConfig: map[string][]byte{
				PipelinesAsCode_githubAppIdKey:   []byte("12345"),
				PipelinesAsCode_githubPrivateKey: []byte("private-key"),
				"gitlab.token":                   []byte("glpat-token"),
			},
			expectedGitProviderConfig: &pacv1alpha1.GitProvider{
				Secret: &pacv1alpha1.Secret{
					Name: gitopsprepare.PipelinesAsCodeSecretName,
					Key:  "gitlab.token",
				},
				WebhookSecret: &pacv1alpha1.Secret{
					Name: PipelinesAsCodeWebhooksSecretName,
					Key:  GetWebhookSecretKeyForComponent(getComponent("https://gitlab.com/user/test-component-repository", nil)),
				},
				URL: "https://gitlab.com",
			},
		},
		{
			name:    "should create PaC repository for self-hosted GitLab webhook",
			repoUrl: "https://gitlab.self-hosted.com/user/test-component-repository/",
			componentAnnotations: map[string]string{
				GitProviderAnnotationName: "gitlab",
				GitProviderAnnotationURL:  "https://gitlab.self-hosted.com",
			},
			pacConfig: map[string][]byte{
				"github.token": []byte("ghp_token"),
				"gitlab.token": []byte("glpat-token"),
			},
			expectedGitProviderConfig: &pacv1alpha1.GitProvider{
				Secret: &pacv1alpha1.Secret{
					Name: gitopsprepare.PipelinesAsCodeSecretName,
					Key:  "gitlab.token",
				},
				WebhookSecret: &pacv1alpha1.Secret{
					Name: PipelinesAsCodeWebhooksSecretName,
					Key:  GetWebhookSecretKeyForComponent(getComponent("https://gitlab.self-hosted.com/user/test-component-repository/", nil)),
				},
				URL: "https://gitlab.self-hosted.com",
			},
		},
		{
			name:    "should create PaC repository for Github application on self-hosted Github",
			repoUrl: "https://github.self-hosted.com/user/test-component-repository",
			componentAnnotations: map[string]string{
				GitProviderAnnotationName: "github",
				GitProviderAnnotationURL:  "https://github.self-hosted.com",
			},
			pacConfig: map[string][]byte{
				PipelinesAsCode_githubAppIdKey:   []byte("12345"),
				PipelinesAsCode_githubPrivateKey: []byte("private-key"),
			},
			expectedGitProviderConfig: &pacv1alpha1.GitProvider{
				URL: "https://github.self-hosted.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			component := getComponent(tt.repoUrl, tt.componentAnnotations)

			pacRepo, err := GeneratePACRepository(component, tt.pacConfig)

			if err != nil {
				t.Errorf("Failed to generate PaC repository object. Cause: %v", err)
			}

			if pacRepo.Name != component.Name {
				t.Errorf("Generated PaC repository must have the same name as corresponding component")
			}
			if pacRepo.Namespace != component.Namespace {
				t.Errorf("Generated PaC repository must have the same namespace as corresponding component")
			}
			if len(pacRepo.Annotations) == 0 {
				t.Errorf("Generated PaC repository must have annotations")
			}
			if pacRepo.Annotations["appstudio.openshift.io/component"] != component.Name {
				t.Errorf("Generated PaC repository must have component annotation")
			}
			expectedRepo := strings.TrimSuffix(strings.TrimSuffix(tt.repoUrl, ".git"), "/")
			if pacRepo.Spec.URL != expectedRepo {
				t.Errorf("Wrong git repository URL in PaC repository: %s, want %s", pacRepo.Spec.URL, expectedRepo)
			}
			if !reflect.DeepEqual(pacRepo.Spec.GitProvider, tt.expectedGitProviderConfig) {
				t.Errorf("Wrong git provider config in PaC repository: %#v, want %#v", pacRepo.Spec.GitProvider, tt.expectedGitProviderConfig)
			}
		})
	}
}

func TestGetProviderTokenKey(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     string
	}{
		{
			name:     "check github key",
			provider: "github",
			want:     "github.token",
		},
		{
			name:     "check gitlab key",
			provider: "gitlab",
			want:     "gitlab.token",
		},
		{
			name:     "check bitbucket key",
			provider: "bitbucket",
			want:     "bitbucket.token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetProviderTokenKey(tt.provider); got != tt.want {
				t.Errorf("Wrong git provider access token key: %s, want %s", got, tt.want)
			}
		})
	}
}

func TestGetWebhookSecretKeyForComponent(t *testing.T) {
	getComponent := func(repoUrl string) appstudiov1alpha1.Component {
		return appstudiov1alpha1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testcomponent",
				Namespace: "workspace-name",
			},
			Spec: appstudiov1alpha1.ComponentSpec{
				Source: appstudiov1alpha1.ComponentSource{
					ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
						GitSource: &appstudiov1alpha1.GitSource{
							URL: repoUrl,
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name      string
		component appstudiov1alpha1.Component
		want      string
	}{
		{
			name:      "should return key for the url",
			component: getComponent("https://github.com/user/test-component-repository"),
			want:      "https___github.com_user_test-component-repository",
		},
		{
			name:      "should ignore .git suffix",
			component: getComponent("https://github.com/user/test-component-repository.git"),
			want:      "https___github.com_user_test-component-repository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetWebhookSecretKeyForComponent(tt.component); got != tt.want {
				t.Errorf("Expected '%s', but got '%s'", tt.want, got)
			}
		})
	}
}

func TestGetGitProvider(t *testing.T) {
	getComponent := func(repoUrl, annotationValue string) appstudiov1alpha1.Component {
		componentMeta := metav1.ObjectMeta{
			Name:      "testcomponent",
			Namespace: "workspace-name",
		}
		if annotationValue != "" {
			componentMeta.Annotations = map[string]string{
				GitProviderAnnotationName: annotationValue,
			}
		}

		component := appstudiov1alpha1.Component{
			ObjectMeta: componentMeta,
			Spec: appstudiov1alpha1.ComponentSpec{
				Source: appstudiov1alpha1.ComponentSource{
					ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
						GitSource: &appstudiov1alpha1.GitSource{
							URL: repoUrl,
						},
					},
				},
			},
		}
		return component
	}

	tests := []struct {
		name                           string
		componentRepoUrl               string
		componentGitProviderAnnotation string
		want                           string
		expectError                    bool
	}{
		{
			name:             "should detect github provider via http url",
			componentRepoUrl: "https://github.com/user/test-component-repository",
			want:             "github",
		},
		{
			name:             "should detect github provider via git url",
			componentRepoUrl: "git@github.com:user/test-component-repository",
			want:             "github",
		},
		{
			name:             "should detect gitlab provider via http url",
			componentRepoUrl: "https://gitlab.com/user/test-component-repository",
			want:             "gitlab",
		},
		{
			name:             "should detect gitlab provider via git url",
			componentRepoUrl: "git@gitlab.com:user/test-component-repository",
			want:             "gitlab",
		},
		{
			name:             "should detect bitbucket provider via http url",
			componentRepoUrl: "https://bitbucket.org/user/test-component-repository",
			want:             "bitbucket",
		},
		{
			name:             "should detect bitbucket provider via git url",
			componentRepoUrl: "git@bitbucket.org:user/test-component-repository",
			want:             "bitbucket",
		},
		{
			name:                           "should detect github provider via annotation",
			componentRepoUrl:               "https://mydomain.com/user/test-component-repository",
			componentGitProviderAnnotation: "github",
			want:                           "github",
		},
		{
			name:                           "should detect gitlab provider via annotation",
			componentRepoUrl:               "https://mydomain.com/user/test-component-repository",
			componentGitProviderAnnotation: "gitlab",
			want:                           "gitlab",
		},
		{
			name:                           "should detect bitbucket provider via annotation",
			componentRepoUrl:               "https://mydomain.com/user/test-component-repository",
			componentGitProviderAnnotation: "bitbucket",
			want:                           "bitbucket",
		},
		{
			name:             "should fail to detect git provider for self-hosted instance if annotation is not set",
			componentRepoUrl: "https://mydomain.com/user/test-component-repository",
			expectError:      true,
		},
		{
			name:                           "should fail to detect git provider for self-hosted instance if annotation is set to invalid value",
			componentRepoUrl:               "https://mydomain.com/user/test-component-repository",
			componentGitProviderAnnotation: "mylab",
			expectError:                    true,
		},
		{
			name:             "should fail to detect git provider component repository URL is invalid",
			componentRepoUrl: "12345",
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			component := getComponent(tt.componentRepoUrl, tt.componentGitProviderAnnotation)
			got, err := GetGitProvider(component)
			if tt.expectError {
				if err == nil {
					t.Errorf("Detecting git provider for component with '%s' url and '%s' annotation value should fail", tt.componentRepoUrl, tt.componentGitProviderAnnotation)
				}
			} else {
				if got != tt.want {
					t.Errorf("Expected git provider is: %s, but got %s", tt.want, got)
				}
			}
		})
	}
}

func TestIsPaCApplicationConfigured(t *testing.T) {
	tests := []struct {
		name        string
		gitProvider string
		config      map[string][]byte
		want        bool
	}{
		{
			name:        "should detect github application configured",
			gitProvider: "github",
			config: map[string][]byte{
				PipelinesAsCode_githubAppIdKey:   []byte("12345"),
				PipelinesAsCode_githubPrivateKey: []byte("private-key"),
			},
			want: true,
		},
		{
			name:        "should prefer github application if both github application and webhook configured",
			gitProvider: "github",
			config: map[string][]byte{
				PipelinesAsCode_githubAppIdKey:   []byte("12345"),
				PipelinesAsCode_githubPrivateKey: []byte("private-key"),
				"github.token":                   []byte("ghp_token"),
			},
			want: true,
		},
		{
			name:        "should not detect application if it is not configured",
			gitProvider: "github",
			config: map[string][]byte{
				"github.token": []byte("ghp_token"),
			},
			want: false,
		},
		{
			name:        "should not detect application if configuration empty",
			gitProvider: "github",
			config:      map[string][]byte{},
			want:        false,
		},
		{
			name:        "should not detect GitHub application if gilab webhook configured",
			gitProvider: "gitlab",
			config: map[string][]byte{
				PipelinesAsCode_githubAppIdKey:   []byte("12345"),
				PipelinesAsCode_githubPrivateKey: []byte("private-key"),
				"gitlab.token":                   []byte("glpat-token"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPaCApplicationConfigured(tt.gitProvider, tt.config); got != tt.want {
				t.Errorf("want %t, but got %t", tt.want, got)
			}
		})
	}
}
