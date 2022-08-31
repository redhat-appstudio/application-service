/*
Copyright 2021-2022 Red Hat, Inc.

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
	"errors"
	"reflect"
	"testing"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	data "github.com/devfile/library/pkg/devfile/parser/data"
	v2 "github.com/devfile/library/pkg/devfile/parser/data/v2"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/github"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	"github.com/redhat-developer/gitops-generator/pkg/testutils"
	"github.com/spf13/afero"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	devfileApi "github.com/devfile/api/v2/pkg/devfile"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//+kubebuilder:scaffold:imports
)

func TestSetGitOpsStatus(t *testing.T) {
	tests := []struct {
		name             string
		devfileData      *v2.DevfileV2
		component        appstudiov1alpha1.Component
		wantGitOpsStatus appstudiov1alpha1.GitOpsStatus
		wantErr          bool
	}{
		{
			name: "Simple application devfile, only gitops url",
			devfileData: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfileApi.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfileApi.DevfileMetadata{
							Name:       "petclinic",
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
						},
					},
				},
			},
			wantGitOpsStatus: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "https://github.com/testorg/petclinic-gitops",
			},
			wantErr: false,
		},
		{
			name: "Simple application devfile, no gitops fields",
			devfileData: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfileApi.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfileApi.DevfileMetadata{
							Name: "petclinic",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Application devfile, all gitops fields",
			devfileData: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfileApi.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfileApi.DevfileMetadata{
							Name:       "petclinic",
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("gitOpsRepository.branch", "main").PutString("gitOpsRepository.context", "/test"),
						},
					},
				},
			},
			wantGitOpsStatus: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "https://github.com/testorg/petclinic-gitops",
				Branch:        "main",
				Context:       "/test",
			},
			wantErr: false,
		},
		{
			name: "Application devfile, gitops branch with invalid value",
			devfileData: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfileApi.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfileApi.DevfileMetadata{
							Name:       "petclinic",
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").Put("gitOpsRepository.branch", appstudiov1alpha1.Component{}, nil),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Application devfile, gitops context with invalid value",
			devfileData: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfileApi.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfileApi.DevfileMetadata{
							Name:       "petclinic",
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").Put("gitOpsRepository.context", appstudiov1alpha1.Component{}, nil),
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setGitopsStatus(&tt.component, tt.devfileData)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestSetGitOpsAnnotations() unexpected error: %v", err)
			}
			if !tt.wantErr {
				compGitOps := tt.component.Status.GitOps
				if !reflect.DeepEqual(compGitOps, tt.wantGitOpsStatus) {
					t.Errorf("TestSetGitOpsAnnotations() error: expected %v got %v", tt.wantGitOpsStatus, compGitOps)
				}
			}
		})
	}

}

func TestGenerateGitops(t *testing.T) {
	executor := testutils.NewMockExecutor()
	appFS := ioutils.NewMemoryFilesystem()
	readOnlyFs := ioutils.NewReadOnlyFs()

	fakeClient := fake.NewClientBuilder().Build()

	r := &ComponentReconciler{
		Log:       ctrl.Log.WithName("controllers").WithName("Component"),
		GitHubOrg: github.AppStudioAppDataOrg,
		GitToken:  "fake-token",
		Executor:  executor,
		Client:    fakeClient,
	}

	// Create a second reconciler for testing error scenarios
	errExec := testutils.NewMockExecutor()
	errExec.Errors.Push(errors.New("Fatal error"))
	errReconciler := &ComponentReconciler{
		Log:       ctrl.Log.WithName("controllers").WithName("Component"),
		GitHubOrg: github.AppStudioAppDataOrg,
		GitToken:  "fake-token",
		Executor:  errExec,
		Client:    fakeClient,
	}

	componentSpec := appstudiov1alpha1.ComponentSpec{
		ComponentName: "test-component",
		Application:   "test-app",
		Source: appstudiov1alpha1.ComponentSource{
			ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
				GitSource: &appstudiov1alpha1.GitSource{
					URL: "git@github.com:testing/testing.git",
				},
			},
		},
	}

	tests := []struct {
		name       string
		reconciler *ComponentReconciler
		fs         afero.Afero
		component  *appstudiov1alpha1.Component
		wantErr    bool
	}{
		{
			name:       "Simple application component, no errors",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
				},
				Spec: componentSpec,
				Status: appstudiov1alpha1.ComponentStatus{
					GitOps: appstudiov1alpha1.GitOpsStatus{
						RepositoryURL: "https://github.com/test/repo",
						Branch:        "main",
						Context:       "/test",
					},
				},
			},
			wantErr: false,
		},
		{
			name:       "Invalid application component, no labels",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-component",
					Namespace:   "test-namespace",
					Annotations: nil,
				},
				Spec: componentSpec,
			},
			wantErr: true,
		},
		{
			name:       "Invalid application component, no gitops URL",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						"fake": "fake",
					},
				},
				Spec: componentSpec,
			},
			wantErr: true,
		},
		{
			name:       "Invalid application component, invalid gitops url",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						"gitOpsRepository.url": "dsfdsf sdfsdf sdk;;;fsd ppz mne@ddsfj#$*(%",
					},
				},
				Spec: componentSpec,
			},
			wantErr: true,
		},
		{
			name:       "Application component, only gitops URL set",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
				},
				Spec: componentSpec,
				Status: appstudiov1alpha1.ComponentStatus{
					GitOps: appstudiov1alpha1.GitOpsStatus{
						RepositoryURL: "https://github.com/test/repo",
					},
				},
			},
			wantErr: false,
		},
		{
			name:       "Gitops generarion fails",
			reconciler: errReconciler,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
				},
				Spec: componentSpec,
				Status: appstudiov1alpha1.ComponentStatus{
					GitOps: appstudiov1alpha1.GitOpsStatus{
						RepositoryURL: "https://github.com/test/repo",
					},
				},
			},
			wantErr: true,
		},
		{
			name:       "Fail to create temp folder",
			reconciler: errReconciler,
			fs:         readOnlyFs,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
				},
				Spec: componentSpec,
				Status: appstudiov1alpha1.ComponentStatus{
					GitOps: appstudiov1alpha1.GitOpsStatus{
						RepositoryURL: "https://github.com/test/repo",
					},
				},
			},
			wantErr: true,
		},
		{
			name:       "Fail to retrieve commit ID for GitOps repository [Mock]",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-git-error",
					Namespace: "test-namespace",
				},
				Spec: componentSpec,
				Status: appstudiov1alpha1.ComponentStatus{
					GitOps: appstudiov1alpha1.GitOpsStatus{
						RepositoryURL: "https://github.com/test/repo",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt.reconciler.AppFS = tt.fs
		t.Run(tt.name, func(t *testing.T) {
			err := tt.reconciler.generateGitops(ctx, ctrl.Request{}, tt.component)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestGenerateGitops() unexpected error: %v", err)
			}
		})
	}

}
