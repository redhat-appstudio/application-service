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

package controllers

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
	//+kubebuilder:scaffold:imports
)

// Test that the "finalize counter" works properly.
//
// If the "finalize counter" works properly, the controller will permit the finalizer to fail up to 5 times before removing
// the finalizer and deleting the resource regardless. If the finalizer counter does not work, the controller will keep trying to remove this resource
// and the test will fail.
//
// There are a few ways to trigger the finalizer to fail:
// 1. GitOps repo deletion fails (not currently possible with our mock go-github client)
// 2. Invalid devfile set in the Application CR status
// 3. Invalid GitOps URL set in the Application CR
//
// The following tests cover the last two scenarios.
var _ = Describe("Application controller finalizer counter tests", func() {

	const (
		AppName      = "test-application"
		AppNamespace = "default"
		DisplayName  = "petclinic"
		Description  = "Simple petclinic app"
	)

	Context("Delete Application CR fields with invalid devfile", func() {
		It("Should delete successfully even when finalizer fails after 5 times", func() {
			// Create a simple Application CR and get its devfile
			fetchedApp := createAndFetchSimpleApp(AppName, AppNamespace, DisplayName, Description)
			curDevfile, err := devfile.ParseDevfileModel(fetchedApp.Status.Devfile)

			// Make sure the devfile model was properly set
			Expect(fetchedApp.Status.Devfile).Should(Not(Equal("")))

			// Set an invalid gitops URL
			devfileMeta := curDevfile.GetMetadata()
			devfileMeta.Attributes.Put("gitOpsRepository.url", "https://github.com/redhat-appstudio-appdata//sdfd-d,cx.x sd sd", &err)
			curDevfile.SetMetadata(devfileMeta)
			_, err = yaml.Marshal(curDevfile)
			Expect(err).ToNot(HaveOccurred())
			fetchedApp.Status.Devfile = "a"
			Expect(k8sClient.Status().Update(context.Background(), fetchedApp)).Should(Succeed())

			// Get the updated resource
			hasAppLookupKey := types.NamespacedName{Name: AppName, Namespace: AppNamespace}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, fetchedApp)
				// Return true if the most recent condition on the CR is updated
				return fetchedApp.Status.Devfile == "a"
			}, timeout, interval).Should(BeTrue())

			// Delete the specified resource
			Eventually(func() error {
				return k8sClient.Delete(context.Background(), fetchedApp)
			}, timeout, interval).Should(Succeed())

			// Wait for delete to finish
			Eventually(func() error {
				f := &appstudiov1alpha1.Application{}
				return k8sClient.Get(context.Background(), hasAppLookupKey, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})

	Context("Delete Application CR with invalid gitops repository", func() {
		It("Should delete successfully even if finalizer fails to delete gitops repository", func() {
			// Create an Application resource and get its devfile
			fetchedHasApp := createAndFetchSimpleApp(AppName, AppNamespace, DisplayName, Description)
			Expect(fetchedHasApp.Status.Devfile).Should(Not(Equal("")))
			curDevfile, err := devfile.ParseDevfileModel(fetchedHasApp.Status.Devfile)

			// Set an invalid gitops URL and update the status of the resource
			devfileMeta := curDevfile.GetMetadata()
			devfileMeta.Attributes.Put("gitOpsRepository.url", "redhat-appstudio-appdata", &err)
			curDevfile.SetMetadata(devfileMeta)
			devfileYaml, err := yaml.Marshal(curDevfile)
			Expect(err).ToNot(HaveOccurred())
			fetchedHasApp.Status.Devfile = string(devfileYaml)
			Expect(k8sClient.Status().Update(context.Background(), fetchedHasApp)).Should(Succeed())

			// Get the updated resource
			hasAppLookupKey := types.NamespacedName{Name: AppName, Namespace: AppNamespace}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, fetchedHasApp)

				// Return true if the fetched resource has our "updated" devfile status
				return fetchedHasApp.Status.Devfile == string(devfileYaml)
			}, timeout, interval).Should(BeTrue())

			// Delete the specified resource
			Eventually(func() error {
				return k8sClient.Delete(context.Background(), fetchedHasApp)
			}, timeout, interval).Should(Succeed())

			// Wait for delete to finish
			Eventually(func() error {
				f := &appstudiov1alpha1.Application{}
				return k8sClient.Get(context.Background(), hasAppLookupKey, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})

})

// Simple function to create, retrieve from k8s, and return a simple Application CR
func createAndFetchSimpleApp(name string, namespace string, display string, description string) *appstudiov1alpha1.Application {
	ctx := context.Background()

	hasApp := &appstudiov1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appstudiov1alpha1.ApplicationSpec{
			DisplayName: display,
			Description: description,
		},
	}

	Expect(k8sClient.Create(ctx, hasApp)).Should(Succeed())

	// Look up the has app resource that was created.
	// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
	hasAppLookupKey := types.NamespacedName{Name: name, Namespace: namespace}
	fetchedHasApp := &appstudiov1alpha1.Application{}
	Eventually(func() bool {
		k8sClient.Get(context.Background(), hasAppLookupKey, fetchedHasApp)
		return len(fetchedHasApp.Status.Conditions) > 0
	}, timeout, interval).Should(BeTrue())

	return fetchedHasApp
}

func TestGetFinalizeCount(t *testing.T) {
	tests := []struct {
		name        string
		application appstudiov1alpha1.Application
		want        int
	}{
		{
			name:        "Application, no finalize counter",
			application: appstudiov1alpha1.Application{},
			want:        0,
		},
		{
			name: "Application, finalize counter already exists",
			application: appstudiov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						finalizeCount: "4",
					},
				},
			},
			want: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := getFinalizeCount(&tt.application)
			if err != nil {
				t.Errorf("TestGetFinalizeCount() unexpected error: %v", err)
			}
			if count != tt.want {
				t.Errorf("TestGetFinalizeCount() error: expected %v got %v", tt.want, count)
			}
		})
	}

}

func TestSetFinalizeCount(t *testing.T) {
	tests := []struct {
		name        string
		application appstudiov1alpha1.Application
		count       int
		want        int
	}{
		{
			name:        "Application, no finalize counter",
			application: appstudiov1alpha1.Application{},
			count:       0,
			want:        0,
		},
		{
			name: "Application, finalize counter already exists",
			application: appstudiov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						finalizeCount: "4",
					},
				},
			},
			count: 4,
			want:  4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setFinalizeCount(&tt.application, tt.count)
			count, err := getFinalizeCount(&tt.application)
			if err != nil {
				t.Errorf("TestGetFinalizeCount() unexpected error: %v", err)
			}
			if count != tt.want {
				t.Errorf("TestGetFinalizeCount() error: expected %v got %v", tt.want, count)
			}
		})
	}

}
