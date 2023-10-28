//
// Copyright 2022-2023 Red Hat, Inc.
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

package webhooks

import (
	"context"
	"fmt"
	"testing"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestComponentCreateValidatingWebhook(t *testing.T) {

	tests := []struct {
		name    string
		newComp appstudiov1alpha1.Component
		err     string
	}{
		{
			name: "component metadata.name is invalid",
			err:  "invalid component name",
			newComp: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name: "1-test-component",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
					Application:   "application1",
				},
			},
		},
		{
			name: "component cannot be created due to bad URL",
			err:  "invalid URI for request" + appstudiov1alpha1.InvalidSchemeGitSourceURL,
			newComp: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-component",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
					Application:   "application1",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "badurl",
							},
						},
					},
				},
			},
		},
		{
			name: "component needs to have one source specified",
			err:  appstudiov1alpha1.MissingGitOrImageSource,
			newComp: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-component",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
					Application:   "application1",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{},
						},
					},
				},
			},
		},
		{
			name: "valid component with invalid git vendor src",
			err:  fmt.Errorf(appstudiov1alpha1.InvalidGithubVendorURL, "http://url", SupportedGitRepo).Error(),
			newComp: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-component",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
					Application:   "application1",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "http://url",
							},
						},
					},
				},
			},
		},
		{
			name: "valid component with invalid git scheme src",
			err:  "invalid URI for request" + appstudiov1alpha1.InvalidSchemeGitSourceURL,
			newComp: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-component",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
					Application:   "application1",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "git@github.com:devfile-samples/devfile-sample-java-springboot-basic.git",
							},
						},
					},
				},
			},
		},
		{
			name: "valid component with container image",
			newComp: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-component",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName:  "component1",
					Application:    "application1",
					ContainerImage: "image",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compWebhook := ComponentWebhook{
				log: zap.New(zap.UseFlagOptions(&zap.Options{
					Development: true,
					TimeEncoder: zapcore.ISO8601TimeEncoder,
				})),
			}
			err := compWebhook.ValidateCreate(context.Background(), &test.newComp)

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.err)
			}
		})
	}
}

func TestComponentUpdateValidatingWebhook(t *testing.T) {

	originalComponent := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName: "component",
			Application:   "application",
			Source: appstudiov1alpha1.ComponentSource{
				ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
					GitSource: &appstudiov1alpha1.GitSource{
						URL:     "http://link",
						Context: "context",
					},
				},
			},
		},
	}

	tests := []struct {
		name       string
		updateComp appstudiov1alpha1.Component
		err        string
	}{
		{
			name: "component name cannot be changed",
			err:  fmt.Errorf(appstudiov1alpha1.ComponentNameUpdateError, "component1").Error(),
			updateComp: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
				},
			},
		},
		{
			name: "application name cannot be changed",
			err:  fmt.Errorf(appstudiov1alpha1.ApplicationNameUpdateError, "application1").Error(),
			updateComp: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component",
					Application:   "application1",
				},
			},
		},
		{
			name: "git src url cannot be changed",
			err: fmt.Errorf(appstudiov1alpha1.GitSourceUpdateError, appstudiov1alpha1.GitSource{
				URL:     "http://link1",
				Context: "context",
			}).Error(),
			updateComp: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component",
					Application:   "application",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL:     "http://link1",
								Context: "context",
							},
						},
					},
				},
			},
		},
		{
			name: "non-url git source can be changed",
			updateComp: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component",
					Application:   "application",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								Context:       "new-context",
								DevfileURL:    "https://new-devfile-url",
								DockerfileURL: "https://new-dockerfile-url",
							},
						},
					},
				},
			},
		},
		{
			name: "container image can be changed",
			updateComp: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName:  "component",
					Application:    "application",
					ContainerImage: "image1",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.err == "" {
				originalComponent = appstudiov1alpha1.Component{
					Spec: appstudiov1alpha1.ComponentSpec{
						ComponentName:  "component",
						Application:    "application",
						ContainerImage: "image",
						Source: appstudiov1alpha1.ComponentSource{
							ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
								GitSource: &appstudiov1alpha1.GitSource{
									Context: "context",
								},
							},
						},
					},
				}
			}
			var err error
			compWebhook := ComponentWebhook{
				log: zap.New(zap.UseFlagOptions(&zap.Options{
					Development: true,
					TimeEncoder: zapcore.ISO8601TimeEncoder,
				})),
			}
			err = compWebhook.ValidateUpdate(context.Background(), &originalComponent, &test.updateComp)

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.err)
			}
		})
	}
}

