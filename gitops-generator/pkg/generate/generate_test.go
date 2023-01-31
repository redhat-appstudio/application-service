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

package generate

import (
	"context"
	"errors"
	"testing"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops-generator/pkg/gitops"
	"github.com/redhat-developer/gitops-generator/pkg/util/ioutils"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGenerateGitopsBase(t *testing.T) {
	appFS := ioutils.NewMemoryFilesystem()
	readOnlyFs := ioutils.NewReadOnlyFs()
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()

	errGen := gitops.NewMockGenerator()
	errGen.Errors.Push(errors.New("Fatal error"))

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
		name         string
		fs           afero.Afero
		component    *appstudiov1alpha1.Component
		gitopsParams GitOpsGenParams
		wantErr      bool
	}{
		{
			name: "Simple application component, no errors",
			fs:   appFS,
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
			gitopsParams: GitOpsGenParams{
				Generator: gitops.NewMockGenerator(),
			},
			wantErr: false,
		},
		{
			name: "Generation error, Read only file system",
			fs:   readOnlyFs,
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
			gitopsParams: GitOpsGenParams{
				Generator: gitops.NewMockGenerator(),
			},
			wantErr: true,
		},
		{
			name: "Generation error",
			fs:   appFS,
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
			gitopsParams: GitOpsGenParams{
				Generator: errGen,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GenerateGitopsBase(ctx, fakeClient, *tt.component, tt.fs, tt.gitopsParams)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestGenerateGitops() unexpected error: %v", err)
			}
		})
	}
}

func TestGenerateGitopsOverlays(t *testing.T) {
	appFS := ioutils.NewMemoryFilesystem()
	//readOnlyFs := ioutils.NewReadOnlyFs()
	ctx := context.Background()
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	appstudiov1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	errGen := gitops.NewMockGenerator()
	errGen.Errors.Push(errors.New("Fatal error"))

	// Before the test runs, make sure that Application, Component and associated resources all exist
	setUpResources(t, &fakeClient, ctx)
	newComponent := appstudiov1alpha1.Component{}
	err := fakeClient.Get(ctx, types.NamespacedName{Namespace: "test-namespace", Name: "test-component"}, &newComponent)
	if err != nil {
		t.Error(err)
	}

	snapshotEnvironmentBinding := appstudiov1alpha1.SnapshotEnvironmentBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "SnapshotEnvironmentBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seb",
			Namespace: "test-namespace",
		},
		Spec: appstudiov1alpha1.SnapshotEnvironmentBindingSpec{
			Application: "test-application",
			Environment: "test-environment",
			Snapshot:    "test-snapshot",
			Components: []appstudiov1alpha1.BindingComponent{
				{
					Name: "test-component",
					Configuration: appstudiov1alpha1.BindingComponentConfiguration{
						Env: []appstudiov1alpha1.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			},
		},
	}
	tests := []struct {
		name         string
		fs           afero.Afero
		seb          *appstudiov1alpha1.SnapshotEnvironmentBinding
		gitopsParams GitOpsGenParams
		wantErr      bool
	}{
		{
			name: "Gitops generation succeeds",
			fs:   appFS,
			seb:  &snapshotEnvironmentBinding,
			gitopsParams: GitOpsGenParams{
				Generator: gitops.NewMockGenerator(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GenerateGitopsOverlays(ctx, fakeClient, *tt.seb, tt.fs, tt.gitopsParams)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestGenerateGitops() unexpected error: %v", err)
			}
		})
	}
}

// setUpResources sets up the necessary Kubernetes resources for the TestGenerateGitopsOverlays test
// The following resources need to be created before the test can be run:
// Component, Environment, Snapshot, SnapshotEnvironmentBinding
func setUpResources(t *testing.T, client *client.WithWatch, ctx context.Context) {
	// Create the Component
	kubeClient := *client
	component := appstudiov1alpha1.Component{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-component",
			Namespace: "test-namespace",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName: "test-component",
			Application:   "test-application",
		},
		Status: appstudiov1alpha1.ComponentStatus{
			GitOps: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "https://github.com/testorg/repo",
				Branch:        "main",
				Context:       "/",
			},
		},
	}
	err := kubeClient.Create(ctx, &component)
	if err != nil {
		t.Error(err)
	}

	// Create the Environment
	environment := appstudiov1alpha1.Environment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Environment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-environment",
			Namespace: "test-namespace",
		},
		Spec: appstudiov1alpha1.EnvironmentSpec{
			Type:               appstudiov1alpha1.EnvironmentType_POC,
			DisplayName:        "Staging Environment",
			DeploymentStrategy: appstudiov1alpha1.DeploymentStrategy_AppStudioAutomated,
			Configuration: appstudiov1alpha1.EnvironmentConfiguration{
				Env: []appstudiov1alpha1.EnvVarPair{
					{
						Name:  "Test",
						Value: "Value",
					},
				},
			},
		},
	}
	err = kubeClient.Create(ctx, &environment)
	if err != nil {
		t.Error(err)
	}

	// Create the Snapshot
	snapshot := appstudiov1alpha1.Snapshot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Snapshot",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-snapshot",
			Namespace: "test-namespace",
		},
		Spec: appstudiov1alpha1.SnapshotSpec{
			Application:        "test-application",
			DisplayName:        "Test Snapshot",
			DisplayDescription: "My First Snapshot",
			Components: []appstudiov1alpha1.SnapshotComponent{
				{
					Name:           "test-component",
					ContainerImage: "quay.io/redhat-appstudio/user-workload:application-service-system-test-component",
				},
			},
		},
	}
	err = kubeClient.Create(ctx, &snapshot)
	if err != nil {
		t.Error(err)
	}
}
