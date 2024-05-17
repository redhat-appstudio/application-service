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

func TestMapApplicationToComponent(t *testing.T) {

	const (
		HASAppName     = "test-app"
		HASCompName    = "test-comp"
		Namespace      = "default"
		DisplayName    = "an application"
		ComponentName  = "backend"
		SampleRepoLink = "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	)

	applicationName := HASAppName + "1"
	componentName := HASCompName + "1"
	componentName2 := HASCompName + "2"

	componentOne := appstudiov1alpha1.Component{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Component",
			APIVersion: "appstudio.redhat.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentName,
			Namespace: Namespace,
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName: componentName,
			Application:   applicationName,
		},
	}
	componentTwo := appstudiov1alpha1.Component{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Component",
			APIVersion: "appstudio.redhat.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentName2,
			Namespace: Namespace,
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName: componentName2,
		},
	}

	//fakeClient := NewFakeClient(t, componentOne, applicationOne)

	t.Run("should return component's parent application", func(t *testing.T) {
		// when
		requests := MapComponentToApplication()(&componentOne)

		// then
		require.Len(t, requests, 1) // binding4 is not returned because binding4 does not have a label matching the staging env
		assert.Contains(t, requests, newRequest(applicationName))
	})

	t.Run("should return no Application requests when Component app name is nil", func(t *testing.T) {
		// when
		requests := MapComponentToApplication()(&componentTwo)

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
	MockGet  func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error
}

func (c *FakeClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if c.MockList != nil {
		return c.MockList(ctx, list, opts...)
	}
	return c.Client.List(ctx, list, opts...)
}

func (c *FakeClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	if c.MockGet != nil {
		return c.MockGet(ctx, key, obj)
	}
	return c.Client.Get(ctx, key, obj)
}