func TestComponentDeleteValidatingWebhook(t *testing.T) {

	tests := []struct {
		name    string
		newComp appstudiov1alpha1.Component
		err     string
	}{
		{
			name:    "ValidateDelete should return nil, it's unimplemented",
			err:     "",
			newComp: appstudiov1alpha1.Component{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compWebhook := ComponentWebhook{
				log: zap.New(zap.UseFlagOptions(&zap.Options{
					Development: true,
					TimeEncoder: zapcore.ISO8601TimeEncoder,
				})),
			}
			err := compWebhook.ValidateDelete(context.Background(), &test.newComp)

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.err)
			}
		})
	}
}

func TestValidateBuildNudgesRefGraph(t *testing.T) {
	fakeClient := setUpComponents(t)
	fakeErrorClient := NewFakeErrorClient(t)

	compWebhook := ComponentWebhook{
		client: fakeClient,
		log: zap.New(zap.UseFlagOptions(&zap.Options{
			Development: true,
			TimeEncoder: zapcore.ISO8601TimeEncoder,
		})),
	}

	errCompWebhook := ComponentWebhook{
		client: fakeErrorClient,
		log: zap.New(zap.UseFlagOptions(&zap.Options{
			Development: true,
			TimeEncoder: zapcore.ISO8601TimeEncoder,
		})),
	}

	tests := []struct {
		name     string
		compName string
		webhook  ComponentWebhook
		errStr   string
	}{
		{
			name:     "simple component relationship, no errors",
			compName: "component1",
			webhook:  compWebhook,
		},
		{
			name:     "component references itself",
			compName: "component-self-ref",
			webhook:  compWebhook,
			errStr:   "cycle detected: component component-self-ref cannot reference itself, directly or indirectly, via build-nudges-ref",
		},
		{
			name:     "nudged component belongs to different app",
			compName: "component-invalid-app",
			webhook:  compWebhook,
			errStr:   "component component4 cannot be added to spec.build-nudges-ref as it belongs to a different application",
		},
		{
			name:     "complex component relationship - some valid, some not valid (self referential)",
			compName: "complexComponent",
			webhook:  compWebhook,
			errStr:   "cycle detected: component complexComponent cannot reference itself, directly or indirectly, via build-nudges-ref",
		},
		{
			name:     "unrelated get error from kubernetes",
			compName: "component1",
			webhook:  errCompWebhook,
			errStr:   "some error",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			component := &appstudiov1alpha1.Component{}
			fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: test.compName}, component)

			err := test.webhook.validateBuildNudgesRefGraph(context.Background(), component.Spec.BuildNudgesRef, "default", test.compName, component.Spec.Application)
			var errStr string
			if err != nil {
				errStr = err.Error()
			}
			if errStr != test.errStr {
				t.Errorf("TestValidateBuildNudgesRefGraph() unexpected error value: want %v, got %v", test.errStr, errStr)
			}
		})
	}
}

