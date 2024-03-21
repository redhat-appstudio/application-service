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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redhat-appstudio/application-service/pkg/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
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
		ComponentName   = "devfile-sample-java-springboot-basic"
		SampleRepoLink  = "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	)

	prometheus.MustRegister(metrics.ImportGitRepoTotalReqs, metrics.ImportGitRepoFailed, metrics.ImportGitRepoSucceeded)

	// test CDQ module
	Context("Create Component Detection Query with URL set", func() {
		It("Should successfully detect a devfile", func() {
			ctx := context.Background()
			beforeImportGitRepoTotalReqs := testutil.ToFloat64(metrics.ImportGitRepoTotalReqs)
			beforeImportGitRepoFailed := testutil.ToFloat64(metrics.ImportGitRepoFailed)
			beforeImportGitRepoSucceeded := testutil.ToFloat64(metrics.ImportGitRepoSucceeded)

			queryName := HASCompDetQuery + "1"

			hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL:      SampleRepoLink,
						Revision: "main",
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
				Expect(devfileDesc.ComponentStub.Source.GitSource.Revision).Should(ContainSubstring("main"))
				Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"))
				Expect(devfileDesc.DevfileFound).Should(BeTrue())
			}
			Expect(testutil.ToFloat64(metrics.ImportGitRepoTotalReqs) > beforeImportGitRepoTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.ImportGitRepoSucceeded) > beforeImportGitRepoSucceeded).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.ImportGitRepoFailed) == beforeImportGitRepoFailed).To(BeTrue())

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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
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
				Expect(devfileDesc.ComponentStub.ComponentName).Should(ContainSubstring(ComponentName))
				Expect(devfileDesc.ComponentStub.ComponentName).Should(Equal(ComponentName))
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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL:        "https://github.com/devfile-resources/multi-components-none",
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
				Expect(devfileName).Should(ContainSubstring("multi-components-none"))
				Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(ContainSubstring("./"))
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with multi comp repo with no devfiles", func() {
		It("Should successfully get the correct devfiles and Dockerfiles", func() {
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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/devfile-resources/multi-components-none",
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

			// Make sure the a devfile is detected
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(2))

			for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect(devfileName).Should(Or(ContainSubstring("java-springboot"), ContainSubstring("python")))
				Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(BeElementOf([]string{"devfile-sample-java-springboot-basic", "devfile-sample-python-basic"}))
				if strings.Contains(devfileName, "java-springboot") {
					Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile"))
				} else if strings.Contains(devfileName, "python") {
					Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile"))
				}
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with a non component devfile repo", func() {
		It("Should complete with no error if no devfile or Dockerfile detected", func() {
			ctx := context.Background()
			beforeImportGitRepoTotalReqs := testutil.ToFloat64(metrics.ImportGitRepoTotalReqs)
			beforeImportGitRepoSucceeded := testutil.ToFloat64(metrics.ImportGitRepoSucceeded)
			beforeImportGitRepoFailed := testutil.ToFloat64(metrics.ImportGitRepoFailed)
			queryName := HASCompDetQuery + "5"

			hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL:      "https://github.com/octocat/Hello-World",
						Revision: "master",
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

			// Make sure the cdq complete with success status
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery has successfully finished"))

			Expect(testutil.ToFloat64(metrics.ImportGitRepoTotalReqs) > beforeImportGitRepoTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.ImportGitRepoSucceeded) > beforeImportGitRepoSucceeded).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.ImportGitRepoFailed) == beforeImportGitRepoFailed).To(BeTrue())

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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/devfile-resources/multi-components-none",
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

			queryName := "python-src-none" + HASCompDetQuery + "7" // this name is tied to mock fn in detect_mock.go

			hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/devfile-resources/python-src-none",
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
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))

			for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect([]string{devfileName}).Should(ContainElement(ContainSubstring("python-src-none")))
				Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(BeElementOf([]string{"./"}))
				Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml"))
				Expect(devfileDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile"))
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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
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
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery failed: unable to GET"))

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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL:      "https://github.com/redhat-appstudio-appdata/!@#$%U%I$F    DFDN##",
						Revision: "main",
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
			beforeImportGitRepoTotalReqs := testutil.ToFloat64(metrics.ImportGitRepoTotalReqs)
			beforeImportGitRepoFailed := testutil.ToFloat64(metrics.ImportGitRepoFailed)
			beforeImportGitRepoSucceeded := testutil.ToFloat64(metrics.ImportGitRepoSucceeded)
			queryName := HASCompDetQuery + "10"

			hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/devfile-resources/multi-components-private",
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
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("authentication failed"))

			Expect(testutil.ToFloat64(metrics.ImportGitRepoTotalReqs) > beforeImportGitRepoTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.ImportGitRepoSucceeded) > beforeImportGitRepoSucceeded).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.ImportGitRepoFailed) == beforeImportGitRepoFailed).To(BeTrue())

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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					Secret: queryName,
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/devfile-resources/multi-components-private",
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
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("authentication failed"))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with private git repo + valid mock token", func() {
		It("Should successfully detect the component from the repo", func() {
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
					"password": "valid-mock-token", // string is tied to mock clone func in cdq-analysis/pkg/componentdetectionquery_mock.go
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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					Secret: queryName,
					GitSource: appstudiov1alpha1.GitSource{
						URL: SampleRepoLink, // using an actual public repo here for testing a valid token case, see cdq-analysis/pkg/componentdetectionquery_mock.go for more information
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
				Expect(devfileDesc.ComponentStub.Source.GitSource.Revision).Should(ContainSubstring("main"))
				Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"))
				Expect(devfileDesc.DevfileFound).Should(BeTrue())
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with private git repo with devfileURL set + valid mock token", func() {
		It("Should err out", func() {
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
					"password": "valid-mock-token", // string is tied to mock clone func in cdq-analysis/pkg/componentdetectionquery_mock.go
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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					Secret: queryName,
					GitSource: appstudiov1alpha1.GitSource{
						DevfileURL: "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml",
						URL:        SampleRepoLink, // using an actual public repo here for testing a valid token case, see cdq-analysis/pkg/componentdetectionquery_mock.go for more information
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
				Expect(devfileDesc.ComponentStub.Source.GitSource.Revision).Should(ContainSubstring("main"))
				Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"))
				Expect(devfileDesc.DevfileFound).Should(BeTrue())
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with private git repo and DevfileURL set + invalid token", func() {
		It("Should err out due to invalid token", func() {
			ctx := context.Background()

			queryName := HASCompDetQuery + "14"

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

			devfileURL := "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"

			hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					Secret: queryName,
					GitSource: appstudiov1alpha1.GitSource{
						DevfileURL: devfileURL,
						URL:        SampleRepoLink,
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
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("unable to GET from %s", devfileURL))

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with no secret", func() {
		It("Should error out since specified secret does not exist", func() {
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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
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

			queryName := HASCompDetQuery + "16"

			hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/devfile-resources/test-bad-devfile",
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
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("cannot unmarshal string into Go value of type map[string]interface"))
			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})
	})

	Context("Create Component Detection Query with springboot repo that has devfile", func() {
		It("Should return a correct devfile when repo URL has leading and trailing spaces", func() {
			ctx := context.Background()

			queryName := "springboot" + HASCompDetQuery + "17"

			hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryName,
					Namespace: HASNamespace,
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL: "   https://github.com/devfile-samples/devfile-sample-java-springboot-basic   ",
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
				Expect(componentDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"))
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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/devfile-samples/fake-sample.git",
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
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery failed"))

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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
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
			Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery failed"))

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

	Context("Create Component Detection Query with multi component repo that has no devfile or Dockerfile", func() {
		It("Should attempt to match a devfile or Dockerfile for every component", func() {
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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/devfile-resources/multi-components-dockerfile",
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
			Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(5)) // mocked, not accurate. check unit test for accurate detection that uses the alizer client instead of the mock client.
			for _, componentDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
				Expect(componentDesc.ComponentStub.Source.GitSource).ShouldNot(BeNil())
			}

			// Delete the specified Detection Query resource
			deleteCompDetQueryCR(hasCompDetQueryLookupKey)
		})

		Context("Create Component Detection Query with a Dockerfile repo", func() {
			It("Should return successfully", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "python-src-docker" + "21"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL:      "https://github.com/devfile-resources/python-src-docker",
							Revision: "testbranch",
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
					Expect(componentDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("Dockerfile"))
					Expect(componentDesc.ComponentStub.Source.GitSource.Revision).Should(Equal("testbranch"))
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Create Component Detection Query with context provided", func() {
			It("Should successfully get the devfiles", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "22"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL:     "https://github.com/devfile-resources/multi-components-none",
							Context: "devfile-sample-java-springboot-basic",
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
					Expect(devfileName).Should(ContainSubstring("java-springboot"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(ContainSubstring("devfile-sample-java-springboot-basic"))
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Create Component Detection Query with context set to \"./\"", func() {
			It("Should successfully detect a devfile", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "23"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL:     SampleRepoLink,
							Context: "./",
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

		Context("Create Component Detection Query with context set to a deeper level folder", func() {
			It("Should successfully detect a devfile", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "24"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL:     "https://github.com/devfile-resources/multi-components-deep",
							Context: "python/devfile-sample-python-basic",
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
					Expect(devfileName).Should(ContainSubstring("python-basic"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(ContainSubstring("python/devfile-sample-python-basic"))
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Create Component Detection Query with Devfile URL that has no kubernetes definition", func() {
			It("Should err out", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "25"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL:        SampleRepoLink,
							DevfileURL: "https://raw.githubusercontent.com/devfile-resources/multi-components-with-no-kubecomps/main/devfile-sample-java-springboot-basic/.devfile/.devfile.yaml",
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
				Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("does not contain a valid outerloop definition"))

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Create Component Detection Query with repo that has devfile but no Dockerfile", func() {
			It("Should successfully detect a devfile and match the proper Dockerfile for it", func() {
				ctx := context.Background()

				queryName := "nodejs-no-dockerfile"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL: "https://github.com/devfile-samples/node-express-hello-devfile-no-dockerfile",
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

				// Make sure the a devfile is detected
				Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))

				for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
					Expect(devfileName).Should(ContainSubstring("node"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/devfile.yaml"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/Dockerfile"))
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Create Component Detection Query for spring boot repository with multiple components detected, one at root", func() {
			It("Should only return one component, a spring boot devfile", func() {
				ctx := context.Background()

				queryName := "spring-boot-root-component"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL:      "https://github.com/devfile-resources/todo-spring-boot/",
							Revision: "main",
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
					Expect(devfileDesc.ComponentStub.Source.GitSource.Revision).Should(ContainSubstring("main"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"))
					Expect(devfileDesc.DevfileFound).Should(BeTrue())
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Create Component Detection Query for nodejs repository with detectable port", func() {
			It("Should only return one component, with target port set", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "devfile-sample-nodejs-basic" + "26"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL:      "https://github.com/devfile-resources/single-component-port-detected",
							Revision: "main",
							Context:  "nodejs",
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
					Expect(devfileName).Should(ContainSubstring("nodejs"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(ContainSubstring("nodejs"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.Revision).Should(ContainSubstring("main"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/devfile.yaml"))
					Expect(devfileDesc.ComponentStub.TargetPort).Should(Equal(8080))
					Expect(devfileDesc.DevfileFound).Should(BeTrue())
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Create Component Detection Query for Dockerfile component", func() {
			It("Should only return one component, with target port set", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "dockerfile-node-sample" + "26"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL: "https://github.com/devfile-resources/node-sample-dockerfile",
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
					Expect(devfileName).Should(ContainSubstring("node-sample"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("Dockerfile"))
					Expect(devfileDesc.ComponentStub.TargetPort).Should(Equal(5005))
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Create Component Detection Query with Dockerfile under other common locations", func() {
			It("Should return Dockerfile under common sub folder", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "27"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL: "https://github.com/devfile-resources/multi-component-dockerfile-deep",
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

				// Make sure the right branch is set
				for _, component := range createdHasCompDetectionQuery.Status.ComponentDetected {
					Expect(component.ComponentStub.Source.GitSource.Revision).Should(Equal("main"))
				}

				// Make sure the components are detected
				Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(3)) // mocked, not accurate. check unit test for accurate detection that uses the alizer client instead of the mock client.
				for _, componentDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
					Expect(componentDesc.ComponentStub.Source.GitSource).ShouldNot(BeNil())
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Create Component Detection Query with multicomponent repo with local Dockerfiles", func() {
			It("Should return successfully and properly set the DockerfileURL", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "quality-dashboard" + "28"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL: "https://github.com/devfile-resources/quality-dashboard",
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

				// Make sure the a devfile is detected
				Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(2))

				for dockerFileName, dockerFileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
					Expect(dockerFileName).Should(Or(ContainSubstring("backend-quality-dashboard"), ContainSubstring("frontend-quality-dashboard")))
					Expect(dockerFileDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("Dockerfile"))
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Create Component Detection Query with multi comp repo with no devfiles and context path set", func() {
			It("Should successfully get the correct devfiles and Dockerfiles", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "multicontext" + "29"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL:     "https://github.com/devfile-resources/multi-components-none-path",
							Context: "context",
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

				// Make sure the a devfile is detected
				Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(2))

				for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
					Expect(devfileName).Should(Or(ContainSubstring("java-springboot"), ContainSubstring("python")))
					Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(BeElementOf([]string{"context/devfile-sample-java-springboot-basic", "context/devfile-sample-python-basic"}))
					if strings.Contains(devfileName, "java-springboot") {
						Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"))
						Expect(devfileDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile"))
					} else if strings.Contains(devfileName, "python") {
						Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml"))
						Expect(devfileDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile"))
					}
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Create Component Detection Query with repo that cannot find and dockerfile or devfile match", func() {
			It("Should successfully return the CDQ", func() {
				ctx := context.Background()

				queryName := "repo-no-dockerfile-or-devfile"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL: "https://github.com/devfile-resources/empty",
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

				// Make sure the right status is set
				Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery has successfully finished, no components detected"))

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Create Component Detection Query for invalid outerloop devfile but valid Dockerfile component", func() {
			It("Should return CDQ success status", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "no-outerloop-with-dockerfile"

				hasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ComponentDetectionQuery",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						GitSource: appstudiov1alpha1.GitSource{
							URL: "https://github.com/devfile-resources/no-outerloop-python",
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
					Expect(devfileName).Should(ContainSubstring("python"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("Dockerfile"))
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Create Component Detection Query with public git repo + invalid mock token", func() {
			It("Should successfully detect the component from the repo", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "30"

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
						"password": "invalid-mock-token",
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
						Annotations: map[string]string{
							"runCDQAnalysisLocal": "true",
						},
					},
					Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
						Secret: queryName,
						GitSource: appstudiov1alpha1.GitSource{
							URL: SampleRepoLink, // using an actual public repo here for testing a invalid token case
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
				Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("failed to retrieve https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"))

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		// test CDQ job
		springDevfileContext := `
schemaVersion: 2.2.0
metadata:
  name: java-springboot
  version: 1.2.1
  projectType: springboot
  provider: Red Hat
  language: Java
`

		pythonDevfileContext := `
schemaVersion: 2.2.0
metadata:
  name: python
  version: 1.0.1
  projectType: Python
  provider: Red Hat
  language: Python
`

		nodeJSDevfileContext := `
schemaVersion: 2.2.0
metadata:
  name: nodejs
  version: 2.1.1
  projectType: Node.js
  provider: Red Hat
  language: JavaScript
`

		Context("Run CDQ Job - Create Component Detection Query with URL set", func() {
			It("Should successfully detect a devfile", func() {
				ctx := context.Background()
				beforeImportGitRepoTotalReqs := testutil.ToFloat64(metrics.ImportGitRepoTotalReqs)
				beforeImportGitRepoSucceeded := testutil.ToFloat64(metrics.ImportGitRepoSucceeded)
				beforeImportGitRepoFailed := testutil.ToFloat64(metrics.ImportGitRepoFailed)

				queryName := HASCompDetQuery + "-job1"

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
							URL:      SampleRepoLink,
							Revision: "main",
						},
					},
				}

				Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

				configMapBinaryData := make(map[string][]byte)
				devfilesMap := make(map[string][]byte)
				devfilesURLMap := make(map[string]string)
				dockerfileContextMap := make(map[string]string)
				devfilesURLMap["./"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"
				dockerfileContextMap["./"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile"
				devfilesMap["./"] = []byte(springDevfileContext)
				devfilesMapbytes, _ := json.Marshal(devfilesMap)
				devfilesURLMapbytes, _ := json.Marshal(devfilesURLMap)
				dockerfileContextMapbytes, _ := json.Marshal(dockerfileContextMap)

				configMapBinaryData["devfilesMap"] = devfilesMapbytes
				configMapBinaryData["devfilesURLMap"] = devfilesURLMapbytes
				configMapBinaryData["dockerfileContextMap"] = dockerfileContextMapbytes
				cdqConfigMap := corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
					},
					BinaryData: configMapBinaryData,
				}
				Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
				createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
				createdJob := &batchv1.Job{}
				createdConfigMap := &corev1.ConfigMap{}

				// The job won't be actually completed, as the container image won't be pulled
				// check for the object to ensure the job has been created
				Eventually(func() bool {
					k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
					return createdJob != nil
				}, timeout, interval).Should(BeTrue())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
					return len(createdConfigMap.BinaryData) > 1
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
					return len(createdHasCompDetectionQuery.Status.Conditions) > 1
				}, timeout, interval).Should(BeTrue())

				// Make sure the a devfile is detected
				Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))

				for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
					Expect(devfileName).Should(ContainSubstring("spring"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(ContainSubstring("./"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.Revision).Should(ContainSubstring("main"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"))
					Expect(devfileDesc.DevfileFound).Should(BeTrue())
				}

				Expect(testutil.ToFloat64(metrics.ImportGitRepoTotalReqs) > beforeImportGitRepoTotalReqs).To(BeTrue())
				Expect(testutil.ToFloat64(metrics.ImportGitRepoSucceeded) > beforeImportGitRepoSucceeded).To(BeTrue())
				Expect(testutil.ToFloat64(metrics.ImportGitRepoFailed) == beforeImportGitRepoFailed).To(BeTrue())
				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Run CDQ Job - Create Component Detection Query with multi comp repo with no devfiles", func() {
			It("Should successfully get the correct devfiles and dockerfiles", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "-job2"

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
							URL: "https://github.com/devfile-resources/multi-components-none",
						},
					},
				}

				Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

				configMapBinaryData := make(map[string][]byte)
				devfilesMap := make(map[string][]byte)
				devfilesURLMap := make(map[string]string)
				dockerfileContextMap := make(map[string]string)

				devfilesURLMap["devfile-sample-java-springboot-basic"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"
				devfilesURLMap["devfile-sample-python-basic"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml"
				dockerfileContextMap["devfile-sample-java-springboot-basic"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile"
				dockerfileContextMap["devfile-sample-python-basic"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile"
				devfilesMap["devfile-sample-java-springboot-basic"] = []byte(springDevfileContext)
				devfilesMap["devfile-sample-python-basic"] = []byte(pythonDevfileContext)

				devfilesMapbytes, _ := json.Marshal(devfilesMap)
				devfilesURLMapbytes, _ := json.Marshal(devfilesURLMap)
				dockerfileContextMapbytes, _ := json.Marshal(dockerfileContextMap)

				configMapBinaryData["devfilesMap"] = devfilesMapbytes
				configMapBinaryData["devfilesURLMap"] = devfilesURLMapbytes
				configMapBinaryData["dockerfileContextMap"] = dockerfileContextMapbytes
				cdqConfigMap := corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
					},
					BinaryData: configMapBinaryData,
				}
				Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
				createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
				createdJob := &batchv1.Job{}
				createdConfigMap := &corev1.ConfigMap{}
				// The job won't be actually completed, as the container image won't be pulled
				// check for the object to ensure the job has been created
				Eventually(func() bool {
					k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
					return createdJob != nil
				}, timeout, interval).Should(BeTrue())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
					return len(createdConfigMap.BinaryData) > 1
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
					return len(createdHasCompDetectionQuery.Status.Conditions) > 1
				}, timeout, interval).Should(BeTrue())
				// Make sure the a devfile is detected
				Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(2))

				for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
					Expect(devfileName).Should(Or(ContainSubstring("java-springboot"), ContainSubstring("python")))
					Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(BeElementOf([]string{"devfile-sample-java-springboot-basic", "devfile-sample-python-basic"}))
					if strings.Contains(devfileName, "java-springboot") {
						Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"))
						Expect(devfileDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile"))
					} else if strings.Contains(devfileName, "python") {
						Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml"))
						Expect(devfileDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile"))
					}
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Run CDQ Job - Create Component Detection Query with a non component devfile repo", func() {
			It("Should complete without any devfile detected", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "-job3"

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

				configMapBinaryData := make(map[string][]byte)
				devfilesMap := make(map[string][]byte)
				devfilesURLMap := make(map[string]string)
				dockerfileContextMap := make(map[string]string)

				devfilesMapbytes, _ := json.Marshal(devfilesMap)
				devfilesURLMapbytes, _ := json.Marshal(devfilesURLMap)
				dockerfileContextMapbytes, _ := json.Marshal(dockerfileContextMap)

				configMapBinaryData["devfilesMap"] = devfilesMapbytes
				configMapBinaryData["devfilesURLMap"] = devfilesURLMapbytes
				configMapBinaryData["dockerfileContextMap"] = dockerfileContextMapbytes
				cdqConfigMap := corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
					},
					BinaryData: configMapBinaryData,
				}
				Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
				createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
				createdJob := &batchv1.Job{}
				createdConfigMap := &corev1.ConfigMap{}
				// The job won't be actually completed, as the container image won't be pulled
				// check for the object to ensure the job has been created
				Eventually(func() bool {
					k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
					return createdJob != nil
				}, timeout, interval).Should(BeTrue())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
					return len(createdConfigMap.BinaryData) > 1
				}, timeout, interval).Should(BeTrue())
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

		Context("Run CDQ Job - Create Component Detection Query with multi component repo that has no devfile", func() {
			It("Should match a devfile with alizer if it can be a component", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "-job4"

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
							URL: "https://github.com/devfile-resources/multi-components-none",
						},
					},
				}

				Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

				configMapBinaryData := make(map[string][]byte)
				devfilesMap := make(map[string][]byte)
				devfilesURLMap := make(map[string]string)
				dockerfileContextMap := make(map[string]string)

				devfilesURLMap["devfile-sample-java-springboot-basic"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"
				devfilesURLMap["devfile-sample-python-basic"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/devfile.yaml"
				dockerfileContextMap["devfile-sample-java-springboot-basic"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile"
				dockerfileContextMap["devfile-sample-python-basic"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile"
				devfilesMap["devfile-sample-java-springboot-basic"] = []byte(springDevfileContext)
				devfilesMap["devfile-sample-python-basic"] = []byte(pythonDevfileContext)

				devfilesMapbytes, _ := json.Marshal(devfilesMap)
				devfilesURLMapbytes, _ := json.Marshal(devfilesURLMap)
				dockerfileContextMapbytes, _ := json.Marshal(dockerfileContextMap)

				configMapBinaryData["devfilesMap"] = devfilesMapbytes
				configMapBinaryData["devfilesURLMap"] = devfilesURLMapbytes
				configMapBinaryData["dockerfileContextMap"] = dockerfileContextMapbytes
				cdqConfigMap := corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
					},
					BinaryData: configMapBinaryData,
				}
				Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
				createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
				createdJob := &batchv1.Job{}
				createdConfigMap := &corev1.ConfigMap{}
				// The job won't be actually completed, as the container image won't be pulled
				// check for the object to ensure the job has been created
				Eventually(func() bool {
					k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
					return createdJob != nil
				}, timeout, interval).Should(BeTrue())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
					return len(createdConfigMap.BinaryData) > 1
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
					return len(createdHasCompDetectionQuery.Status.Conditions) > 1
				}, timeout, interval).Should(BeTrue())

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

		Context("Run CDQ Job - Create Component Detection Query with repo that has no devfile", func() {
			It("Should match a devfile with alizer if it can be a component", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "-job5"

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
							URL: "https://github.com/devfile-resources/devfile-sample-java-springboot-basic-no-devfile",
						},
					},
				}

				Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

				configMapBinaryData := make(map[string][]byte)
				devfilesMap := make(map[string][]byte)
				devfilesURLMap := make(map[string]string)
				dockerfileContextMap := make(map[string]string)

				devfilesURLMap["./"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"
				dockerfileContextMap["./"] = "https://raw.githubusercontent.com/devfile-resources/devfile-sample-java-springboot-basic-no-devfile/main/docker/Dockerfile"
				devfilesMap["./"] = []byte(springDevfileContext)

				devfilesMapbytes, _ := json.Marshal(devfilesMap)
				devfilesURLMapbytes, _ := json.Marshal(devfilesURLMap)
				dockerfileContextMapbytes, _ := json.Marshal(dockerfileContextMap)

				configMapBinaryData["devfilesMap"] = devfilesMapbytes
				configMapBinaryData["devfilesURLMap"] = devfilesURLMapbytes
				configMapBinaryData["dockerfileContextMap"] = dockerfileContextMapbytes
				cdqConfigMap := corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
					},
					BinaryData: configMapBinaryData,
				}
				Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
				createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
				createdJob := &batchv1.Job{}
				createdConfigMap := &corev1.ConfigMap{}
				// The job won't be actually completed, as the container image won't be pulled
				// check for the object to ensure the job has been created
				Eventually(func() bool {
					k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
					return createdJob != nil
				}, timeout, interval).Should(BeTrue())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
					return len(createdConfigMap.BinaryData) > 1
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
					return len(createdHasCompDetectionQuery.Status.Conditions) > 1
				}, timeout, interval).Should(BeTrue())

				// Make sure the right err is set
				Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery has successfully finished"))

				// Make sure the a devfile is detected
				Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))

				for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
					Expect([]string{devfileName}).Should(ContainElement(ContainSubstring("devfile-sample-java-springboot-basic-no-devfile")))
					Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(BeElementOf([]string{"./"}))
					Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-resources/devfile-sample-java-springboot-basic-no-devfile/main/docker/Dockerfile"))
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Run CDQ Job - Create Component Detection Query with private multicomponent Github repo", func() {
			It("Should err out due to authentication required error", func() {
				ctx := context.Background()

				beforeImportGitRepoTotalReqs := testutil.ToFloat64(metrics.ImportGitRepoTotalReqs)
				beforeImportGitRepoSucceeded := testutil.ToFloat64(metrics.ImportGitRepoSucceeded)
				beforeImportGitRepoFailed := testutil.ToFloat64(metrics.ImportGitRepoFailed)

				queryName := HASCompDetQuery + "-job6"

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
							URL: "https://github.com/devfile-resources/private-repo-test",
						},
					},
				}

				Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

				configMapBinaryData := make(map[string][]byte)
				errorMap := make(map[string]string)
				errorMap["InternalError"] = "some internal system error"

				errorMapbytes, _ := json.Marshal(errorMap)
				configMapBinaryData["errorMap"] = errorMapbytes

				cdqConfigMap := corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
					},
					BinaryData: configMapBinaryData,
				}
				Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())
				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
				createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
				createdJob := &batchv1.Job{}
				createdConfigMap := &corev1.ConfigMap{}
				// The job won't be actually completed, as the container image won't be pulled
				// check for the object to ensure the job has been created
				Eventually(func() bool {
					k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
					return createdJob != nil
				}, timeout, interval).Should(BeTrue())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
					return len(createdConfigMap.BinaryData) > 0
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
					return len(createdHasCompDetectionQuery.Status.Conditions) > 1
				}, timeout, interval).Should(BeTrue())

				// Make sure the right err is set
				Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("internal error: some internal system error"))

				Expect(testutil.ToFloat64(metrics.ImportGitRepoTotalReqs) > beforeImportGitRepoTotalReqs).To(BeTrue())
				Expect(testutil.ToFloat64(metrics.ImportGitRepoSucceeded) == beforeImportGitRepoSucceeded).To(BeTrue())
				Expect(testutil.ToFloat64(metrics.ImportGitRepoFailed) > beforeImportGitRepoFailed).To(BeTrue())

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		// Private repo tests
		Context("Run CDQ Job - Create Component Detection Query with private git repo + invalid token", func() {
			It("Should err out due to invalid token", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "-job7"

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
							URL: "https://github.com/devfile-resources/private-repo-test",
						},
					},
				}

				Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

				configMapBinaryData := make(map[string][]byte)
				errorMap := make(map[string]string)
				errorMap["InternalError"] = "failed to clone the repo"

				errorMapbytes, _ := json.Marshal(errorMap)
				configMapBinaryData["errorMap"] = errorMapbytes

				cdqConfigMap := corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
					},
					BinaryData: configMapBinaryData,
				}
				Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
				createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
				createdJob := &batchv1.Job{}
				createdConfigMap := &corev1.ConfigMap{}
				// The job won't be actually completed, as the container image won't be pulled
				// check for the object to ensure the job has been created
				Eventually(func() bool {
					k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
					return createdJob != nil
				}, timeout, interval).Should(BeTrue())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
					return len(createdConfigMap.BinaryData) > 0
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
					return len(createdHasCompDetectionQuery.Status.Conditions) > 1
				}, timeout, interval).Should(BeTrue())

				// index is 1 because of CDQ status condition Processing
				Expect(createdHasCompDetectionQuery.Status.Conditions[1].Status).Should(Equal(metav1.ConditionFalse))
				Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery failed: internal error: failed to clone the repo"))

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Run CDQ Job - Create Component Detection Query with private git repo + valid mock token", func() {
			It("Should successfully detect the component from the repo", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "-job8"

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
						"password": "valid-mock-token",
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
							URL: SampleRepoLink, // using an actual public repo here for testing a valid token case
						},
					},
				}

				Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

				configMapBinaryData := make(map[string][]byte)
				devfilesMap := make(map[string][]byte)
				devfilesURLMap := make(map[string]string)
				dockerfileContextMap := make(map[string]string)
				devfilesURLMap["./"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"
				dockerfileContextMap["./"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/docker/Dockerfile"
				devfilesMap["./"] = []byte(springDevfileContext)
				devfilesMapbytes, _ := json.Marshal(devfilesMap)
				devfilesURLMapbytes, _ := json.Marshal(devfilesURLMap)
				dockerfileContextMapbytes, _ := json.Marshal(dockerfileContextMap)

				configMapBinaryData["revision"] = []byte("main")
				configMapBinaryData["devfilesMap"] = devfilesMapbytes
				configMapBinaryData["devfilesURLMap"] = devfilesURLMapbytes
				configMapBinaryData["dockerfileContextMap"] = dockerfileContextMapbytes
				cdqConfigMap := corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
					},
					BinaryData: configMapBinaryData,
				}
				Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
				createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
				createdJob := &batchv1.Job{}
				createdConfigMap := &corev1.ConfigMap{}

				// The job won't be actually completed, as the container image won't be pulled
				// check for the object to ensure the job has been created
				Eventually(func() bool {
					k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
					return createdJob != nil
				}, timeout, interval).Should(BeTrue())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
					return len(createdConfigMap.BinaryData) > 1
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
					return len(createdHasCompDetectionQuery.Status.Conditions) > 1
				}, timeout, interval).Should(BeTrue())

				// Make sure the a devfile is detected
				Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))

				for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
					Expect(devfileName).Should(ContainSubstring("spring"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(ContainSubstring("./"))
					Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/main/devfile.yaml"))
					Expect(devfileDesc.DevfileFound).Should(BeTrue())
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Run CDQ Job - Create Component Detection Query with no secret", func() {
			It("Should error out since specified secret does not exist", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "-job10"

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

				configMapBinaryData := make(map[string][]byte)
				errorMap := make(map[string]string)
				errorMap["InternalError"] = fmt.Sprintf("Secret %q not found", queryName)

				errorMapbytes, _ := json.Marshal(errorMap)
				configMapBinaryData["errorMap"] = errorMapbytes

				cdqConfigMap := corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
					},
					BinaryData: configMapBinaryData,
				}
				Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
				createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
				createdJob := &batchv1.Job{}
				createdConfigMap := &corev1.ConfigMap{}
				// The job won't be actually completed, as the container image won't be pulled
				// check for the object to ensure the job has been created
				Eventually(func() bool {
					k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
					return createdJob != nil
				}, timeout, interval).Should(BeTrue())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
					return len(createdConfigMap.BinaryData) > 0
				}, timeout, interval).Should(BeTrue())
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

		Context("Run CDQ Job - Create Component Detection Query with URL set to repo with invalid devfile", func() {
			It("Should detect a devfile but return an error", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "-job11"

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
							URL: "https://github.com/devfile-resources/test-bad-devfile",
						},
					},
				}

				Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

				configMapBinaryData := make(map[string][]byte)
				errorMap := make(map[string]string)
				errorMap["InternalError"] = "failed to decode devfile json: json: cannot unmarshal string into Go value of type map[string]"

				errorMapbytes, _ := json.Marshal(errorMap)
				configMapBinaryData["errorMap"] = errorMapbytes

				cdqConfigMap := corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
					},
					BinaryData: configMapBinaryData,
				}
				Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
				createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
				createdJob := &batchv1.Job{}
				createdConfigMap := &corev1.ConfigMap{}
				// The job won't be actually completed, as the container image won't be pulled
				// check for the object to ensure the job has been created
				Eventually(func() bool {
					k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
					return createdJob != nil
				}, timeout, interval).Should(BeTrue())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
					return len(createdConfigMap.BinaryData) > 0
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
					return len(createdHasCompDetectionQuery.Status.Conditions) > 1
				}, timeout, interval).Should(BeTrue())

				// Make sure that the proper error condition is set
				Expect(createdHasCompDetectionQuery.Status.Conditions[1].Status).Should(Equal(metav1.ConditionFalse))
				Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("internal error: failed to decode devfile json: json: cannot unmarshal string into Go value of type map[string]"))
				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})
		})

		Context("Run CDQ Job - A Requeued ComponentDetectionQuery", func() {
			It("Should delete itself", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "-job12"

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

				configMapBinaryData := make(map[string][]byte)
				errorMap := make(map[string]string)
				errorMap["AuthenticationFailed"] = "authentication failed"

				errorMapbytes, _ := json.Marshal(errorMap)
				configMapBinaryData["errorMap"] = errorMapbytes

				cdqConfigMap := corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
					},
					BinaryData: configMapBinaryData,
				}
				Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
				createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
				createdJob := &batchv1.Job{}
				createdConfigMap := &corev1.ConfigMap{}
				// The job won't be actually completed, as the container image won't be pulled
				// check for the object to ensure the job has been created
				Eventually(func() bool {
					k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
					return createdJob != nil
				}, timeout, interval).Should(BeTrue())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
					return len(createdConfigMap.BinaryData) > 0
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
					return len(createdHasCompDetectionQuery.Status.Conditions) > 1
				}, timeout, interval).Should(BeTrue())

				// Make sure no component was detected
				Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(0))

				// Make sure the right err is set
				Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("authentication failed"))

				// Trigger a requeue by updating the resource
				createdHasCompDetectionQuery.Spec.GitSource.URL = SampleRepoLink
				Expect(k8sClient.Update(ctx, createdHasCompDetectionQuery)).Should(Succeed())

				// Validate that the resource has been deleted
				hasCompDetQueryLookupKey = types.NamespacedName{Name: queryName, Namespace: HASNamespace}
				deletedCompDetQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
				deletedCDQJob := &batchv1.Job{}
				deletedConfigMap := &corev1.ConfigMap{}
				Eventually(func() bool {
					err := k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, deletedCompDetQuery)
					return err != nil
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, deletedCDQJob)
					return err != nil
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, deletedConfigMap)
					return err != nil
				}, timeout, interval).Should(BeTrue())
			})
		})

		Context("Run CDQ Job - Create Component Detection Query with multi component repo that has no devfile or dockerfile", func() {
			It("Should attempt to match a devfile or dockerfile for every component", func() {
				ctx := context.Background()

				queryName := HASCompDetQuery + "-job13"

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
							URL: "https://github.com/devfile-resources/multi-components-dockerfile",
						},
					},
				}

				Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

				configMapBinaryData := make(map[string][]byte)
				devfilesMap := make(map[string][]byte)
				devfilesURLMap := make(map[string]string)
				dockerfileContextMap := make(map[string]string)

				devfilesURLMap["devfile-sample-java-springboot-basic"] = "https://raw.githubusercontent.com/devfile-resources/multi-components-dockerfile/main/devfile-sample-java-springboot-basic/devfile.yaml"
				dockerfileContextMap["devfile-sample-java-springboot-basic"] = "https://raw.githubusercontent.com/devfile-resources/multi-components-dockerfile/main/devfile-sample-java-springboot-basic/docker/Dockerfile"
				devfilesMap["devfile-sample-java-springboot-basic"] = []byte(springDevfileContext)

				devfilesURLMap["devfile-sample-nodejs-basic"] = "https://raw.githubusercontent.com/devfile-resources/multi-components-dockerfile/main/devfile-sample-nodejs-basic/devfile.yaml"
				dockerfileContextMap["devfile-sample-nodejs-basic"] = "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/Dockerfile"
				devfilesMap["devfile-sample-nodejs-basic"] = []byte(nodeJSDevfileContext)

				devfilesURLMap["devfile-sample-python-basic"] = "https://raw.githubusercontent.com/devfile-resources/multi-components-dockerfile/main/devfile-sample-python-basic/devfile.yaml"
				dockerfileContextMap["devfile-sample-python-basic"] = "https://raw.githubusercontent.com/devfile-resources/multi-components-dockerfile/main/devfile-sample-python-basic/Dockerfile"
				devfilesMap["devfile-sample-python-basic"] = []byte(pythonDevfileContext)

				devfilesURLMap["python-src-none"] = "https://registry.devfile.io/devfiles/python-basic"
				dockerfileContextMap["python-src-none"] = "https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker/Dockerfile"
				devfilesMap["python-src-none"] = []byte(pythonDevfileContext)

				dockerfileContextMap["python-src-docker"] = "https://raw.githubusercontent.com/devfile-resources/multi-components-dockerfile/main/python-src-docker/Dockerfile"

				devfilesMapbytes, _ := json.Marshal(devfilesMap)
				devfilesURLMapbytes, _ := json.Marshal(devfilesURLMap)
				dockerfileContextMapbytes, _ := json.Marshal(dockerfileContextMap)

				configMapBinaryData["devfilesMap"] = devfilesMapbytes
				configMapBinaryData["devfilesURLMap"] = devfilesURLMapbytes
				configMapBinaryData["dockerfileContextMap"] = dockerfileContextMapbytes
				cdqConfigMap := corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: HASNamespace,
					},
					BinaryData: configMapBinaryData,
				}
				Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
				createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
				createdJob := &batchv1.Job{}
				createdConfigMap := &corev1.ConfigMap{}
				// The job won't be actually completed, as the container image won't be pulled
				// check for the object to ensure the job has been created
				Eventually(func() bool {
					k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
					return createdJob != nil
				}, timeout, interval).Should(BeTrue())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
					return len(createdConfigMap.BinaryData) > 1
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
					return len(createdHasCompDetectionQuery.Status.Conditions) > 1
				}, timeout20s, interval).Should(BeTrue())

				// Make sure the right status is set
				Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery has successfully finished"))

				// Make sure the devfiles are detected
				Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(5)) // mocked, not accurate. check unit test for accurate detection that uses the alizer client instead of the mock client.
				for _, componentDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
					Expect(componentDesc.ComponentStub.Source.GitSource).ShouldNot(BeNil())
				}

				// Delete the specified Detection Query resource
				deleteCompDetQueryCR(hasCompDetQueryLookupKey)
			})

			Context("Run CDQ Job - Create Component Detection Query with a dockerfile repo", func() {
				It("Should return successfully", func() {
					ctx := context.Background()

					queryName := HASCompDetQuery + "-job14"

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
								URL: "https://github.com/devfile-resources/python-src-docker",
							},
						},
					}

					Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

					configMapBinaryData := make(map[string][]byte)
					dockerfileContextMap := make(map[string]string)

					dockerfileContextMap["./"] = "https://raw.githubusercontent.com/devfile-resources/python-src-docker/main/Dockerfile"

					dockerfileContextMapbytes, _ := json.Marshal(dockerfileContextMap)

					configMapBinaryData["dockerfileContextMap"] = dockerfileContextMapbytes
					cdqConfigMap := corev1.ConfigMap{
						TypeMeta: metav1.TypeMeta{
							Kind:       "ConfigMap",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      queryName,
							Namespace: HASNamespace,
						},
						BinaryData: configMapBinaryData,
					}
					Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

					// Look up the has app resource that was created.
					// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
					hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
					createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
					createdJob := &batchv1.Job{}
					createdConfigMap := &corev1.ConfigMap{}
					// The job won't be actually completed, as the container image won't be pulled
					// check for the object to ensure the job has been created
					Eventually(func() bool {
						k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
						return createdJob != nil
					}, timeout, interval).Should(BeTrue())

					// Look up the has app resource that was created.
					// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
					Eventually(func() bool {
						k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
						return len(createdConfigMap.BinaryData) > 0
					}, timeout, interval).Should(BeTrue())
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
						Expect(componentDesc.ComponentStub.Source.GitSource.DockerfileURL).Should(Equal("https://raw.githubusercontent.com/devfile-resources/python-src-docker/main/Dockerfile"))
					}

					// Delete the specified Detection Query resource
					deleteCompDetQueryCR(hasCompDetQueryLookupKey)
				})
			})

			Context("Run CDQ Job - Create Component Detection Query with context provided", func() {
				It("Should successfully get the devfiles", func() {
					ctx := context.Background()

					queryName := HASCompDetQuery + "-job15"

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
								URL:     "https://github.com/devfile-resources/multi-components-none",
								Context: "devfile-sample-java-springboot-basic",
							},
						},
					}

					Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

					configMapBinaryData := make(map[string][]byte)
					devfilesMap := make(map[string][]byte)
					devfilesURLMap := make(map[string]string)
					dockerfileContextMap := make(map[string]string)

					devfilesURLMap["devfile-sample-java-springboot-basic"] = "https://raw.githubusercontent.com/devfile-resources/multi-components-dockerfile/main/devfile-sample-java-springboot-basic/devfile.yaml"
					dockerfileContextMap["devfile-sample-java-springboot-basic"] = "https://raw.githubusercontent.com/devfile-resources/multi-components-dockerfile/main/devfile-sample-java-springboot-basic/docker/Dockerfile"
					devfilesMap["devfile-sample-java-springboot-basic"] = []byte(springDevfileContext)

					devfilesMapbytes, _ := json.Marshal(devfilesMap)
					devfilesURLMapbytes, _ := json.Marshal(devfilesURLMap)
					dockerfileContextMapbytes, _ := json.Marshal(dockerfileContextMap)

					configMapBinaryData["devfilesMap"] = devfilesMapbytes
					configMapBinaryData["devfilesURLMap"] = devfilesURLMapbytes
					configMapBinaryData["dockerfileContextMap"] = dockerfileContextMapbytes

					cdqConfigMap := corev1.ConfigMap{
						TypeMeta: metav1.TypeMeta{
							Kind:       "ConfigMap",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      queryName,
							Namespace: HASNamespace,
						},
						BinaryData: configMapBinaryData,
					}
					Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

					// Look up the has app resource that was created.
					// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
					hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
					createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
					createdJob := &batchv1.Job{}
					createdConfigMap := &corev1.ConfigMap{}
					// The job won't be actually completed, as the container image won't be pulled
					// check for the object to ensure the job has been created
					Eventually(func() bool {
						k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
						return createdJob != nil
					}, timeout, interval).Should(BeTrue())

					// Look up the has app resource that was created.
					// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
					Eventually(func() bool {
						k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
						return len(createdConfigMap.BinaryData) > 1
					}, timeout, interval).Should(BeTrue())
					Eventually(func() bool {
						k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
						return len(createdHasCompDetectionQuery.Status.Conditions) > 1
					}, timeout, interval).Should(BeTrue())

					// Make sure the a devfile is detected
					Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))

					for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
						Expect(devfileName).Should(ContainSubstring("java-springboot"))
						Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(ContainSubstring("devfile-sample-java-springboot-basic"))
					}

					// Delete the specified Detection Query resource
					deleteCompDetQueryCR(hasCompDetQueryLookupKey)
				})
			})

			Context("Run CDQ Job - Create Component Detection Query for nodejs repository with detectable port", func() {
				It("Should only return one component, with target port set", func() {
					ctx := context.Background()

					queryName := HASCompDetQuery + "-job16"

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
								URL:      "https://github.com/devfile-resources/single-component-port-detected",
								Revision: "main",
								Context:  "nodejs",
							},
						},
					}

					Expect(k8sClient.Create(ctx, hasCompDetectionQuery)).Should(Succeed())

					configMapBinaryData := make(map[string][]byte)
					devfilesMap := make(map[string][]byte)
					devfilesURLMap := make(map[string]string)
					dockerfileContextMap := make(map[string]string)
					componentPortsMap := make(map[string][]int)

					devfilesURLMap["./"] = "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/devfile.yaml"
					dockerfileContextMap["./"] = "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/docker/Dockerfile"
					devfilesMap["./"] = []byte(nodeJSDevfileContext)
					componentPortsMap["./"] = []int{8080}

					devfilesMapbytes, _ := json.Marshal(devfilesMap)
					devfilesURLMapbytes, _ := json.Marshal(devfilesURLMap)
					dockerfileContextMapbytes, _ := json.Marshal(dockerfileContextMap)
					componentPortsMapbytes, _ := json.Marshal(componentPortsMap)

					configMapBinaryData["devfilesMap"] = devfilesMapbytes
					configMapBinaryData["devfilesURLMap"] = devfilesURLMapbytes
					configMapBinaryData["dockerfileContextMap"] = dockerfileContextMapbytes
					configMapBinaryData["componentPortsMap"] = componentPortsMapbytes
					cdqConfigMap := corev1.ConfigMap{
						TypeMeta: metav1.TypeMeta{
							Kind:       "ConfigMap",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      queryName,
							Namespace: HASNamespace,
						},
						BinaryData: configMapBinaryData,
					}
					Expect(k8sClient.Create(ctx, &cdqConfigMap)).Should(Succeed())

					// Look up the has app resource that was created.
					// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
					hasCompDetQueryLookupKey := types.NamespacedName{Name: queryName, Namespace: HASNamespace}
					createdHasCompDetectionQuery := &appstudiov1alpha1.ComponentDetectionQuery{}
					createdJob := &batchv1.Job{}
					createdConfigMap := &corev1.ConfigMap{}
					// The job won't be actually completed, as the container image won't be pulled
					// check for the object to ensure the job has been created
					Eventually(func() bool {
						k8sClient.Get(context.Background(), types.NamespacedName{Name: queryName + "-job", Namespace: HASNamespace}, createdJob)
						return createdJob != nil
					}, timeout, interval).Should(BeTrue())

					// Look up the has app resource that was created.
					// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
					Eventually(func() bool {
						k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdConfigMap)
						return len(createdConfigMap.BinaryData) > 1
					}, timeout, interval).Should(BeTrue())
					Eventually(func() bool {
						k8sClient.Get(context.Background(), hasCompDetQueryLookupKey, createdHasCompDetectionQuery)
						return len(createdHasCompDetectionQuery.Status.Conditions) > 1
					}, timeout, interval).Should(BeTrue())

					// Make sure the right err is set
					Expect(createdHasCompDetectionQuery.Status.Conditions[1].Message).Should(ContainSubstring("ComponentDetectionQuery has successfully finished"))

					// Make sure the a devfile is detected
					Expect(len(createdHasCompDetectionQuery.Status.ComponentDetected)).Should(Equal(1))

					for devfileName, devfileDesc := range createdHasCompDetectionQuery.Status.ComponentDetected {
						Expect(devfileName).Should(ContainSubstring("single-component-port-detected"))
						Expect(devfileDesc.ComponentStub.Source.GitSource.Context).Should(ContainSubstring("./"))
						Expect(devfileDesc.ComponentStub.Source.GitSource.Revision).Should(ContainSubstring("main"))
						Expect(devfileDesc.ComponentStub.Source.GitSource.DevfileURL).Should(Equal("https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/devfile.yaml"))
						Expect(devfileDesc.ComponentStub.TargetPort).Should(Equal(8080))
						Expect(devfileDesc.DevfileFound).Should(BeTrue())
					}

					// Delete the specified Detection Query resource
					deleteCompDetQueryCR(hasCompDetQueryLookupKey)

				})
			})
		})

	})
	Context("Create Component Detection Query with non Github URL", func() {
		It("Should err out", func() {
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
					Annotations: map[string]string{
						"runCDQAnalysisLocal": "true",
					},
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL:      "https://gitlab.com/redhat-appstudio-appdata/sample",
						Revision: "main",
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
