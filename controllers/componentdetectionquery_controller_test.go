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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the a devfile is detected
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))

			for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect(devfileName).Should(ContainSubstring("spring"))
				Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(ContainSubstring("./"))
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the a devfile is detected
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))

			for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect(devfileName).Should(ContainSubstring("spring"))
				Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(ContainSubstring("./"))
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with devfileURL and isMultiComponent set", func() {
		It("Should use the devfileURL without err", func() {
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
					GitSource: appstudiov1alpha1.GitSource{
						URL:        "https://github.com/maysunfaisal/multi-components-none",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the status is successful
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery has successfully finished"))

			// Make sure the a devfile is detected
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))

			for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect(devfileName).Should(ContainSubstring("spring"))
				Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(ContainSubstring("./"))
			}

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
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/maysunfaisal/multi-components-none",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the a devfile is detected
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(2))

			for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect(devfileName).Should(Or(ContainSubstring("java-springboot"), ContainSubstring("python")))
				Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(BeElementOf([]string{"devfile-sample-java-springboot-basic", "devfile-sample-python-basic"}))
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with a non component devfile repo", func() {
		It("Should complete without any devfile detected", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "5"

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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the right err is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery has successfully finished"))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with multi component repo that has no devfile", func() {
		It("Should match a devfile with alizer if it can be a component", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "6"

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
						URL: "https://github.com/maysunfaisal/multi-components-none",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout20s, interval).Should(BeTrue())

			// Make sure the right err is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery has successfully finished"))

			// Make sure the a devfile is detected
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(2))

			for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect(devfileName).Should(Or(ContainSubstring("java-springboot"), ContainSubstring("python")))
				Expect(devfileDesc.ComponentStub.Source.GitSource).ShouldNot(BeNil())
				Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(BeElementOf([]string{"devfile-sample-java-springboot-basic", "devfile-sample-python-basic"}))
				Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).ShouldNot(BeEmpty())
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with repo that has no devfile", func() {
		It("Should match a devfile with alizer if it can be a component", func() {
			ctx := context.Background()

			queryName := "springboot" + HASCompDetQuery + "7" // this name is tied to mock fn in detect_mock.go

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
						URL: "https://github.com/maysunfaisal/devfile-sample-java-springboot-basic-1",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the right err is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery has successfully finished"))

			// Make sure the a devfile is detected
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))

			for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect([]string{devfileName}).Should(ContainElement(ContainSubstring("java-springboot")))
				Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(BeElementOf([]string{"./"}))
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with Devfile URL that does not exist", func() {
		It("Should err out", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "8"

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
						DevfileURL: "https://registry.devfile.io/devfiles/fake",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the right err is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("unable to GET from https://registry.devfile.io/devfiles/fake"))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with invalid Git URL", func() {
		It("Should err out", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "9"

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
						URL: "https://github.com/redhat-appstudio-appdata/!@#$%U%I$F    DFDN##",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the right err is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("parse \"https://github.com/redhat-appstudio-appdata/!@#$%U%I$F    DFDN##\": invalid URL escape \"%U%\""))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with private multicomponent Github repo", func() {
		It("Should err out due to authentication required error", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "10"

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
						URL: "https://github.com/johnmcollier/private-repo-test",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the right err is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("authentication required"))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	// Private repo tests
	Context("Create Component Detection Query with private git repo + invalid token", func() {
		It("Should err out due to invalid token", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "11"

			// Create a git secret
			tokenSecret := &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind: "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
				},
				StringData: map[string]string{
					"password": "fake-token",
				},
			}

			Expect(k8sClient.Create(ctx, tokenSecret)).Should(Succeed())

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
					Secret: queryName,
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// index is 1 because of CDQ status condition Processing
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Status).Should(Equal(metav1.ConditionFalse))
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery failed: authentication required"))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create multi Component Detection Query with private git repo + invalid token", func() {
		It("Should error out due to invalid token", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "12"

			// Create a git secret
			tokenSecret := &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind: "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
				},
				StringData: map[string]string{
					"password": "fake-token",
				},
			}

			Expect(k8sClient.Create(ctx, tokenSecret)).Should(Succeed())

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
					Secret: queryName,
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// index is 1 because of CDQ status condition Processing
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Status).Should(Equal(metav1.ConditionFalse))
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery failed: authentication required"))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	// Test when an error from the SPI client is returned
	// The Mock SPI client is configured to mock an error response if the repo name contains "test-error-response"
	Context("Create Component Detection Query with invalid token", func() {
		It("Should error out", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "13"

			// Create a git secret
			tokenSecret := &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind: "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
				},
				StringData: map[string]string{
					"password": "fake-token",
				},
			}

			Expect(k8sClient.Create(ctx, tokenSecret)).Should(Succeed())

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
					Secret: queryName,
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/test-repo/test-error-response",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// index is 1 because of CDQ status condition Processing
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Status).Should(Equal(metav1.ConditionFalse))
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("authentication required"))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with no secret", func() {
		It("Should error out since specified secret does not exist", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "14"

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
					Secret: queryName,
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/test-repo/testrepo",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the a devfile is detected
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Status).Should(Equal(metav1.ConditionFalse))
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring(fmt.Sprintf("ComponentDetectionQuery failed: Secret %q not found", queryName)))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with URL set to repo with invalid devfile", func() {
		It("Should detect a devfile but return an error", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "15"

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
						URL: "https://github.com/johnmcollier/test-bad-devfile",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure that the proper error condition is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Status).Should(Equal(metav1.ConditionFalse))
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery failed: failed to decode devfile json: json: cannot unmarshal string into Go value of type map[string]"))
			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with springboot repo that has no devfile", func() {
		It("Should match a devfile", func() {
			ctx := context.Background()

			queryName := "springboot" + HASCompDetQuery + "16"

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
						URL: "https://github.com/maysunfaisal/devfile-sample-java-springboot-basic-1",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the right status is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery has successfully finished"))

			// Make sure a devfile is matched
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))
			for _, componentDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect(componentDesc.ComponentStub.Source.GitSource).ShouldNot(BeNil())
				Expect(componentDesc.ComponentStub.Source.GitSource.DevfileURL).ShouldNot(BeEmpty())
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with URL set", func() {
		It("Should err out cloning on a fake sample", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "18"

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
						URL: "https://github.com/devfile-samples/fake-sample",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the a devfile is detected
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(0))

			// Make sure the right err is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery failed: authentication required"))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("A Requeued ComponentDetectionQuery", func() {
		It("Should delete itself", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "19"

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
						URL: "https://github.com/devfile-samples/fake-sample",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure no component was detected
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(0))

			// Make sure the right err is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery failed: authentication required"))

			// Trigger a requeue by updating the resource
			createdHasCompDetectionQuery.Spec.GitSource.URL = SampleRepoLink
			Expect(k8sClient.Update(ctx, createdHasCompDetectionQuery)).Should(Succeed())

			// Validate that the resource has been deleted
			hasCompDetQueryLookupKey = types.NamespacedName{Name: queryName, Namespace: HASNamespace}
			deletedCompDetQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, deletedCompDetQuery)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Create Component Detection Query with multi component repo that has no devfile or dockerfile", func() {
		It("Should attempt to match a devfile or dockerfile for every component", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "20"

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
						URL: "https://github.com/maysunfaisal/multi-components-dockerfile",
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
				return len(createdHasCompDetectionQuery.Status.Conditions) > 1
			}, timeout20s, interval).Should(BeTrue())

			// Make sure the right status is set
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery has successfully finished"))

			// Make sure the devfiles are detected
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(4)) // mocked, not accurate. check unit test for accurate detection that uses the alizer client instead of the mock client.
			for _, componentDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect(componentDesc.ComponentStub.Source.GitSource).ShouldNot(BeNil())
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})

		Context("Create Component Detection Query with a dockerfile repo", func() {
			It("Should return successfully", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "21"

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
							URL: "https://github.com/maysunfaisal/python-src-docker",
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
					return len(createdHasCompDetectionQuery.Status.Conditions) > 1
				}, timeout20s, interval).Should(BeTrue())

				// Make sure the right status is set
				Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery has successfully finished"))

				// Make sure the devfiles are detected
				Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))
				for _, componentDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
					Expect(componentDesc.ComponentStub.Source.GitSource).ShouldNot(BeNil())
					Expect(componentDesc.ComponentStub.Source.GitSource.DockerfileURL).ShouldNot(BeEmpty())
					Expect(componentDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("https://raw.githubusercontent.com/maysunfaisal/python-src-docker/main/./Dockerfile"))
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
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