// setUpComponents creates a fake controller-runtime Kube client with components to test the build-nudges-ref field
func setUpComponents(t *testing.T) client.WithWatch {
	s := scheme.Scheme
	err := appstudiov1alpha1.AddToScheme(s)
	require.NoError(t, err)
	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		Build()

	component1 := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      "component1",
			Namespace: "default",
		},
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName:  "component1",
			Application:    "application1",
			BuildNudgesRef: []string{"component2"},
		},
	}
	err = fakeClient.Create(context.Background(), &component1)
	require.NoError(t, err)

	component2 := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      "component2",
			Namespace: "default",
		},
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName:  "component2",
			Application:    "application1",
			BuildNudgesRef: []string{"component3"},
		},
	}
	err = fakeClient.Create(context.Background(), &component2)
	require.NoError(t, err)

	component3 := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      "component3",
			Namespace: "default",
		},
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName: "component3",
			Application:   "application1",
		},
	}
	err = fakeClient.Create(context.Background(), &component3)
	require.NoError(t, err)

	componentSelfReference := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      "component-self-ref",
			Namespace: "default",
		},
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName:  "component-self-ref",
			Application:    "application1",
			BuildNudgesRef: []string{"component-self-ref"},
		},
	}
	err = fakeClient.Create(context.Background(), &componentSelfReference)
	require.NoError(t, err)

	componentInvalidApp := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      "component-invalid-app",
			Namespace: "default",
		},
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName:  "component-invalid-app",
			Application:    "application1",
			BuildNudgesRef: []string{"component4"},
		},
	}
	err = fakeClient.Create(context.Background(), &componentInvalidApp)
	require.NoError(t, err)

	component4 := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      "component4",
			Namespace: "default",
		},
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName: "component2",
			Application:   "application2",
		},
	}
	err = fakeClient.Create(context.Background(), &component4)
	require.NoError(t, err)

	complexComponent := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      "complexComponent",
			Namespace: "default",
		},
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName:  "complexComponent",
			Application:    "application1",
			BuildNudgesRef: []string{"component1", "complexComponentNudged"},
		},
	}
	err = fakeClient.Create(context.Background(), &complexComponent)
	require.NoError(t, err)

	complexComponentNudged := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      "complexComponentNudged",
			Namespace: "default",
		},
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName:  "complexComponentNudged",
			Application:    "application1",
			BuildNudgesRef: []string{"component5", "component6", "component7"},
		},
	}
	err = fakeClient.Create(context.Background(), &complexComponentNudged)
	require.NoError(t, err)

	component5 := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      "component5",
			Namespace: "default",
		},
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName: "component5",
			Application:   "application1",
		},
	}
	err = fakeClient.Create(context.Background(), &component5)
	require.NoError(t, err)

	component6 := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      "component6",
			Namespace: "default",
		},
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName:  "component6",
			Application:    "application1",
			BuildNudgesRef: []string{"component8"},
		},
	}
	err = fakeClient.Create(context.Background(), &component6)
	require.NoError(t, err)

	component7 := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      "component7",
			Namespace: "default",
		},
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName:  "component7",
			Application:    "application1",
			BuildNudgesRef: []string{"component9"},
		},
	}
	err = fakeClient.Create(context.Background(), &component7)
	require.NoError(t, err)

	component8 := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      "component8",
			Namespace: "default",
		},
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName: "component8",
			Application:   "application1",
		},
	}
	err = fakeClient.Create(context.Background(), &component8)
	require.NoError(t, err)

	component9 := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name:      "component9",
			Namespace: "default",
		},
		TypeMeta: v1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName:  "component9",
			Application:    "application1",
			BuildNudgesRef: []string{"complexComponent"},
		},
	}
	err = fakeClient.Create(context.Background(), &component9)
	require.NoError(t, err)

	return fakeClient
}

// NewFakeErrorClient returns a fake Kube client whose get method returns an error
// Currently it always returns an error, but can be modified in the future to selectively return errors
func NewFakeErrorClient(t *testing.T, initObjs ...runtime.Object) *FakeClient {
	s := scheme.Scheme
	err := appstudiov1alpha1.AddToScheme(s)
	require.NoError(t, err)
	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithRuntimeObjects(initObjs...).
		Build()
	return &FakeClient{Client: cl, MockGet: func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
		return fmt.Errorf("some error")
	}}
}

type FakeClient struct {
	client.Client
	MockList func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
	MockGet  func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error
}

func (c *FakeClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	if c.MockGet != nil {
		return c.MockGet(ctx, key, obj)
	}
	return c.Client.Get(ctx, key, obj)
}
