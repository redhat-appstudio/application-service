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

package contracts

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	appstudiov1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/onsi/ginkgo"
	gomega "github.com/onsi/gomega"
	models "github.com/pact-foundation/pact-go/v2/models"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

const timeout = 10 * time.Second
const interval = 250 * time.Millisecond

func createApp(setup bool, state models.ProviderState) (models.ProviderStateResponse, error) {
	if !setup {
		err := os.Setenv("SETUP", "false")
		gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Setting up env var failed: %s", err))
		return nil, nil
	}
	err := os.Setenv("SETUP", "true")
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Setting up env var failed: %s", err))

	params := parseApplication(state.Parameters)
	hasApp := getApplicationSpec(params.appName, params.namespace)
	hasAppLookupKey := types.NamespacedName{Name: params.appName, Namespace: params.namespace}
	createdHasApp := &appstudiov1alpha1.Application{}

	// create app
	err = k8sClient.Create(ctx, hasApp)
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Failed to create application: %s", err))

	// check it is created
	gomega.Eventually(func() bool {
		err := k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
		gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Failed to get application: %s", err))
		return len(createdHasApp.Status.Conditions) > 0
	}, timeout, interval).Should(gomega.BeTrue())

	return nil, nil

}

func createComponents(setup bool, state models.ProviderState) (models.ProviderStateResponse, error) {
	if !setup {
		err := os.Setenv("SETUP", "false")
		gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Setting up env var failed: %s", err))
		return nil, nil
	}
	err := os.Setenv("SETUP", "true")
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Setting up env var failed: %s", err))

	components := parseComponents(state.Parameters)
	for _, comp := range components {
		ghComp := getGhComponentSpec(comp.name, comp.app.namespace, comp.app.appName, comp.repo)

		hasAppLookupKey := types.NamespacedName{Name: comp.app.appName, Namespace: comp.app.namespace}
		createdHasApp := &appstudiov1alpha1.Application{}

		//create gh component
		err := k8sClient.Create(ctx, ghComp)
		gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Failed to craete component: %s", err))
		hasCompLookupKey := types.NamespacedName{Name: comp.name, Namespace: comp.app.namespace}
		createdHasComp := &appstudiov1alpha1.Component{}

		// wait until component is created
		gomega.Eventually(func() bool {
			err := k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Failed to get component: %s", err))
			return len(createdHasComp.Status.Conditions) > 1
		}, timeout, interval).Should(gomega.BeTrue())

		// wait until component is ready
		gomega.Eventually(func() bool {
			err := k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Failed to get application: %s", err))
			return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, comp.name)
		}, timeout, interval).Should(gomega.BeTrue())
	}
	return nil, nil

}

func removeAllInstacesInNamespace(namespace string, myInstance client.Object) {
	objectKind := strings.Split(reflect.TypeOf(myInstance).String(), ".")[1]
	remainingCount := getObjectCountInNamespace(objectKind, namespace)
	if remainingCount == 0 {
		return
	}
	// remove resources in namespace
	err := k8sClient.DeleteAllOf(context.Background(), myInstance, client.InNamespace(namespace))
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Failed to delete %s: %s", myInstance, err))

	// watch number of resources existing
	gomega.Eventually(func() bool {
		objectKind := strings.Split(reflect.TypeOf(myInstance).String(), ".")[1]
		remainingCount := getObjectCountInNamespace(objectKind, namespace)
		fmt.Fprintf(ginkgo.GinkgoWriter, "Removing %s instance from %s namespace. Remaining: %d", objectKind, namespace, remainingCount)
		return remainingCount == 0
	}, timeout, interval).Should(gomega.BeTrue())
}

func getObjectCountInNamespace(objectKind string, namespace string) int {
	unstructuredObject := &unstructured.Unstructured{}

	unstructuredObject.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   appstudiov1alpha1.GroupVersion.Group,
		Version: appstudiov1alpha1.GroupVersion.Version,
		Kind:    objectKind,
	})

	err := k8sClient.List(context.Background(), unstructuredObject, &client.ListOptions{Namespace: namespace})
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Failed to get list of %s: %s", objectKind, err))

	listOfObjects, _ := unstructuredObject.ToList()
	return len(listOfObjects.Items)
}

func cleanUpNamespaces() {
	fmt.Fprint(ginkgo.GinkgoWriter, "clean up namespaces")
	removeAllInstances(&appstudiov1alpha1.Component{})
	removeAllInstances(&appstudiov1alpha1.Application{})
	removeAllInstances(&appstudiov1alpha1.ComponentDetectionQuery{})
}

// remove all instances of the given type within the whole cluster
func removeAllInstances(myInstance client.Object) {
	listOfNamespaces := getListOfNamespaces()
	for _, item := range listOfNamespaces.Items {
		removeAllInstacesInNamespace(item.Name, myInstance)
	}
}

// return all namespaces where the instances of the specified object kind exist
func getListOfNamespaces() core.NamespaceList {
	namespaceList := &core.NamespaceList{}
	err := k8sClient.List(context.Background(), namespaceList, &client.ListOptions{Namespace: ""})
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Failed to get list of namespaces: %s", err))
	return *namespaceList
}
