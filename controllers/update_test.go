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

	devfileAPIV1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/api/v2/pkg/devfile"
	v2 "github.com/devfile/library/pkg/devfile/parser/data/v2"
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"
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
		{
			name: "Git source in Component is nil",
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
							GitSource: nil,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name:     "Devfile Projects list is nil",
			projects: nil,
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "new",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: nil,
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devfileData := &v2.DevfileV2{
				Devfile: devfileAPIV1.Devfile{
					DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
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
			name: "Component with envFrom component - should error out as it's not supported right now",
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
					ComponentName: "component1",
					Env: []corev1.EnvVar{
						{
							Name:  "FOO",
							Value: "foo",
						},
						{
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									Key: "test",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Component with invalid component type - should error out",
			components: []devfileAPIV1.Component{
				{
					Name:           "component1",
					ComponentUnion: devfileAPIV1.ComponentUnion{},
				},
			},
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
					Env: []corev1.EnvVar{
						{
							Name:  "FOO",
							Value: "foo",
						},
						{
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									Key: "test",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devfileData := &v2.DevfileV2{
				Devfile: devfileAPIV1.Devfile{
					DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
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

func TestUpdateComponentStub(t *testing.T) {

	componentsValid := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
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
					Container: devfileAPIV1.Container{
						Image: "image",
						Env: []devfileAPIV1.EnvVar{
							{
								Name:  "name1",
								Value: "value1",
							},
						},
						CpuLimit:      "2",
						CpuRequest:    "700m",
						MemoryLimit:   "500Mi",
						MemoryRequest: "400Mi",
					},
				},
			},
			Attributes: attributes.Attributes{}.PutInteger(replicaKey, 1).PutString(routeKey, "route1").PutString(storageLimitKey, "400Mi").PutString(ephemeralStorageLimitKey, "400Mi").PutString(storageRequestKey, "200Mi").PutString(ephemeralStorageRequestKey, "200Mi"),
		},
		{
			Name: "component2",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Endpoints: []devfileAPIV1.Endpoint{
						{
							Name:       "endpoint22",
							TargetPort: 1003,
						},
					},
					Container: devfileAPIV1.Container{
						Image: "image2",
					},
				},
			},
		},
	}

	componentsReplicaErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image: "image",
					},
				},
			},
			Attributes: attributes.Attributes{}.PutBoolean(replicaKey, true),
		},
	}

	var err error

	componentsRouteErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image: "image",
					},
				},
			},
			Attributes: attributes.Attributes{}.Put(routeKey, []string{"a", "b"}, &err),
		},
	}
	if err != nil {
		t.Errorf("unexpected err: %+v", err)
		return
	}

	componentsStorageLimitErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image: "image",
					},
				},
			},
			Attributes: attributes.Attributes{}.Put(storageLimitKey, []string{"a", "b"}, &err),
		},
	}
	if err != nil {
		t.Errorf("unexpected err: %+v", err)
		return
	}

	componentsEphemeralStorageLimitErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image: "image",
					},
				},
			},
			Attributes: attributes.Attributes{}.Put(ephemeralStorageLimitKey, []string{"a", "b"}, &err),
		},
	}
	if err != nil {
		t.Errorf("unexpected err: %+v", err)
		return
	}

	componentsStorageRequestErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image: "image",
					},
				},
			},
			Attributes: attributes.Attributes{}.Put(storageRequestKey, []string{"a", "b"}, &err),
		},
	}
	if err != nil {
		t.Errorf("unexpected err: %+v", err)
		return
	}

	componentsEphemeralStorageRequestErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image: "image",
					},
				},
			},
			Attributes: attributes.Attributes{}.Put(ephemeralStorageRequestKey, []string{"a", "b"}, &err),
		},
	}
	if err != nil {
		t.Errorf("unexpected err: %+v", err)
		return
	}

	componentsCPULimitParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image:    "image",
						CpuLimit: "xyz",
					},
				},
			},
		},
	}

	componentsMemoryLimitParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image:       "image",
						MemoryLimit: "xyz",
					},
				},
			},
		},
	}

	componentsStorageLimitParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image: "image",
					},
				},
			},
			Attributes: attributes.Attributes{}.PutString(storageLimitKey, "xyz"),
		},
	}

	componentsEphemeralStorageLimitParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image: "image",
					},
				},
			},
			Attributes: attributes.Attributes{}.PutString(ephemeralStorageLimitKey, "xyz"),
		},
	}

	componentsCPURequestParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image:      "image",
						CpuRequest: "xyz",
					},
				},
			},
		},
	}

	componentsMemoryRequestParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image:         "image",
						MemoryRequest: "xyz",
					},
				},
			},
		},
	}

	componentsStorageRequestParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image: "image",
					},
				},
			},
			Attributes: attributes.Attributes{}.PutString(storageRequestKey, "xyz"),
		},
	}

	componentsEphemeralStorageRequestParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Container: &devfileAPIV1.ContainerComponent{
					Container: devfileAPIV1.Container{
						Image: "image",
					},
				},
			},
			Attributes: attributes.Attributes{}.PutString(ephemeralStorageRequestKey, "xyz"),
		},
	}

	tests := []struct {
		name            string
		devfilesDataMap map[string]*v2.DevfileV2
		isNil           bool
		wantErr         bool
	}{
		{
			name: "Container Components present",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsValid,
							},
						},
					},
				},
			},
		},
		{
			name: "No Container Components present",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{},
						},
					},
				},
			},
		},
		{
			name: "Check err condition",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{},
						},
					},
				},
			},
			isNil:   true,
			wantErr: true,
		},
		{
			name: "Check err for replica as non integer",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsReplicaErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for route as non string",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsRouteErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for storage limit as non string",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsStorageLimitErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for ephemeral storage limit as non string",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsEphemeralStorageLimitErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for storage request as non string",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsStorageRequestErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for ephemeral storage request as non string",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsEphemeralStorageRequestErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for cpu limit parse err",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsCPULimitParseErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for memory limit parse err",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsMemoryLimitParseErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for storage limit parse err",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsStorageLimitParseErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for ephemeral storage limit parse err",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsEphemeralStorageLimitParseErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for cpu request parse err",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsCPURequestParseErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for memory request parse err",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsMemoryRequestParseErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for storage request parse err",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsStorageRequestParseErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for ephemeral storage request parse err",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.1.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsEphemeralStorageRequestParseErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			componentDetectionQuery := appstudiov1alpha1.ComponentDetectionQuery{
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL: "url",
					},
				},
			}
			devfilesMap := make(map[string][]byte)

			for context, devfileData := range tt.devfilesDataMap {
				yamlData, err := yaml.Marshal(devfileData)
				if err != nil {
					t.Errorf("unexpected error %v", err)
				}
				devfilesMap[context] = yamlData
			}

			ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{
				Development: true,
			})))
			r := ComponentDetectionQueryReconciler{
				Log: ctrl.Log.WithName("TestUpdateComponentStub"),
			}
			var err error
			if tt.isNil {
				err = r.updateComponentStub(nil, devfilesMap, nil)
			} else {
				err = r.updateComponentStub(&componentDetectionQuery, devfilesMap, nil)
			}

			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else if err == nil {
				if len(componentDetectionQuery.Status.ComponentDetected) != len(tt.devfilesDataMap) {
					t.Errorf("expected no of devfiles: %v, actual no of devfiles %v", len(tt.devfilesDataMap), len(componentDetectionQuery.Status.ComponentDetected))
				} else {
					for _, hasCompDetection := range componentDetectionQuery.Status.ComponentDetected {
						assert.Equal(t, hasCompDetection.Language, tt.devfilesDataMap[hasCompDetection.ComponentStub.Context].Metadata.Language, "The language should be the same")
						assert.Equal(t, hasCompDetection.ProjectType, tt.devfilesDataMap[hasCompDetection.ComponentStub.Context].Metadata.ProjectType, "The project type should be the same")
						assert.Equal(t, hasCompDetection.DevfileFound, true, "The devfile found should be true")
						assert.Equal(t, hasCompDetection.ComponentStub.ComponentName, tt.devfilesDataMap[hasCompDetection.ComponentStub.Context].Metadata.Name, "The component name should be the same")
						assert.Equal(t, hasCompDetection.ComponentStub.Application, "insert-application-name", "The application name should match the generic name")

						for _, devfileComponent := range tt.devfilesDataMap[hasCompDetection.ComponentStub.Context].Components {
							if devfileComponent.Container != nil {
								for _, devfileEnv := range devfileComponent.Container.Env {
									matched := false
									for _, compEnv := range hasCompDetection.ComponentStub.Env {
										if devfileEnv.Name == compEnv.Name && devfileEnv.Value == compEnv.Value {
											matched = true
										}
									}
									assert.True(t, matched, "env %s:%s should match", devfileEnv.Name, devfileEnv.Value)
								}

								for i, endpoint := range devfileComponent.Container.Endpoints {
									if i == 0 {
										assert.Equal(t, endpoint.TargetPort, hasCompDetection.ComponentStub.TargetPort, "target port should match")
									}
								}

								var err error
								limits := hasCompDetection.ComponentStub.Resources.Limits
								if len(limits) > 0 {
									resourceCPULimit := limits[corev1.ResourceCPU]
									assert.Equal(t, resourceCPULimit.String(), devfileComponent.Container.CpuLimit, "The cpu limit should be the same")

									resourceMemoryLimit := limits[corev1.ResourceMemory]
									assert.Equal(t, resourceMemoryLimit.String(), devfileComponent.Container.MemoryLimit, "The memory limit should be the same")

									resourceStorageLimit := limits[corev1.ResourceStorage]
									assert.Equal(t, resourceStorageLimit.String(), devfileComponent.Attributes.GetString(storageLimitKey, &err), "The storage limit should be the same")
									assert.Nil(t, err, "err should be nil")

									resourceEphemeralStorageLimit := limits[corev1.ResourceEphemeralStorage]
									assert.Equal(t, resourceEphemeralStorageLimit.String(), devfileComponent.Attributes.GetString(ephemeralStorageLimitKey, &err), "The ephemeral storage limit should be the same")
									assert.Nil(t, err, "err should be nil")
								}

								requests := hasCompDetection.ComponentStub.Resources.Requests
								if len(requests) > 0 {
									resourceCPURequest := requests[corev1.ResourceCPU]
									assert.Equal(t, resourceCPURequest.String(), devfileComponent.Container.CpuRequest, "The cpu request should be the same")

									resourceMemoryRequest := requests[corev1.ResourceMemory]
									assert.Equal(t, resourceMemoryRequest.String(), devfileComponent.Container.MemoryRequest, "The memory request should be the same")

									resourceStorageRequest := requests[corev1.ResourceStorage]
									assert.Equal(t, resourceStorageRequest.String(), devfileComponent.Attributes.GetString(storageRequestKey, &err), "The storage request should be the same")
									assert.Nil(t, err, "err should be nil")

									resourceEphemeralStorageRequest := requests[corev1.ResourceEphemeralStorage]
									assert.Equal(t, resourceEphemeralStorageRequest.String(), devfileComponent.Attributes.GetString(ephemeralStorageRequestKey, &err), "The ephemeral storage request should be the same")
									assert.Nil(t, err, "err should be nil")
								}

								assert.Equal(t, hasCompDetection.ComponentStub.Replicas, int(devfileComponent.Attributes.GetNumber(replicaKey, &err)), "The replicas should be the same")
								assert.Nil(t, err, "err should be nil")

								assert.Equal(t, hasCompDetection.ComponentStub.Route, devfileComponent.Attributes.GetString(routeKey, &err), "The route should be the same")
								assert.Nil(t, err, "err should be nil")

								break // dont check for the second container
							}
						}
					}
				}
			}
		})
	}
}
