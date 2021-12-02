//
// Copyright 2021 Red Hat, Inc.
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
	"testing"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	devfileAPIV1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	v2 "github.com/devfile/library/pkg/devfile/parser/data/v2"
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestUpdateApplicationDevfileModel(t *testing.T) {
	tests := []struct {
		name      string
		projects  []devfileAPIV1.Project
		component appstudiov1alpha1.Component
		wantErr   bool
	}{
		{
			name: "Project already present",
			projects: []devfileAPIV1.Project{
				{
					Name: "duplicate",
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "duplicate",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Project added successfully",
			projects: []devfileAPIV1.Project{
				{
					Name: "present",
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "new",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "url",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devfileData := &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevWorkspaceTemplateSpec: v1alpha2.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: v1alpha2.DevWorkspaceTemplateSpecContent{
							Projects: tt.projects,
						},
					},
				},
			}
			r := ComponentReconciler{}
			err := r.updateApplicationDevfileModel(devfileData, tt.component)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else if err == nil {
				projects, err := devfileData.GetProjects(common.DevfileOptions{})
				if err != nil {
					t.Errorf("got unexpected error: %v", err)
				}
				matched := false
				for _, project := range projects {
					projectGitSrc := project.ProjectSource.Git
					if project.Name == tt.component.Spec.ComponentName && projectGitSrc != nil && projectGitSrc.Remotes["origin"] == tt.component.Spec.Source.GitSource.URL {
						matched = true
					}
				}

				if !matched {
					t.Errorf("unable to find devfile with project: %s", tt.component.Spec.ComponentName)
				}
			}
		})
	}
}

func TestUpdateComponentDevfileModel(t *testing.T) {

	storage1GiResource, err := resource.ParseQuantity("1Gi")
	if err != nil {
		t.Error(err)
	}
	core500mResource, err := resource.ParseQuantity("500m")
	if err != nil {
		t.Error(err)
	}

	originalResources := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:              core500mResource,
			corev1.ResourceMemory:           storage1GiResource,
			corev1.ResourceStorage:          storage1GiResource,
			corev1.ResourceEphemeralStorage: storage1GiResource,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:              core500mResource,
			corev1.ResourceMemory:           storage1GiResource,
			corev1.ResourceStorage:          storage1GiResource,
			corev1.ResourceEphemeralStorage: storage1GiResource,
		},
	}

	env := []corev1.EnvVar{
		{
			Name:  "FOO",
			Value: "foo1",
		},
		{
			Name:  "BAR",
			Value: "bar1",
		},
	}

	tests := []struct {
		name           string
		components     []devfileAPIV1.Component
		component      appstudiov1alpha1.Component
		updateExpected bool
		wantErr        bool
	}{
		{
			name: "No container component",
			components: []devfileAPIV1.Component{
				{
					Name: "component1",
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Kubernetes: &devfileAPIV1.KubernetesComponent{},
					},
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "componentName",
				},
			},
		},
		{
			name: "one container component",
			components: []devfileAPIV1.Component{
				{
					Name: "component1",
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Container: &devfileAPIV1.ContainerComponent{
							Container: devfileAPIV1.Container{
								Env: []devfileAPIV1.EnvVar{
									{
										Name:  "FOO",
										Value: "foo",
									},
								},
							},
							Endpoints: []devfileAPIV1.Endpoint{
								{
									Name:       "endpoint1",
									TargetPort: 1001,
								},
								{
									Name:       "endpoint2",
									TargetPort: 1002,
								},
							},
						},
					},
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "componentName",
					Application:   "applicationName",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "url",
							},
						},
					},
					Route:      "route1",
					Replicas:   1,
					TargetPort: 1111,
					Env:        env,
					Resources:  originalResources,
				},
			},
			updateExpected: true,
		},
		{
			name: "two container components",
			components: []devfileAPIV1.Component{
				{
					Name: "component1",
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Container: &devfileAPIV1.ContainerComponent{
							Container: devfileAPIV1.Container{
								Env: []devfileAPIV1.EnvVar{
									{
										Name:  "FOO",
										Value: "foo",
									},
								},
							},
							Endpoints: []devfileAPIV1.Endpoint{
								{
									Name:       "endpoint1",
									TargetPort: 1001,
								},
								{
									Name:       "endpoint2",
									TargetPort: 1002,
								},
							},
						},
					},
				},
				{
					Name: "component2",
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Container: &devfileAPIV1.ContainerComponent{
							Container: devfileAPIV1.Container{
								Env: []devfileAPIV1.EnvVar{
									{
										Name:  "FOO",
										Value: "foo",
									},
								},
								MemoryLimit: "2Gi",
							},
							Endpoints: []devfileAPIV1.Endpoint{
								{
									Name:       "endpoint3",
									TargetPort: 3333,
								},
								{
									Name:       "endpoint4",
									TargetPort: 4444,
								},
							},
						},
					},
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "componentName",
					Application:   "applicationName",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "url",
							},
						},
					},
					Route:      "route1",
					Replicas:   1,
					TargetPort: 1111,
					Env:        env,
					Resources:  originalResources,
				},
			},
			updateExpected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devfileData := &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevWorkspaceTemplateSpec: v1alpha2.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: v1alpha2.DevWorkspaceTemplateSpecContent{
							Components: tt.components,
						},
					},
				},
			}

			ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{
				Development: true,
			})))
			r := ComponentReconciler{
				Log: ctrl.Log.WithName("TestUpdateComponentDevfileModel"),
			}
			err := r.updateComponentDevfileModel(devfileData, tt.component)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else if err == nil {
				if tt.updateExpected {
					// it has been updated
					checklist := updateChecklist{
						route:     tt.component.Spec.Route,
						replica:   tt.component.Spec.Replicas,
						port:      tt.component.Spec.TargetPort,
						env:       tt.component.Spec.Env,
						resources: tt.component.Spec.Resources,
					}

					verifyHASComponentUpdates(devfileData, checklist, t)
				}
			}
		})
	}
}
