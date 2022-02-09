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

package v1alpha1

import (
	"context"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

const (
	timeout  = time.Second * 10
	duration = time.Second * 10
	interval = time.Millisecond * 250
)

var _ = Describe("Application validation webhook", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		HASAppName      = "test-application"
		HASAppNamespace = "default"
		DisplayName     = "petclinic"
		Description     = "Simple petclinic app"
	)

	Context("Update Application CR fields", func() {
		It("Should update non immutable fields successfully and err out on immutable fields", func() {
			ctx := context.Background()

			hasApp := &Application{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Application",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      HASAppName,
					Namespace: HASAppNamespace,
				},
				Spec: ApplicationSpec{
					DisplayName: DisplayName,
					Description: Description,
					AppModelRepository: ApplicationGitRepository{
						URL: "https://github.com/testorg/petclinic-app",
					},
					GitOpsRepository: ApplicationGitRepository{
						URL: "https://github.com/testorg/gitops-app",
					},
				},
			}

			Expect(k8sClient.Create(ctx, hasApp)).Should(Succeed())

			// Look up the has app resource that was created.
			// These tests do not go through reconcile, so no need to check reconcile logic here
			hasAppLookupKey := types.NamespacedName{Name: HASAppName, Namespace: HASAppNamespace}
			fetchedHasApp := &Application{}
			Eventually(func() bool {
				k8sClient.Get(ctx, hasAppLookupKey, fetchedHasApp)
				return !reflect.DeepEqual(fetchedHasApp, &Application{})
			}, timeout, interval).Should(BeTrue())

			// Update the hasApp resource
			fetchedHasApp.Spec.DisplayName = "newname"
			fetchedHasApp.Spec.Description = "New Description"
			fetchedHasApp.Spec.AppModelRepository.URL = "newurl"
			err := k8sClient.Update(ctx, fetchedHasApp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("app model repository cannot be updated"))

			// revert appmodel but update gitops
			fetchedHasApp.Spec.AppModelRepository.URL = "https://github.com/testorg/petclinic-app"
			fetchedHasApp.Spec.GitOpsRepository.URL = "https://github.com/testorg/petclinic-app"

			err = k8sClient.Update(ctx, fetchedHasApp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("gitops repository cannot be updated"))

			// Delete the specified resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

})

// deleteHASAppCR deletes the specified hasApp resource and verifies it was properly deleted
func deleteHASAppCR(hasAppLookupKey types.NamespacedName) {
	// Delete
	Eventually(func() error {
		f := &Application{}
		k8sClient.Get(context.Background(), hasAppLookupKey, f)
		return k8sClient.Delete(context.Background(), f)
	}, timeout, interval).Should(Succeed())

	// Wait for delete to finish
	Eventually(func() error {
		f := &Application{}
		return k8sClient.Get(context.Background(), hasAppLookupKey, f)
	}, timeout, interval).ShouldNot(Succeed())
}
