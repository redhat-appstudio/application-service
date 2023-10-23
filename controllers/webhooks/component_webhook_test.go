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
	"errors"
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

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
		ComponentName   = "backend"
		SampleRepoLink  = "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	)

	Context("Create Application CR with bad fields", func() {
		It("Should reject until all the fields are valid", func() {
			ctx := context.Background()

			uniqueHASCompName := HASCompName + "1"

			badHASCompName := "1-sdsfsdfsdf-bad-name"

			// Bad Component Name, Bad Application Name and no Src
			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "@#",
					Application:   "@#",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{},
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

			hasComp.Name = badHASCompName
			err = k8sClient.Create(ctx, hasComp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(fmt.Errorf(appstudiov1alpha1.InvalidDNS1035Name, hasComp.Name).Error()))
			hasComp.Name = uniqueHASCompName

			err = k8sClient.Create(ctx, hasComp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(appstudiov1alpha1.MissingGitOrImageSource))

			// Bad URL
			hasComp.Spec.Source.GitSource.URL = "badurl"
			err = k8sClient.Create(ctx, hasComp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errors.New("invalid URI for request" + appstudiov1alpha1.InvalidSchemeGitSourceURL).Error()))

			//Bad URL with unsupported vendor
			hasComp.Spec.Source.GitSource.URL = "http://url"
			err = k8sClient.Create(ctx, hasComp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(fmt.Errorf(appstudiov1alpha1.InvalidGithubVendorURL, "http://url", SupportedGitRepo).Error()))

			// Good URL
			hasComp.Spec.Source.GitSource.URL = SampleRepoLink
			err = k8sClient.Create(ctx, hasComp)
			Expect(err).Should(BeNil())

			// Look up the has app resource that was created.
			hasCompLookupKey := types.NamespacedName{Name: uniqueHASCompName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(ctx, hasCompLookupKey, createdHasComp)
				return !reflect.DeepEqual(createdHasComp, &appstudiov1alpha1.Component{})
			}, timeout, interval).Should(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)
		})
	})

	Context("Update Application CR fields", func() {
		It("Should update non immutable fields successfully and err out on immutable fields", func() {
			ctx := context.Background()

			uniqueHASCompName := HASCompName + "2"

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueHASCompName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: ComponentName,
					Application:   HASAppName,
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: SampleRepoLink,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the has app resource that was created.
			hasCompLookupKey := types.NamespacedName{Name: uniqueHASCompName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(ctx, hasCompLookupKey, createdHasComp)
				return !reflect.DeepEqual(createdHasComp, &appstudiov1alpha1.Component{})
			}, timeout, interval).Should(BeTrue())

			// Update the Comp application
			createdHasComp.Spec.Application = "newapp"
			err := k8sClient.Update(ctx, createdHasComp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(fmt.Errorf(appstudiov1alpha1.ApplicationNameUpdateError, createdHasComp.Spec.Application).Error()))

			// Update the Comp component name
			createdHasComp.Spec.Application = hasComp.Spec.Application
			createdHasComp.Spec.ComponentName = "newcomp"
			err = k8sClient.Update(ctx, createdHasComp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(fmt.Errorf(appstudiov1alpha1.ComponentNameUpdateError, createdHasComp.Spec.ComponentName).Error()))

			// Update the Comp git src
			createdHasComp.Spec.ComponentName = hasComp.Spec.ComponentName
			createdHasComp.Spec.Source.GitSource.Context = hasComp.Spec.Source.GitSource.Context
			createdHasComp.Spec.Source.GitSource = &appstudiov1alpha1.GitSource{
				URL: "newlink",
			}
			err = k8sClient.Update(ctx, createdHasComp)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(fmt.Errorf(appstudiov1alpha1.GitSourceUpdateError, *createdHasComp.Spec.Source.GitSource).Error()))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)
		})
	})

	Context("Create Application CR with invalid build-nudges-ref", func() {
		It("Should reject until it's resolved", func() {
			ctx := context.Background()

			uniqueHASCompName := HASCompName + "3"

			nudgedComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: HASAppNamespace,
					Name:      uniqueHASCompName + "-nudge",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: uniqueHASCompName + "-nudge",
					Application:   "test-application",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: SampleRepoLink,
							},
						},
					},
				},
			}

			err := k8sClient.Create(ctx, nudgedComp)
			Expect(err).Should(Not(HaveOccurred()))

			// comp in a different app
			differentAppComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: HASAppNamespace,
					Name:      uniqueHASCompName + "-new-app",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: uniqueHASCompName + "-new-app",
					Application:   "test-application2",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: SampleRepoLink,
							},
						},
					},
				},
			}

			err = k8sClient.Create(ctx, differentAppComp)
			Expect(err).Should(Not(HaveOccurred()))

			// nudgingComp
			nudgingComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: HASAppNamespace,
					Name:      uniqueHASCompName,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName:  uniqueHASCompName,
					Application:    "test-application",
					BuildNudgesRef: []string{uniqueHASCompName},
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: SampleRepoLink,
							},
						},
					},
				},
			}

			err = k8sClient.Create(ctx, nudgingComp)
			Expect(err).Should((HaveOccurred()))
			Expect(err.Error()).Should(ContainSubstring("cycle detected"))

			// After changing to a valid build nudges ref, create should succeed
			nudgingComp.Spec.BuildNudgesRef = []string{uniqueHASCompName + "-nudge"}
			err = k8sClient.Create(ctx, nudgingComp)
			Expect(err).Should(BeNil())

			// Look up the has app resource that was created.
			hasCompLookupKey := types.NamespacedName{Name: uniqueHASCompName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(ctx, hasCompLookupKey, createdHasComp)
				return !reflect.DeepEqual(createdHasComp, &appstudiov1alpha1.Component{})
			}, timeout, interval).Should(BeTrue())

			// Now attempt to update the build-nudges-ref field to an invalid component (different app)
			createdHasComp.Spec.BuildNudgesRef = []string{uniqueHASCompName + "-new-app"}
			err = k8sClient.Update(ctx, createdHasComp)
			Expect(err).Should((HaveOccurred()))
			Expect(err.Error()).Should(ContainSubstring("belongs to a different application"))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)
		})
	})

})

// deleteHASCompCR deletes the specified hasComp resource and verifies it was properly deleted
func deleteHASCompCR(hasCompLookupKey types.NamespacedName) {
	// Delete
	Eventually(func() error {
		f := &appstudiov1alpha1.Component{}
		k8sClient.Get(context.Background(), hasCompLookupKey, f)
		return k8sClient.Delete(context.Background(), f)
	}, timeout, interval).Should(Succeed())

	// Wait for delete to finish
	Eventually(func() error {
		f := &appstudiov1alpha1.Component{}
		return k8sClient.Get(context.Background(), hasCompLookupKey, f)
	}, timeout, interval).ShouldNot(Succeed())
}
