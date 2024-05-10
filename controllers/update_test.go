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

package controllers

import (
	"context"
	"strings"
	"testing"

	devfileAPIV1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/api/v2/pkg/devfile"
	v2 "github.com/devfile/library/v2/pkg/devfile/parser/data/v2"
	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	"github.com/prometheus/client_golang/prometheus/testutil"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	devfilePkg "github.com/redhat-appstudio/application-service/pkg/devfile"
	"github.com/redhat-appstudio/application-service/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

var (
	oneReplica    = 1
	zeroReplica   = 0
	threeReplicas = 3
)

var k8sInlined = ` 
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        creationTimestamp: null
        labels:
          maysun: test
        name: deploy-sample
      spec:
        replicas: 3
        selector: {}
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              app.kubernetes.io/instance: component-sample
          spec:
            containers:
            - env:
              - name: FOO
                value: foo1
              - name: BARBAR
                value: bar1
              image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              imagePullPolicy: Always
              livenessProbe:
                httpGet:
                  path: /
                  port: 1111
                initialDelaySeconds: 10
                periodSeconds: 10
              name: container-image
              ports:
              - containerPort: 1111
              readinessProbe:
                initialDelaySeconds: 10
                periodSeconds: 10
                tcpSocket:
                  port: 1111
              resources:
                limits:
                  cpu: "2"
                  memory: 500Mi
                requests:
                  cpu: 700m
                  memory: 400Mi
      status: {}`

func TestUpdateApplicationDevfileModel(t *testing.T) {
	tests := []struct {
		name           string
		projects       []devfileAPIV1.Project
		attributes     attributes.Attributes
		containerImage string
		components     []appstudiov1alpha1.Component
		wantErr        bool
	}{
		{
			name: "Project already present",
			projects: []devfileAPIV1.Project{
				{
					Name: "duplicate",
				},
			},
			components: []appstudiov1alpha1.Component{
				{
					Spec: appstudiov1alpha1.ComponentSpec{
						ComponentName: "duplicate",
						Source: appstudiov1alpha1.ComponentSource{
							ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
								GitSource: &appstudiov1alpha1.GitSource{},
							},
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
			components: []appstudiov1alpha1.Component{
				{
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
		},
		{
			name: "Git source in Component is nil",
			projects: []devfileAPIV1.Project{
				{
					Name: "present",
				},
			},
			components: []appstudiov1alpha1.Component{
				{
					Spec: appstudiov1alpha1.ComponentSpec{
						ComponentName: "new",
						Source: appstudiov1alpha1.ComponentSource{
							ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
								GitSource: nil,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name:     "Devfile Projects list is nil",
			projects: nil,
			components: []appstudiov1alpha1.Component{
				{
					Spec: appstudiov1alpha1.ComponentSpec{
						ComponentName: "new",
						Source: appstudiov1alpha1.ComponentSource{
							ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
								GitSource: nil,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name:       "Container image added successfully",
			attributes: attributes.Attributes{}.PutString("containerImage/otherComponent", "other-image"),
			components: []appstudiov1alpha1.Component{
				{
					Spec: appstudiov1alpha1.ComponentSpec{
						ComponentName:  "new",
						ContainerImage: "an-image",
					},
				},
			},
		},
		{
			name:       "Container image already exists",
			attributes: attributes.Attributes{}.PutString("containerImage/new", "an-image"),
			components: []appstudiov1alpha1.Component{
				{
					Spec: appstudiov1alpha1.ComponentSpec{
						ComponentName:  "new",
						ContainerImage: "an-image",
					},
				},
			},
			wantErr: true,
		},
		{
			name:       "Container image already exists, but invalid entry",
			attributes: attributes.Attributes{}.Put("containerImage/new", make(chan error), nil),
			components: []appstudiov1alpha1.Component{
				{
					Spec: appstudiov1alpha1.ComponentSpec{
						ComponentName:  "new",
						ContainerImage: "an-image",
					},
				},
			},
			wantErr: true,
		},
		{
			name:     "Multiple Projects added successfully",
			projects: nil,
			components: []appstudiov1alpha1.Component{
				{
					Spec: appstudiov1alpha1.ComponentSpec{
						ComponentName: "compname",
						Source: appstudiov1alpha1.ComponentSource{
							ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
								GitSource: &appstudiov1alpha1.GitSource{
									URL: "url",
								},
							},
						},
					},
				},
				{
					Spec: appstudiov1alpha1.ComponentSpec{
						ComponentName: "compnametwo",
						Source: appstudiov1alpha1.ComponentSource{
							ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
								GitSource: &appstudiov1alpha1.GitSource{
									URL: "urltwo",
								},
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
				Devfile: devfileAPIV1.Devfile{
					DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
						DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
							Attributes: tt.attributes,
							Projects:   tt.projects,
						},
					},
				},
			}
			r := ApplicationReconciler{}
			err := r.addComponentsToApplicationDevfileModel(&devfileData.DevWorkspaceTemplateSpec, tt.components)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else if err == nil {
				for _, component := range tt.components {
					if component.Spec.Source.GitSource != nil {
						projects, err := devfileData.GetProjects(common.DevfileOptions{})
						if err != nil {
							t.Errorf("got unexpected error: %v", err)
						}
						matched := false
						for _, project := range projects {
							projectGitSrc := project.ProjectSource.Git
							if project.Name == component.Spec.ComponentName && projectGitSrc != nil && projectGitSrc.Remotes["origin"] == component.Spec.Source.GitSource.URL {
								matched = true
							}
						}

						if !matched {
							t.Errorf("unable to find devfile with project: %s", component.Spec.ComponentName)
						}

					} else {
						devfileAttr, err := devfileData.GetAttributes()
						if err != nil {
							t.Errorf("got unexpected error: %v", err)
						}
						if devfileAttr == nil {
							t.Errorf("devfile attributes should not be nil")
						}
						containerImage := devfileAttr.GetString("containerImage/new", &err)
						if err != nil {
							t.Errorf("got unexpected error: %v", err)
						}
						if containerImage != component.Spec.ContainerImage {
							t.Errorf("unable to find component with container iamge: %s", component.Spec.ContainerImage)
						}
					}
				}

			}
		})
	}
}

func TestGetAndAddComponentApplicationsToModel(t *testing.T) {
	applicationName := "my-app"
	namespace := "default"
	component1 := "component-one"
	component2 := "component-two"

	tests := []struct {
		name            string
		projects        []devfileAPIV1.Project
		containerImage  string
		attributes      attributes.Attributes
		applicationName string
		componentOne    *appstudiov1alpha1.Component
		componentTwo    *appstudiov1alpha1.Component
		wantErr         bool
	}{
		{
			name: "Project already present",
			projects: []devfileAPIV1.Project{
				{
					Name: "duplicate",
				},
			},
			applicationName: applicationName,
			componentOne: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Component",
					APIVersion: "appstudio.redhat.com",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      component1,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Application:   applicationName,
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
			applicationName: applicationName,
			componentOne: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Component",
					APIVersion: "appstudio.redhat.com",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      component1,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Application:   applicationName,
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
			applicationName: applicationName,
			componentOne: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Component",
					APIVersion: "appstudio.redhat.com",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      component1,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Application:   applicationName,
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
			name:            "Devfile Projects list is nil",
			projects:        nil,
			applicationName: applicationName,
			componentOne: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Component",
					APIVersion: "appstudio.redhat.com",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      component1,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Application:   applicationName,
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
			name:            "Container image added successfully",
			attributes:      attributes.Attributes{}.PutString("containerImage/otherComponent", "other-image"),
			applicationName: applicationName,
			componentOne: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Component",
					APIVersion: "appstudio.redhat.com",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      component1,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Application:    applicationName,
					ComponentName:  "new",
					ContainerImage: "an-image",
				},
			},
		},
		{
			name:            "Container image already exists",
			attributes:      attributes.Attributes{}.PutString("containerImage/new", "an-image"),
			applicationName: applicationName,
			componentOne: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Component",
					APIVersion: "appstudio.redhat.com",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      component1,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Application:    applicationName,
					ComponentName:  "new",
					ContainerImage: "an-image",
				},
			},
			wantErr: true,
		},
		{
			name:            "Container image already exists, but invalid entry",
			attributes:      attributes.Attributes{}.Put("containerImage/new", make(chan error), nil),
			applicationName: applicationName,
			componentOne: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Component",
					APIVersion: "appstudio.redhat.com",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      component1,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Application:    applicationName,
					ComponentName:  "new",
					ContainerImage: "an-image",
				},
			},
			wantErr: true,
		},
		{
			name:            "Multiple Projects added successfully",
			projects:        nil,
			applicationName: applicationName,
			componentOne: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Component",
					APIVersion: "appstudio.redhat.com",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      component1,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Application:   applicationName,
					ComponentName: "compname",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "url",
							},
						},
					},
				},
			},

			componentTwo: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Component",
					APIVersion: "appstudio.redhat.com",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      component2,
					Namespace: namespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					Application:   applicationName,
					ComponentName: "compnametwo",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "urltwo",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fakeClient *FakeClient
			if tt.componentOne != nil && tt.componentTwo != nil {
				fakeClient = NewFakeClient(t, tt.componentOne, tt.componentTwo)
			} else if tt.componentOne != nil {
				fakeClient = NewFakeClient(t, tt.componentOne)
			} else if tt.componentTwo != nil {
				fakeClient = NewFakeClient(t, tt.componentTwo)
			}

			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.ApplicationCreationSucceeded)
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.ApplicationCreationFailed)

			devSpec := devfileAPIV1.DevWorkspaceTemplateSpec{
				DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
					Attributes: tt.attributes,
					Projects:   tt.projects,
				},
			}

			log := zap.New(zap.UseFlagOptions(&zap.Options{
				Development: true,
				TimeEncoder: zapcore.ISO8601TimeEncoder,
			}))
			r := ApplicationReconciler{Client: fakeClient}
			req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: tt.applicationName}}
			err := r.getAndAddComponentApplicationsToModel(log, req, tt.applicationName, &devSpec)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestGetAndAddComponentApplicationsToModel() unexpected error: %v", err)
			}

			if err != nil && tt.name == "Container image already exists" {
				// This is a user error scenario, so expect the "creation success" metric to be incremented (as only system errors are counted for failure)
				if testutil.ToFloat64(metrics.ApplicationCreationSucceeded) <= beforeCreateSucceedReqs {
					t.Errorf("TestGetAndAddComponentApplicationsToModel() expected metric 'ApplicationCreationSucceeded' to be incremented")
				}
				if testutil.ToFloat64(metrics.ApplicationCreationFailed) != beforeCreateFailedReqs {
					t.Errorf("TestGetAndAddComponentApplicationsToModel() expected metric 'ApplicationCreationFailed' to be unchanged")
				}
			}

			components := []appstudiov1alpha1.Component{}
			if tt.componentOne != nil {
				components = append(components, *tt.componentOne)
			}
			if tt.componentTwo != nil {
				components = append(components, *tt.componentTwo)
			}
			if err == nil && !tt.wantErr {
				for _, component := range components {
					if component.Spec.Source.GitSource != nil {
						projects := devSpec.Projects
						matched := false
						for _, project := range projects {
							projectGitSrc := project.ProjectSource.Git
							if project.Name == component.Spec.ComponentName && projectGitSrc != nil && projectGitSrc.Remotes["origin"] == component.Spec.Source.GitSource.URL {
								matched = true
							}
						}

						if !matched {
							t.Errorf("unable to find devfile with project: %s", component.Spec.ComponentName)
						}

					} else {
						devfileAttr := devSpec.Attributes
						if devfileAttr == nil {
							t.Errorf("devfile attributes should not be nil")
						}
						containerImage := devfileAttr.GetString("containerImage/new", &err)
						if err != nil {
							t.Errorf("got unexpected error: %v", err)
						}
						if containerImage != component.Spec.ContainerImage {
							t.Errorf("unable to find component with container iamge: %s", component.Spec.ContainerImage)
						}
					}
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
			corev1.ResourceCPU:     core500mResource,
			corev1.ResourceMemory:  storage1GiResource,
			corev1.ResourceStorage: storage1GiResource,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:     core500mResource,
			corev1.ResourceMemory:  storage1GiResource,
			corev1.ResourceStorage: storage1GiResource,
		},
	}

	envAttributes := attributes.Attributes{}.FromMap(map[string]interface{}{devfilePkg.ContainerENVKey: []corev1.EnvVar{{Name: "FOO", Value: "foo"}}}, &err)
	if err != nil {
		t.Error(err)
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
		wantReplica    *int
	}{
		{
			name: "No kubernetes component",
			components: []devfileAPIV1.Component{
				{
					Name: "component1",
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Container: &devfileAPIV1.ContainerComponent{},
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
			name: "image component with local Dockerfile uri updated to component's absolute DockerfileURL",
			components: []devfileAPIV1.Component{
				{
					Name:       "component1",
					Attributes: envAttributes.PutInteger(devfilePkg.ContainerImagePortKey, 1001),
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Kubernetes: &devfileAPIV1.KubernetesComponent{},
					},
				},
				{
					Name:       "component2",
					Attributes: envAttributes.PutInteger(devfilePkg.ContainerImagePortKey, 3333).PutString(devfilePkg.MemoryLimitKey, "2Gi"),
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Image: &devfileAPIV1.ImageComponent{

							Image: devfileAPIV1.Image{
								ImageUnion: devfileAPIV1.ImageUnion{
									Dockerfile: &devfileAPIV1.DockerfileImage{
										DockerfileSrc: devfileAPIV1.DockerfileSrc{
											Uri: "./dockerfile",
										},
									},
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
								URL:           "url",
								DockerfileURL: "https://website.com/dockerfiles/dockerfile",
							},
						},
					},
				},
			},
			updateExpected: true,
		},
		{
			name: "devfile with invalid components, error out when trying to update devfile's Dockerfile uri",
			components: []devfileAPIV1.Component{
				{
					Name:       "component1",
					Attributes: envAttributes.PutInteger(devfilePkg.ContainerImagePortKey, 1001),
					ComponentUnion: devfileAPIV1.ComponentUnion{
						ComponentType: "bad-component",
					},
				},
				{
					Name:       "component2",
					Attributes: envAttributes.PutInteger(devfilePkg.ContainerImagePortKey, 3333).PutString(devfilePkg.MemoryLimitKey, "2Gi"),
					ComponentUnion: devfileAPIV1.ComponentUnion{
						Image: &devfileAPIV1.ImageComponent{

							Image: devfileAPIV1.Image{
								ImageUnion: devfileAPIV1.ImageUnion{
									Dockerfile: &devfileAPIV1.DockerfileImage{
										DockerfileSrc: devfileAPIV1.DockerfileSrc{
											Uri: "./dockerfile",
										},
									},
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
								URL:           "url",
								DockerfileURL: "https://website.com/dockerfiles/dockerfile",
							},
						},
					},
					Route:      "route1",
					Replicas:   &oneReplica,
					TargetPort: 1111,
					Env:        env,
					Resources:  originalResources,
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
			err := r.updateComponentDevfileModel(ctrl.Request{}, devfileData, tt.component)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			}
		})
	}
}

func TestUpdateComponentStub(t *testing.T) {
	var err error
	envAttributes := attributes.Attributes{}.FromMap(map[string]interface{}{devfilePkg.ContainerENVKey: []corev1.EnvVar{{Name: "name1", Value: "value1"}}}, &err)
	if err != nil {
		t.Error(err)
	}
	ctx := context.TODO()
	fakeClientNoErr := NewFakeClient(t)
	fakeClientNoErr.MockGet = func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
		return nil
	}

	fakeClientWithErr := NewFakeClient(t)
	fakeClientWithErr.MockGet = func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
		return errors.NewNotFound(schema.GroupResource{}, "not found")
	}

	componentsValid := []devfileAPIV1.Component{
		{
			Name: "component1",
			Attributes: envAttributes.PutInteger(devfilePkg.ReplicaKey, 1).PutString(devfilePkg.RouteKey, "route1").PutInteger(
				devfilePkg.ContainerImagePortKey, 1001).PutString(devfilePkg.CpuLimitKey, "2").PutString(devfilePkg.CpuRequestKey, "700m").PutString(
				devfilePkg.MemoryLimitKey, "500Mi").PutString(devfilePkg.MemoryRequestKey, "400Mi").PutString(
				devfilePkg.StorageLimitKey, "400Mi").PutString(devfilePkg.StorageRequestKey, "200Mi"),
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{
					K8sLikeComponent: devfileAPIV1.K8sLikeComponent{
						K8sLikeComponentLocation: devfileAPIV1.K8sLikeComponentLocation{
							Uri: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/deploy.yaml",
						},
					},
				},
			},
		},
		{
			Name:       "component2",
			Attributes: attributes.Attributes{}.PutInteger(devfilePkg.ContainerImagePortKey, 1003),
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{
					K8sLikeComponent: devfileAPIV1.K8sLikeComponent{
						K8sLikeComponentLocation: devfileAPIV1.K8sLikeComponentLocation{
							Uri: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/deploy.yaml",
						},
					},
				},
			},
		},
	}

	componentsInvalidDeployYamlErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			Attributes: envAttributes.PutInteger(devfilePkg.ReplicaKey, 1).PutString(devfilePkg.RouteKey, "route1").PutInteger(
				devfilePkg.ContainerImagePortKey, 1001).PutString(devfilePkg.CpuLimitKey, "2").PutString(devfilePkg.CpuRequestKey, "700m").PutString(
				devfilePkg.MemoryLimitKey, "500Mi").PutString(devfilePkg.MemoryRequestKey, "400Mi").PutString(
				devfilePkg.StorageLimitKey, "400Mi").PutString(devfilePkg.StorageRequestKey, "200Mi"),
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{
					K8sLikeComponent: devfileAPIV1.K8sLikeComponent{
						K8sLikeComponentLocation: devfileAPIV1.K8sLikeComponentLocation{
							Uri: "testLocation/deploy.yaml",
						},
					},
				},
			},
		},
	}

	componentsValidWithPort := []devfileAPIV1.Component{
		{
			Name: "component1",
			Attributes: envAttributes.PutInteger(devfilePkg.ReplicaKey, 1).PutString(devfilePkg.RouteKey, "route1").PutInteger(
				devfilePkg.ContainerImagePortKey, 8080).PutString(devfilePkg.CpuLimitKey, "2").PutString(devfilePkg.CpuRequestKey, "700m").PutString(
				devfilePkg.MemoryLimitKey, "500Mi").PutString(devfilePkg.MemoryRequestKey, "400Mi").PutString(
				devfilePkg.StorageLimitKey, "400Mi").PutString(devfilePkg.StorageRequestKey, "200Mi"),
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{
					K8sLikeComponent: devfileAPIV1.K8sLikeComponent{
						K8sLikeComponentLocation: devfileAPIV1.K8sLikeComponentLocation{
							Uri: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/deploy.yaml",
						},
					},
				},
			},
		},
	}

	componentsReplicaErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.PutBoolean(devfilePkg.ReplicaKey, true),
		},
	}

	componentsContainerPortErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.PutBoolean(devfilePkg.ContainerImagePortKey, true),
		},
	}

	componentsRouteErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.Put(devfilePkg.RouteKey, []string{"a", "b"}, &err),
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
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.Put(devfilePkg.StorageLimitKey, []string{"a", "b"}, &err),
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
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.Put(devfilePkg.StorageRequestKey, []string{"a", "b"}, &err),
		},
	}
	if err != nil {
		t.Errorf("unexpected err: %+v", err)
		return
	}

	componentsCpuLimitErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.Put(devfilePkg.CpuLimitKey, []string{"a", "b"}, &err),
		},
	}
	if err != nil {
		t.Errorf("unexpected err: %+v", err)
		return
	}

	componentsCpuRequestErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.Put(devfilePkg.CpuRequestKey, []string{"a", "b"}, &err),
		},
	}
	if err != nil {
		t.Errorf("unexpected err: %+v", err)
		return
	}

	componentsMemoryLimitErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.Put(devfilePkg.MemoryLimitKey, []string{"a", "b"}, &err),
		},
	}
	if err != nil {
		t.Errorf("unexpected err: %+v", err)
		return
	}

	componentsMemoryRequestErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.Put(devfilePkg.MemoryRequestKey, []string{"a", "b"}, &err),
		},
	}
	if err != nil {
		t.Errorf("unexpected err: %+v", err)
		return
	}

	componentsCpuLimitParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.PutString(devfilePkg.CpuLimitKey, "xyz"),
		},
	}

	componentsMemoryLimitParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.PutString(devfilePkg.MemoryLimitKey, "xyz"),
		},
	}

	componentsStorageLimitParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.PutString(devfilePkg.StorageLimitKey, "xyz"),
		},
	}

	componentsCpuRequestParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.PutString(devfilePkg.CpuRequestKey, "xyz"),
		},
	}

	componentsMemoryRequestParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.PutString(devfilePkg.MemoryRequestKey, "xyz"),
		},
	}

	componentsStorageRequestParseErr := []devfileAPIV1.Component{
		{
			Name: "component1",
			ComponentUnion: devfileAPIV1.ComponentUnion{
				Kubernetes: &devfileAPIV1.KubernetesComponent{},
			},
			Attributes: attributes.Attributes{}.PutString(devfilePkg.StorageRequestKey, "xyz"),
		},
	}

	tests := []struct {
		name              string
		devfilesDataMap   map[string]*v2.DevfileV2
		devfilesURLMap    map[string]string
		dockerfileURLMap  map[string]string
		componentPortsMap map[string][]int
		isNil             bool
		testNoDup         bool
		wantErr           bool
	}{
		{
			name: "Kubernetes Components present",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.2.0",
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
			name: "Detected ports present and with component exist",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.2.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsValidWithPort,
							},
						},
					},
				},
			},
			testNoDup: true,
			componentPortsMap: map[string][]int{
				"./": {8080},
			},
		},
		{
			name: "Kubernetes Components present with a devfile URL",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.2.0",
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
			devfilesURLMap: map[string]string{
				"./": "http://somelink",
			},
		},
		{
			name: "Kubernetes Components present with a devfile & Dockerfile URL",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.2.0",
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
			devfilesURLMap: map[string]string{
				"./": "http://somelink",
			},
			dockerfileURLMap: map[string]string{
				"./": "http://someotherlink",
			},
		},
		{
			name: "Dockerfile URL only",
			dockerfileURLMap: map[string]string{
				"./": "http://someotherlink",
			},
		},
		{
			name: "Dockerfile URL with ports",
			dockerfileURLMap: map[string]string{
				"./": "Dockerfile",
			},
			componentPortsMap: map[string][]int{
				"./": {8080},
			},
		},
		{
			name: "No Kubernetes Components present",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.2.0",
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
							SchemaVersion: "2.2.0",
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
							SchemaVersion: "2.2.0",
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
			name: "Check err for container port as non integer",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.2.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsContainerPortErr,
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
							SchemaVersion: "2.2.0",
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
							SchemaVersion: "2.2.0",
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
			name: "Check err for storage request as non string",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.2.0",
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
			name: "Check err for cpu limit as non string",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.2.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsCpuLimitErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for cpu request as non string",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.2.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsCpuRequestErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for memory limit as non string",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.2.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsMemoryLimitErr,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Check err for memory request as non string",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.2.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsMemoryRequestErr,
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
							SchemaVersion: "2.2.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsCpuLimitParseErr,
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
							SchemaVersion: "2.2.0",
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
							SchemaVersion: "2.2.0",
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
			name: "Check err for cpu request parse err",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.2.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsCpuRequestParseErr,
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
							SchemaVersion: "2.2.0",
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
							SchemaVersion: "2.2.0",
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
			name: "Check err for invalid deploy yaml uri error",
			devfilesDataMap: map[string]*v2.DevfileV2{
				"./": {
					Devfile: devfileAPIV1.Devfile{
						DevfileHeader: devfile.DevfileHeader{
							SchemaVersion: "2.2.0",
							Metadata: devfile.DevfileMetadata{
								Name:        "test-devfile",
								Language:    "language",
								ProjectType: "project",
							},
						},
						DevWorkspaceTemplateSpec: devfileAPIV1.DevWorkspaceTemplateSpec{
							DevWorkspaceTemplateSpecContent: devfileAPIV1.DevWorkspaceTemplateSpecContent{
								Components: componentsInvalidDeployYamlErr,
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
			if tt.testNoDup {
				r.Client = fakeClientWithErr
			} else {
				r.Client = fakeClientNoErr
			}
			var err error
			if tt.isNil {
				err = r.updateComponentStub(ctrl.Request{}, ctx, nil, devfilesMap, nil, nil, nil)
			} else {
				err = r.updateComponentStub(ctrl.Request{}, ctx, &componentDetectionQuery, devfilesMap, tt.devfilesURLMap, tt.dockerfileURLMap, tt.componentPortsMap)
			}

			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("got unexpected error %v", err)
			} else if err == nil {
				for compName, hasCompDetection := range componentDetectionQuery.Status.ComponentDetected {
					if tt.testNoDup {
						assert.Equal(t, "url", compName, "The component name should match the expected name")
					} else {
						assert.NotEqual(t, "url", compName, "The component name should not exactly match the expected name")
					}
					// Application Name
					assert.Equal(t, hasCompDetection.ComponentStub.Application, "insert-application-name", "The application name should match the generic name")

					if len(tt.devfilesDataMap) != 0 {
						// Language
						assert.Equal(t, hasCompDetection.Language, tt.devfilesDataMap[hasCompDetection.ComponentStub.Source.GitSource.Context].Metadata.Language, "The language should be the same")

						// Project Type
						assert.Equal(t, hasCompDetection.ProjectType, tt.devfilesDataMap[hasCompDetection.ComponentStub.Source.GitSource.Context].Metadata.ProjectType, "The project type should be the same")

						// Devfile Found
						assert.Equal(t, hasCompDetection.DevfileFound, len(tt.devfilesURLMap[hasCompDetection.ComponentStub.Source.GitSource.Context]) != 0, "The devfile found did not match expected")

						// Component Name
						assert.Contains(t, hasCompDetection.ComponentStub.ComponentName, "url", "The component name did not match the expected")

						// Devfile URL
						if len(tt.devfilesURLMap) > 0 {
							assert.NotNil(t, hasCompDetection.ComponentStub.Source.GitSource, "The git source cannot be nil for this test")
							assert.Equal(t, hasCompDetection.ComponentStub.Source.GitSource.URL, "url", "The URL should match")
							assert.Equal(t, hasCompDetection.ComponentStub.Source.GitSource.DevfileURL, tt.devfilesURLMap[hasCompDetection.ComponentStub.Source.GitSource.Context], "The devfile URL should match")
						}

						// Dockerfile URL
						if len(tt.dockerfileURLMap) > 0 {
							assert.NotNil(t, hasCompDetection.ComponentStub.Source.GitSource, "The git source cannot be nil for this test")
							assert.Equal(t, hasCompDetection.ComponentStub.Source.GitSource.URL, "url", "The URL should match")
							assert.Equal(t, hasCompDetection.ComponentStub.Source.GitSource.DockerfileURL, tt.dockerfileURLMap[hasCompDetection.ComponentStub.Source.GitSource.Context], "The Dockerfile URL should match")
						}

						for _, devfileComponent := range tt.devfilesDataMap[hasCompDetection.ComponentStub.Source.GitSource.Context].Components {
							if devfileComponent.Kubernetes != nil {
								componentAttributes := devfileComponent.Attributes
								var containerENVs []corev1.EnvVar
								err := componentAttributes.GetInto(devfilePkg.ContainerENVKey, &containerENVs)
								assert.Nil(t, err, "err should be nil")
								for _, devfileEnv := range containerENVs {
									matched := false
									for _, compEnv := range hasCompDetection.ComponentStub.Env {
										if devfileEnv.Name == compEnv.Name && devfileEnv.Value == compEnv.Value {
											matched = true
										}
									}
									assert.True(t, matched, "env %s:%s should match", devfileEnv.Name, devfileEnv.Value)
								}

								limits := hasCompDetection.ComponentStub.Resources.Limits
								if len(limits) > 0 {
									resourceCPULimit := limits[corev1.ResourceCPU]
									assert.Equal(t, resourceCPULimit.String(), devfileComponent.Attributes.GetString(devfilePkg.CpuLimitKey, &err), "The cpu limit should be the same")
									assert.Nil(t, err, "err should be nil")

									resourceMemoryLimit := limits[corev1.ResourceMemory]
									assert.Equal(t, resourceMemoryLimit.String(), devfileComponent.Attributes.GetString(devfilePkg.MemoryLimitKey, &err), "The memory limit should be the same")
									assert.Nil(t, err, "err should be nil")

									resourceStorageLimit := limits[corev1.ResourceStorage]
									assert.Equal(t, resourceStorageLimit.String(), devfileComponent.Attributes.GetString(devfilePkg.StorageLimitKey, &err), "The storage limit should be the same")
									assert.Nil(t, err, "err should be nil")
								}

								requests := hasCompDetection.ComponentStub.Resources.Requests
								if len(requests) > 0 {
									resourceCPURequest := requests[corev1.ResourceCPU]
									assert.Equal(t, resourceCPURequest.String(), devfileComponent.Attributes.GetString(devfilePkg.CpuRequestKey, &err), "The cpu request should be the same")
									assert.Nil(t, err, "err should be nil")

									resourceMemoryRequest := requests[corev1.ResourceMemory]
									assert.Equal(t, resourceMemoryRequest.String(), devfileComponent.Attributes.GetString(devfilePkg.MemoryRequestKey, &err), "The memory request should be the same")
									assert.Nil(t, err, "err should be nil")

									resourceStorageRequest := requests[corev1.ResourceStorage]
									assert.Equal(t, resourceStorageRequest.String(), devfileComponent.Attributes.GetString(devfilePkg.StorageRequestKey, &err), "The storage request should be the same")
									assert.Nil(t, err, "err should be nil")
								}

								assert.Equal(t, *hasCompDetection.ComponentStub.Replicas, int(devfileComponent.Attributes.GetNumber(devfilePkg.ReplicaKey, &err)), "The replicas should be the same")
								assert.Nil(t, err, "err should be nil")

								assert.Equal(t, hasCompDetection.ComponentStub.TargetPort, int(devfileComponent.Attributes.GetNumber(devfilePkg.ContainerImagePortKey, &err)), "The target port should be the same")
								assert.Nil(t, err, "err should be nil")

								assert.Equal(t, hasCompDetection.ComponentStub.Route, devfileComponent.Attributes.GetString(devfilePkg.RouteKey, &err), "The route should be the same")
								assert.Nil(t, err, "err should be nil")

								break // dont check for the second Kubernetes component
							}
						}
					}

					if len(tt.dockerfileURLMap) != 0 {
						// Language
						assert.Equal(t, hasCompDetection.Language, "Dockerfile", "The language should be the same")

						// Project Type
						assert.Equal(t, hasCompDetection.ProjectType, "Dockerfile", "The project type should be the same")

						// Devfile Found
						assert.Equal(t, hasCompDetection.DevfileFound, false, "The devfile found did not match expected")

						// Component Name
						assert.Contains(t, hasCompDetection.ComponentStub.ComponentName, "url", "The component name did not match the expected")

						// Dockerfile URL
						if len(tt.dockerfileURLMap) > 0 {
							assert.NotNil(t, hasCompDetection.ComponentStub.Source.GitSource, "The git source cannot be nil for this test")
							assert.Equal(t, hasCompDetection.ComponentStub.Source.GitSource.URL, "url", "The URL should match")
							assert.Equal(t, hasCompDetection.ComponentStub.Source.GitSource.DockerfileURL, tt.dockerfileURLMap[hasCompDetection.ComponentStub.Source.GitSource.Context], "The Dockerfile URL should match")
						}
					}
				}
			}
		})
	}
}

func TestGetComponentName(t *testing.T) {
	// Repos used in tests are most likely dummy repos
	ctx := context.TODO()
	fakeClientNoErr := NewFakeClient(t)
	fakeClientNoErr.MockGet = func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
		return nil
	}

	fakeClientWithErr := NewFakeClient(t)
	fakeClientWithErr.MockGet = func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
		return errors.NewNotFound(schema.GroupResource{}, "not found")
	}

	r := ComponentDetectionQueryReconciler{
		Log: ctrl.Log.WithName("TestGetComponentName"),
	}

	tests := []struct {
		name                  string
		gitSource             *appstudiov1alpha1.GitSource
		testNoDup             bool
		expectedName          string
		generateComponentName bool
	}{
		{
			name: "valid repo name",
			gitSource: &appstudiov1alpha1.GitSource{
				URL: "https://github.com/devfile-samples/devfile-sample-go-basic",
			},
			testNoDup:             true,
			generateComponentName: false,
			expectedName:          "devfile-sample-go-basic",
		},
		{
			name: "long repo name with special chars",
			gitSource: &appstudiov1alpha1.GitSource{
				URL: "https://github.com/devfile-samples/123-testdevfilego--ImportRepository--withaverylongreporitoryname-test-validation-and-generation",
			},
			testNoDup:             true,
			generateComponentName: false,
			expectedName:          "comp-123-testdevfilego--importrepository--withaverylongrep",
		},
		{
			name: "numeric repo name",
			gitSource: &appstudiov1alpha1.GitSource{
				URL: "https://github.com/devfile-samples/123454678.git",
			},
			expectedName: "comp-123454678",
		},
		{
			name: "valid repo name with context",
			gitSource: &appstudiov1alpha1.GitSource{
				URL:     "https://github.com/devfile-samples/devfile-multi-component",
				Context: "nodejs",
			},
			expectedName:          "nodejs-devfile-multi-component",
			generateComponentName: true,
		},
		{
			name: "repo URL with forward slash at the end",
			gitSource: &appstudiov1alpha1.GitSource{
				URL: "https://github.com/devfile-samples/devfile-multi-component/",
			},
			expectedName:          "devfile-multi-component",
			generateComponentName: true,
		},
		{
			name: "repo URL with forward slash and context",
			gitSource: &appstudiov1alpha1.GitSource{
				URL:     "https://github.com/devfile-samples/devfile-multi-component/",
				Context: "nodejs",
			},
			expectedName: "nodejs-devfile-multi-component",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.testNoDup {
				r.Client = fakeClientWithErr
			} else {
				r.Client = fakeClientNoErr
			}

			gotComponentName := r.getComponentName(r.Log, ctx, "default", tt.gitSource, tt.generateComponentName)
			if !tt.generateComponentName && tt.testNoDup {
				assert.Equal(t, tt.expectedName, gotComponentName, "the component name should equal to repo name")
			} else {
				assert.Contains(t, gotComponentName, tt.expectedName, "the component name should contains the expected name")
				assert.NotEqual(t, tt.expectedName, gotComponentName, "the component name should not equal to repo name")
			}

		})
	}

}

