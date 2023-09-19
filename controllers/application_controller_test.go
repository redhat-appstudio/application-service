/*
Copyright 2021-2023 Red Hat, Inc.

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
	"time"

	"github.com/devfile/library/v2/pkg/devfile/parser"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	cdqanalysis "github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

const (
	timeout    = time.Second * 10
	timeout20s = time.Second * 20
	timeout40s = time.Second * 40
	duration   = time.Second * 10
	interval   = time.Millisecond * 250
)

var _ = Describe("Application controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		HASAppName      = "test-application"
		HASAppNamespace = "default"
		DisplayName     = "petclinic"
		Description     = "Simple petclinic app"
	)

	Context("Create Application with no repositories set", func() {
		It("Should create successfully with generated repositories", func() {
			ctx := context.Background()

			applicationName := HASAppName + "1"

			hasApp := &appstudiov1alpha1.Application{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Application",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      applicationName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ApplicationSpec{
					DisplayName: DisplayName,
					Description: Description,
				},
			}

			Expect(k8sClient.Create(ctx, hasApp)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			devfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasApp.Status.Devfile)})
			Expect(err).Should(Not(HaveOccurred()))

			// gitOpsRepo and appModelRepo should both be set
			Expect(string(devfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(Equal("")))
			Expect(string(devfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(Not(Equal("")))

			// Delete the specified resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Application with no repositories set", func() {
		It("Should create successfully with generated repositories", func() {
			ctx := context.Background()

			hasApp := &appstudiov1alpha1.Application{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Application",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      HASAppName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ApplicationSpec{
					DisplayName: DisplayName,
					Description: Description,
				},
			}

			Expect(k8sClient.Create(ctx, hasApp)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasAppLookupKey := types.NamespacedName{Name: HASAppName, Namespace: HASAppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			devfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasApp.Status.Devfile)})

			Expect(err).Should(Not(HaveOccurred()))

			// gitOpsRepo and appModelRepo should both be set
			Expect(string(devfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(Equal("")))
			Expect(string(devfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(Not(Equal("")))

			// Delete the specified resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Application with no appmodel repository set", func() {
		It("Should create successfully with appmodel repository set to gitops repository", func() {
			ctx := context.Background()

			applicationName := HASAppName + "2"

			hasApp := &appstudiov1alpha1.Application{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Application",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      applicationName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ApplicationSpec{
					DisplayName: DisplayName,
					Description: Description,
					GitOpsRepository: appstudiov1alpha1.ApplicationGitRepository{
						URL: "https://github.com/testorg/petclinic-gitops",
					},
				},
			}

			Expect(k8sClient.Create(ctx, hasApp)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			devfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasApp.Status.Devfile)})

			Expect(err).Should(Not(HaveOccurred()))

			// gitOpsRepo and appModelRepo should both be set
			Expect(string(devfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(ContainSubstring(hasApp.Spec.GitOpsRepository.URL))
			Expect(string(devfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(Not(Equal("")))
			Expect(string(devfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(ContainSubstring(hasApp.Spec.GitOpsRepository.URL))

			// Delete the specified resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Application with no gitops repository set", func() {
		It("Should create successfully with generated gitops repository", func() {
			ctx := context.Background()

			applicationName := HASAppName + "3"

			hasApp := &appstudiov1alpha1.Application{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Application",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      applicationName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ApplicationSpec{
					DisplayName: DisplayName,
					Description: Description,
					AppModelRepository: appstudiov1alpha1.ApplicationGitRepository{
						URL: "https://github.com/testorg/petclinic-app",
					},
				},
			}

			Expect(k8sClient.Create(ctx, hasApp)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}

			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			devfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasApp.Status.Devfile)})

			Expect(err).Should(Not(HaveOccurred()))

			// gitOpsRepo and appModelRepo should both be set
			Expect(string(devfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should((ContainSubstring(hasApp.Spec.AppModelRepository.URL)))
			Expect(string(devfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(Equal("")))
			Expect(string(devfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(ContainSubstring(hasApp.Spec.AppModelRepository.URL)))

			// Delete the specified resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Update Application CR fields", func() {
		It("Should update successfully with updated description", func() {
			ctx := context.Background()

			applicationName := HASAppName + "4"

			hasApp := &appstudiov1alpha1.Application{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Application",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      applicationName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ApplicationSpec{
					DisplayName: DisplayName,
					Description: Description,
					AppModelRepository: appstudiov1alpha1.ApplicationGitRepository{
						URL: "https://github.com/testorg/petclinic-app",
					},
				},
			}

			Expect(k8sClient.Create(ctx, hasApp)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			fetchedHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, fetchedHasApp)
				return len(fetchedHasApp.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Update the name and description of the hasApp resource
			fetchedHasApp.Spec.DisplayName = "newname"
			fetchedHasApp.Spec.Description = "New Description"
			Expect(k8sClient.Update(context.Background(), fetchedHasApp)).Should(Succeed())

			// Make sure the devfile model was properly set
			Expect(fetchedHasApp.Status.Devfile).Should(Not(Equal("")))

			// Get the updated resource
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, fetchedHasApp)
				// Return true if the most recent condition on the CR is updated
				return fetchedHasApp.Status.Conditions[len(fetchedHasApp.Status.Conditions)-1].Type == "Updated"
			}, timeout, interval).Should(BeTrue())

			devfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(fetchedHasApp.Status.Devfile)})

			Expect(err).Should(Not(HaveOccurred()))
			Expect(string(devfile.GetMetadata().Name)).Should(Equal("newname"))
			Expect(string(devfile.GetMetadata().Description)).Should(Equal("New Description"))

			// Delete the specified resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Application CR with gitops repo creation failure", func() {
		It("Should update successfully with updated description", func() {
			ctx := context.Background()

			applicationName := "test-error-response" + "5"

			hasApp := &appstudiov1alpha1.Application{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Application",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      applicationName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ApplicationSpec{
					DisplayName: "test-error-response",
					Description: Description,
					AppModelRepository: appstudiov1alpha1.ApplicationGitRepository{
						URL: "https://github.com/testorg/petclinic-app",
					},
				},
			}

			Expect(k8sClient.Create(ctx, hasApp)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			fetchedHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, fetchedHasApp)
				return len(fetchedHasApp.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			Expect(fetchedHasApp.Status.Conditions[0].Status).Should(Equal(metav1.ConditionFalse))

			// Delete the specified resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

})

// deleteHASAppCR deletes the specified hasApp resource and verifies it was properly deleted
func deleteHASAppCR(hasAppLookupKey types.NamespacedName) {
	// Delete
	Eventually(func() error {
		f := &appstudiov1alpha1.Application{}
		k8sClient.Get(context.Background(), hasAppLookupKey, f)
		return k8sClient.Delete(context.Background(), f)
	}, timeout, interval).Should(Succeed())

	// Wait for delete to finish
	Eventually(func() error {
		f := &appstudiov1alpha1.Application{}
		return k8sClient.Get(context.Background(), hasAppLookupKey, f)
	}, timeout, interval).ShouldNot(Succeed())
}
