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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Component Detection Query controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		HASAppName      = "test-application"
		HASCompName     = "test-component"
		HASCompDetQuery = "test-componentdetectionquery"
		HASNamespace    = "default"
		DisplayName     = "petclinic"
		Description     = "Simple petclinic app"
		ComponentName   = "backend"
		SampleRepoLink  = "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	)

	Context("Create Component Detection Query with URL set", func() {
		It("Should successfully detect a devfile", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "1"

			hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL: SampleRepoLink,
					},
				},
			}

			Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
			createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
				return len(createdHasCompDetectionQuery.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Make sure the a devfile is detected
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))

			for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect(devfileName).Should(ContainSubstring("spring"))
				Expect(devfileDesc.ComponentStub.Context).Should(ContainSubstring("./"))
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with devfileURL set", func() {
		It("Should successfully get a devfile", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "2"

			hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL:        SampleRepoLink,
						DevfileURL: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml",
					},
				},
			}

			Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
			createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
				return len(createdHasCompDetectionQuery.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Make sure the a devfile is detected
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))

			for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect(devfileName).Should(ContainSubstring("spring"))
				Expect(devfileDesc.ComponentStub.Context).Should(ContainSubstring("./"))
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with devfileURL and isMultiComponent set", func() {
		It("Should err out", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "3"

			hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					IsMultiComponent: true,
					GitSource: appstudiov1alpha1.GitSource{
						URL:        SampleRepoLink,
						DevfileURL: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml",
					},
				},
			}

			Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
			createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
				return len(createdHasCompDetectionQuery.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Make sure the right err is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[0].Message).Should(ContainSubstring("cannot set IsMultiComponent"))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with multi comp repo", func() {
		It("Should successfully get the devfiles", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "4"

			hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					IsMultiComponent: true,
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/maysunfaisal/multi-components",
					},
				},
			}

			Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
			createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
				return len(createdHasCompDetectionQuery.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Make sure the a devfile is detected
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(2))

			for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect(devfileName).Should(BeElementOf([]string{"java-springboot", "python"}))
				Expect(devfileDesc.ComponentStub.Context).Should(BeElementOf([]string{"devfile-sample-java-springboot-basic", "devfile-sample-python-basic"}))
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with multi comp repo that has no devfile", func() {
		It("Should err out", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "4"

			hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					IsMultiComponent: true,
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/octocat/Hello-World",
					},
				},
			}

			Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
			createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
				return len(createdHasCompDetectionQuery.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Make sure the right err is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[0].Message).Should(ContainSubstring("unable to find any devfile"))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with repo that has no devfile", func() {
		It("Should err out", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "4"

			hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/octocat/Hello-World",
					},
				},
			}

			Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
			createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
				return len(createdHasCompDetectionQuery.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Make sure the right err is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[0].Message).Should(ContainSubstring("unable to curl to any of known devfile locations"))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})
})

// deleteCompDetQueryCR deletes the specified Comp Detection Query resource and verifies it was properly deleted
func deleteCompDetQueryCR(hasCompDetectionQueryLookupKey types.NamespacedName) {
	// Delete
	Eventually(func() error {
		f := &appstudiov1alpha1.ComponentDetectionQuery{}
		k8sClient.Get(context.Background(), hasCompDetectionQueryLookupKey, f)
		return k8sClient.Delete(context.Background(), f)
	}, timeout, interval).Should(Succeed())

	// Wait for delete to finish
	Eventually(func() error {
		f := &appstudiov1alpha1.ComponentDetectionQuery{}
		return k8sClient.Get(context.Background(), hasCompDetectionQueryLookupKey, f)
	}, timeout, interval).ShouldNot(Succeed())
}
