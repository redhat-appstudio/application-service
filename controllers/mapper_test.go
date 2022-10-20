//
// Copyright 2022 Red Hat, Inc.
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
	"fmt"
	"testing"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Adapted from https://github.com/codeready-toolchain/host-operator/blob/master/controllers/spacebindingcleanup/mapper_test.go
func TestMapToBindingByBoundObject(t *testing.T) {

	const (
		HASAppName      = "test-app"
		HASCompName     = "test-comp"
		HASSnapshotName = "test-snapshot"
		HASBindingName  = "test-binding"
		Namespace       = "default"
		DisplayName     = "an environment"
		ComponentName   = "backend"
		SampleRepoLink  = "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	)

	applicationName := HASAppName + "1"
	applicationName2 := HASAppName + "2"
	componentName := HASCompName + "1"
	componentName2 := HASCompName + "2"
	snapshotName := HASSnapshotName + "1"
	snapshotName2 := HASSnapshotName + "2"
	bindingName := HASBindingName + "1"
	bindingName2 := HASBindingName + "2"
	bindingName3 := HASBindingName + "3"
	bindingName4 := HASBindingName + "4"
	staging := "staging"
	dev := "dev"
	prod := "prod"
	replicas := int32(3)

	// given
	binding1 := &appstudiov1alpha1.ApplicationSnapshotEnvironmentBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "ApplicationSnapshotEnvironmentBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: Namespace,
			Labels: map[string]string{
				"appstudio.environment": staging,
				"appstudio.application": applicationName,
			},
		},
		Spec: appstudiov1alpha1.ApplicationSnapshotEnvironmentBindingSpec{
			Application: applicationName,
			Environment: staging,
			Snapshot:    snapshotName,
			Components: []appstudiov1alpha1.BindingComponent{
				{
					Name: componentName,
					Configuration: appstudiov1alpha1.BindingComponentConfiguration{
						Replicas: int(replicas),
					},
				},
			},
		},
	}

	binding2 := &appstudiov1alpha1.ApplicationSnapshotEnvironmentBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "ApplicationSnapshotEnvironmentBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName2,
			Namespace: Namespace,
			Labels: map[string]string{
				"appstudio.environment": dev,
				"appstudio.application": applicationName2,
			},
		},
		Spec: appstudiov1alpha1.ApplicationSnapshotEnvironmentBindingSpec{
			Application: applicationName2,
			Environment: dev,
			Snapshot:    snapshotName2,
			Components: []appstudiov1alpha1.BindingComponent{
				{
					Name: componentName2,
					Configuration: appstudiov1alpha1.BindingComponentConfiguration{
						Replicas: int(replicas),
					},
				},
			},
		},
	}

	binding3 := &appstudiov1alpha1.ApplicationSnapshotEnvironmentBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "ApplicationSnapshotEnvironmentBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName3,
			Namespace: Namespace,
			Labels: map[string]string{
				"appstudio.environment": staging,
				"appstudio.application": applicationName2,
			},
		},
		Spec: appstudiov1alpha1.ApplicationSnapshotEnvironmentBindingSpec{
			Application: applicationName2,
			Environment: staging,
			Snapshot:    snapshotName,
			Components: []appstudiov1alpha1.BindingComponent{
				{
					Name: componentName,
					Configuration: appstudiov1alpha1.BindingComponentConfiguration{
						Replicas: int(replicas),
					},
				},
			},
		},
	}

	binding4 := &appstudiov1alpha1.ApplicationSnapshotEnvironmentBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "ApplicationSnapshotEnvironmentBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName4,
			Namespace: Namespace,
		},
		Spec: appstudiov1alpha1.ApplicationSnapshotEnvironmentBindingSpec{
			Application: applicationName,
			Environment: staging,
			Snapshot:    snapshotName,
			Components: []appstudiov1alpha1.BindingComponent{
				{
					Name: componentName,
					Configuration: appstudiov1alpha1.BindingComponentConfiguration{
						Replicas: int(replicas),
					},
				},
			},
		},
	}

	stagingEnv := &appstudiov1alpha1.Environment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Environment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      staging,
			Namespace: Namespace,
		},
		Spec: appstudiov1alpha1.EnvironmentSpec{
			Type:               "POC",
			DisplayName:        DisplayName,
			DeploymentStrategy: appstudiov1alpha1.DeploymentStrategy_AppStudioAutomated,
			Configuration: appstudiov1alpha1.EnvironmentConfiguration{
				Env: []appstudiov1alpha1.EnvVarPair{
					{
						Name:  "FOO",
						Value: "BAR",
					},
				},
			},
		},
	}

	prodEnv := &appstudiov1alpha1.Environment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Environment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      prod,
			Namespace: Namespace,
		},
		Spec: appstudiov1alpha1.EnvironmentSpec{
			Type:               "Non-POC",
			DisplayName:        DisplayName,
			DeploymentStrategy: appstudiov1alpha1.DeploymentStrategy_AppStudioAutomated,
			Configuration: appstudiov1alpha1.EnvironmentConfiguration{
				Env: []appstudiov1alpha1.EnvVarPair{
					{
						Name:  "FOO",
						Value: "BAR",
					},
				},
			},
		},
	}

	fakeClient := NewFakeClient(t, binding1, binding2, binding3, binding4)

	t.Run("should return two Binding requests for staging Env", func(t *testing.T) {
		// when
		requests := MapToBindingByBoundObjectName(fakeClient, "Environment", "appstudio.environment")(stagingEnv)

		// then
		require.Len(t, requests, 2) // binding4 is not returned because binding4 does not have a label matching the staging env
		assert.Contains(t, requests, newRequest(binding1.Name))
		assert.Contains(t, requests, newRequest(binding3.Name))
	})

	t.Run("should return no Binding requests for prod Env", func(t *testing.T) {
		// when
		requests := MapToBindingByBoundObjectName(fakeClient, "Environment", "appstudio.environment")(prodEnv)

		// then
		require.Empty(t, requests)
	})

	t.Run("should return no Binding requests when Binding list fails", func(t *testing.T) {
		fakeClient.MockList = func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
			return fmt.Errorf("some error")
		}
		// when
		requests := MapToBindingByBoundObjectName(fakeClient, "Environment", "appstudio.environment")(prodEnv)

		// then
		require.Empty(t, requests)
	})
}

func newRequest(name string) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "default",
			Name:      name,
		},
	}
}

// NewFakeClient creates a fake K8s client with ability to override specific List function
// Adapted from https://github.com/codeready-toolchain/toolchain-common/blob/master/pkg/test/client.go#L19
// fake client pkg by default does not allow mocking or injecting errors,
// see https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/client/fake/doc.go#L31
func NewFakeClient(t *testing.T, initObjs ...runtime.Object) *FakeClient {
	s := scheme.Scheme
	err := appstudiov1alpha1.AddToScheme(s)
	require.NoError(t, err)
	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithRuntimeObjects(initObjs...).
		Build()
	return &FakeClient{Client: cl}
}

type FakeClient struct {
	client.Client
	MockList func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
	MockGet  func(ctx context.Context, key types.NamespacedName, obj client.Object) error
}

func (c *FakeClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if c.MockList != nil {
		return c.MockList(ctx, list, opts...)
	}
	return c.Client.List(ctx, list, opts...)
}

func (c *FakeClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object) error {
	if c.MockGet != nil {
		return c.MockGet(ctx, key, obj)
	}
	return c.Client.Get(ctx, key, obj)
}
