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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Application validation webhook", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		HASAppName      = "test-application-123"
		HASCompName     = "test-component-123"
		HASAppNamespace = "default"
		DisplayName     = "petclinic"
		Description     = "Simple petclinic app"
		ComponentName   = "backend"
		SampleRepoLink  = "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	)

	Context("Create Application CR with bad fields", func() {
		It("Should reject until all the fields are valid", func() {
			ctx := context.Background()

			uniqueHASCompName := HASCompName + "1"

			// Bad Component Name, Bad Application Name and no Src
			hasComp := &Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueHASCompName,
					Namespace: HASAppNamespace,
				},
				Spec: ComponentSpec{
					ComponentName: "@#",
					Application:   "@#",
					Source: ComponentSource{
						ComponentSourceUnion: ComponentSourceUnion{
							GitSource: &GitSource{},
						},
					},
				},
			}

			err := k8sClient.Create(ctx, hasComp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("spec.componentName in body should match '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'"))
			Expect(err.Error()).Should(ContainSubstring("spec.application in body should match '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'"))

			hasComp.Spec.ComponentName = ComponentName
			hasComp.Spec.Application = HASAppName

			err = k8sClient.Create(ctx, hasComp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("git source or an image source must be specified"))

			// Bad URL
			hasComp.Spec.Source.GitSource.URL = "badurl"
			err = k8sClient.Create(ctx, hasComp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("invalid URI for request"))

			// Good URL
			hasComp.Spec.Source.GitSource.URL = SampleRepoLink
			err = k8sClient.Create(ctx, hasComp)
			Expect(err).Should(BeNil())

			// Look up the has app resource that was created.
			hasCompLookupKey := types.NamespacedName{Name: uniqueHASCompName, Namespace: HASAppNamespace}
			createdHasComp := &Component{}
			Eventually(func() bool {
				k8sClient.Get(ctx, hasCompLookupKey, createdHasComp)
				return !reflect.DeepEqual(createdHasComp, &Component{})
			}, timeout, interval).Should(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)
		})
	})

	Context("Update Application CR fields", func() {
		It("Should update non immutable fields successfully and err out on immutable fields", func() {
			ctx := context.Background()

			uniqueHASCompName := HASCompName + "2"

			hasComp := &Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueHASCompName,
					Namespace: HASAppNamespace,
				},
				Spec: ComponentSpec{
					ComponentName: ComponentName,
					Application:   HASAppName,
					Source: ComponentSource{
						ComponentSourceUnion: ComponentSourceUnion{
							GitSource: &GitSource{
								URL: SampleRepoLink,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the has app resource that was created.
			hasCompLookupKey := types.NamespacedName{Name: uniqueHASCompName, Namespace: HASAppNamespace}
			createdHasComp := &Component{}
			Eventually(func() bool {
				k8sClient.Get(ctx, hasCompLookupKey, createdHasComp)
				return !reflect.DeepEqual(createdHasComp, &Component{})
			}, timeout, interval).Should(BeTrue())

			// Update the Comp application
			createdHasComp.Spec.Application = "newapp"
			err := k8sClient.Update(ctx, createdHasComp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("application name cannot be updated"))

			// Update the Comp component name
			createdHasComp.Spec.Application = hasComp.Spec.Application
			createdHasComp.Spec.ComponentName = "newcomp"
			err = k8sClient.Update(ctx, createdHasComp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("component name cannot be updated"))

			// Update the Comp git src
			createdHasComp.Spec.ComponentName = hasComp.Spec.ComponentName
			createdHasComp.Spec.Source.GitSource.Context = hasComp.Spec.Source.GitSource.Context
			createdHasComp.Spec.Source.GitSource = &GitSource{
				URL: "newlink",
			}
			err = k8sClient.Update(ctx, createdHasComp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("git source cannot be updated"))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)
		})
	})

})

// deleteHASCompCR deletes the specified hasComp resource and verifies it was properly deleted
func deleteHASCompCR(hasCompLookupKey types.NamespacedName) {
	// Delete
	Eventually(func() error {
		f := &Component{}
		k8sClient.Get(context.Background(), hasCompLookupKey, f)
		return k8sClient.Delete(context.Background(), f)
	}, timeout, interval).Should(Succeed())

	// Wait for delete to finish
	Eventually(func() error {
		f := &Component{}
		return k8sClient.Get(context.Background(), hasCompLookupKey, f)
	}, timeout, interval).ShouldNot(Succeed())
}