func TestSanitizeComponentName(t *testing.T) {

	tests := []struct {
		name          string
		componentName string
		want          string
	}{
		{
			name:          "simple component name",
			componentName: "devfile-sample-go-basic",
			want:          "devfile-sample-go-basic",
		},
		{
			name:          "simple component name, all numbers",
			componentName: "123412341234",
			want:          "comp-123412341234",
		},
		{
			name:          "simple component name, start with a number",
			componentName: "123-testcomp",
			want:          "comp-123-testcomp",
		},
		{
			name:          "Empty string, should have a name generated for it",
			componentName: "",
		},
		{
			name:          "component name with uppercase",
			componentName: "devfile-SAMPLE-gO-BASIC",
			want:          "devfile-sample-go-basic",
		},
		{
			name:          "component name with greater than 58 characters",
			componentName: "devfile-sample-go-basic-devfile-sample-go-basic-devfile-sample",
			want:          "devfile-sample-go-basic-devfile-sample-go-basic-devfile-sa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitizedName := sanitizeComponentName(tt.componentName)

			if tt.componentName == "" {
				if sanitizedName == "" {
					t.Errorf("TestSanitizeComponentName(): expected generated name for empty component name, got %v", sanitizedName)
				}
			} else {
				if !strings.Contains(sanitizedName, tt.want) {
					t.Errorf("TestSanitizeComponentName(): want %v, got %v", tt.want, sanitizedName)
				}
			}

		})
	}

}
