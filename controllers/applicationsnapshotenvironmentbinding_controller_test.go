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
	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("ApplicationSnapshotEnvironmentBinding controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		HASAppName      = "test-app"
		HASCompName     = "test-comp"
		HASSnapshotName = "test-snapshot"
		HASBindingName  = "test-binding"
		HASAppNamespace = "default"
		DisplayName     = "petclinic"
		Description     = "Simple petclinic app"
		ComponentName   = "backend"
		SampleRepoLink  = "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	)

	Context("Create ApplicationSnapshotEnvironmentBinding with component configurations", func() {
		It("Should generate gitops overlays successfully", func() {
			ctx := context.Background()

			applicationName := HASAppName + "1"
			componentName := HASCompName + "1"
			snapshotName := HASSnapshotName + "1"
			bindingName := HASBindingName + "1"
			environmentName := "staging"

			hasGitopsGeneratedResource := map[string]bool{
				"deployment-patch.yaml": true,
			}

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: ComponentName,
					Application:   applicationName,
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
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			appSnapshot := &appstudioshared.ApplicationSnapshot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotSpec{
					Application:        applicationName,
					DisplayName:        "My Snapshot",
					DisplayDescription: "My Snapshot",
					Components: []appstudioshared.ApplicationSnapshotComponent{
						{
							Name:           componentName,
							ContainerImage: "image1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appSnapshot)).Should(Succeed())

			appSnapshotLookupKey := types.NamespacedName{Name: snapshotName, Namespace: HASAppNamespace}
			createdAppSnapshot := &appstudioshared.ApplicationSnapshot{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), appSnapshotLookupKey, createdAppSnapshot)
				return len(createdAppSnapshot.Spec.Components) > 0
			}, timeout, interval).Should(BeTrue())

			appBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: environmentName,
					Snapshot:    snapshotName,
					Components: []appstudioshared.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudioshared.BindingComponentConfiguration{
								Replicas: int(replicas),
								Env: []appstudioshared.EnvVarPair{
									{
										Name:  "FOO",
										Value: "BAR",
									},
								},
								Resources: &corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("1"),
									},
								},
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, appBinding)).Should(Succeed())

			bindingLookupKey := types.NamespacedName{Name: bindingName, Namespace: HASAppNamespace}
			createdBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) > 0 && len(createdBinding.Status.Components) == 1
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.GitOpsRepoConditions[0].Message).Should(Equal("GitOps repository sync successful"))
			Expect(createdBinding.Status.Components[0].Name).Should(Equal(componentName))
			Expect(createdBinding.Status.Components[0].GitOpsRepository.Path).Should(Equal(fmt.Sprintf("components/%s/overlays/%s", componentName, environmentName)))
			Expect(createdBinding.Status.Components[0].GitOpsRepository.URL).Should(Equal(createdHasComp.Status.GitOps.RepositoryURL))

			// check the list of generated gitops resources to make sure we account for every one
			for _, generatedResource := range createdBinding.Status.Components[0].GitOpsRepository.GeneratedResources {
				Expect(hasGitopsGeneratedResource[generatedResource]).Should(BeTrue())
			}

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)

			// Delete the specified snapshot
			deleteSnapshot(appSnapshotLookupKey)

		})
	})

	Context("Create ApplicationSnapshotEnvironmentBinding with a missing component", func() {
		It("Should fail if there is no such component by name", func() {
			ctx := context.Background()

			applicationName := HASAppName + "2"
			componentName := HASCompName + "2"
			componentName2 := HASCompName + "2-2"
			snapshotName := HASSnapshotName + "2"
			bindingName := HASBindingName + "2"

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: ComponentName,
					Application:   applicationName,
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
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			appSnapshot := &appstudioshared.ApplicationSnapshot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotSpec{
					Application:        applicationName,
					DisplayName:        "My Snapshot",
					DisplayDescription: "My Snapshot",
					Components: []appstudioshared.ApplicationSnapshotComponent{
						{
							Name:           componentName,
							ContainerImage: "image1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appSnapshot)).Should(Succeed())

			appSnapshotLookupKey := types.NamespacedName{Name: snapshotName, Namespace: HASAppNamespace}
			createdAppSnapshot := &appstudioshared.ApplicationSnapshot{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), appSnapshotLookupKey, createdAppSnapshot)
				return len(createdAppSnapshot.Spec.Components) > 0
			}, timeout, interval).Should(BeTrue())

			appBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: "staging",
					Snapshot:    snapshotName,
					Components: []appstudioshared.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudioshared.BindingComponentConfiguration{
								Replicas: int(replicas),
								Env: []appstudioshared.EnvVarPair{
									{
										Name:  "FOO",
										Value: "BAR",
									},
								},
								Resources: &corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("1"),
									},
								},
							},
						},
						{
							Name: componentName2,
							Configuration: appstudioshared.BindingComponentConfiguration{
								Replicas: int(replicas),
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, appBinding)).Should(Succeed())

			bindingLookupKey := types.NamespacedName{Name: bindingName, Namespace: HASAppNamespace}
			createdBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) > 0
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.GitOpsRepoConditions[len(createdBinding.Status.GitOpsRepoConditions)-1].Message).Should(ContainSubstring(fmt.Sprintf("%q not found", componentName2)))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)

			// Delete the specified snapshot
			deleteSnapshot(appSnapshotLookupKey)

		})
	})

	Context("Create ApplicationSnapshotEnvironmentBinding with a missing snapshot", func() {
		It("Should fail if there is no such snapshot by name", func() {
			ctx := context.Background()

			applicationName := HASAppName + "3"
			componentName := HASCompName + "3"
			snapshotName := HASSnapshotName + "3"
			bindingName := HASBindingName + "3"

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: ComponentName,
					Application:   applicationName,
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
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			appBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: "staging",
					Snapshot:    snapshotName,
					Components: []appstudioshared.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudioshared.BindingComponentConfiguration{
								Replicas: int(replicas),
								Env: []appstudioshared.EnvVarPair{
									{
										Name:  "FOO",
										Value: "BAR",
									},
								},
								Resources: &corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("1"),
									},
								},
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, appBinding)).Should(Succeed())

			bindingLookupKey := types.NamespacedName{Name: bindingName, Namespace: HASAppNamespace}
			createdBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) > 0
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.GitOpsRepoConditions[len(createdBinding.Status.GitOpsRepoConditions)-1].Message).Should(ContainSubstring(fmt.Sprintf("%q not found", snapshotName)))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)
		})
	})

	Context("Create ApplicationSnapshotEnvironmentBinding and Snapshot referencing a wrong Application", func() {
		It("Should err out when Snapshot doesnt reference the same Application as the Binding", func() {
			ctx := context.Background()

			applicationName := HASAppName + "4"
			applicationName2 := HASAppName + "4-2"
			componentName := HASCompName + "4"
			snapshotName := HASSnapshotName + "4"
			bindingName := HASBindingName + "4"

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: ComponentName,
					Application:   applicationName,
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
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			appSnapshot := &appstudioshared.ApplicationSnapshot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotSpec{
					Application:        applicationName2,
					DisplayName:        "My Snapshot",
					DisplayDescription: "My Snapshot",
					Components: []appstudioshared.ApplicationSnapshotComponent{
						{
							Name:           componentName,
							ContainerImage: "image1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appSnapshot)).Should(Succeed())

			appSnapshotLookupKey := types.NamespacedName{Name: snapshotName, Namespace: HASAppNamespace}
			createdAppSnapshot := &appstudioshared.ApplicationSnapshot{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), appSnapshotLookupKey, createdAppSnapshot)
				return len(createdAppSnapshot.Spec.Components) > 0
			}, timeout, interval).Should(BeTrue())

			appBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: "staging",
					Snapshot:    snapshotName,
					Components: []appstudioshared.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudioshared.BindingComponentConfiguration{
								Replicas: int(replicas),
								Env: []appstudioshared.EnvVarPair{
									{
										Name:  "FOO",
										Value: "BAR",
									},
								},
								Resources: &corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("1"),
									},
								},
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, appBinding)).Should(Succeed())

			bindingLookupKey := types.NamespacedName{Name: bindingName, Namespace: HASAppNamespace}
			createdBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) > 0
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.GitOpsRepoConditions[len(createdBinding.Status.GitOpsRepoConditions)-1].Message).Should(ContainSubstring(fmt.Sprintf("application snapshot %s does not belong to the application %s", snapshotName, applicationName)))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)

			// Delete the specified snapshot
			deleteSnapshot(appSnapshotLookupKey)

		})
	})

	Context("Create ApplicationSnapshotEnvironmentBinding and Component referencing a wrong Application", func() {
		It("Should err out when Component doesnt reference the same Application as the Binding", func() {
			ctx := context.Background()

			applicationName := HASAppName + "5"
			applicationName2 := HASAppName + "5-2"
			componentName := HASCompName + "5"
			componentName2 := HASCompName + "5-2"
			snapshotName := HASSnapshotName + "5"
			bindingName := HASBindingName + "5"

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)
			createAndFetchSimpleApp(applicationName2, HASAppNamespace, DisplayName, Description)

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: ComponentName,
					Application:   applicationName,
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
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			hasComp2 := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName2,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: ComponentName + "2",
					Application:   applicationName2,
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: SampleRepoLink,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasComp2)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey2 := types.NamespacedName{Name: componentName2, Namespace: HASAppNamespace}
			createdHasComp2 := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey2, createdHasComp2)
				return len(createdHasComp2.Status.Conditions) > 0 && createdHasComp2.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp2.Status.Devfile).Should(Not(Equal("")))

			appSnapshot := &appstudioshared.ApplicationSnapshot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotSpec{
					Application:        applicationName,
					DisplayName:        "My Snapshot",
					DisplayDescription: "My Snapshot",
					Components: []appstudioshared.ApplicationSnapshotComponent{
						{
							Name:           componentName,
							ContainerImage: "image1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appSnapshot)).Should(Succeed())

			appSnapshotLookupKey := types.NamespacedName{Name: snapshotName, Namespace: HASAppNamespace}
			createdAppSnapshot := &appstudioshared.ApplicationSnapshot{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), appSnapshotLookupKey, createdAppSnapshot)
				return len(createdAppSnapshot.Spec.Components) > 0
			}, timeout, interval).Should(BeTrue())

			appBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: "staging",
					Snapshot:    snapshotName,
					Components: []appstudioshared.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudioshared.BindingComponentConfiguration{
								Replicas: int(replicas),
								Env: []appstudioshared.EnvVarPair{
									{
										Name:  "FOO",
										Value: "BAR",
									},
								},
								Resources: &corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("1"),
									},
								},
							},
						},
						{
							Name: componentName2,
							Configuration: appstudioshared.BindingComponentConfiguration{
								Replicas: int(replicas),
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, appBinding)).Should(Succeed())

			bindingLookupKey := types.NamespacedName{Name: bindingName, Namespace: HASAppNamespace}
			createdBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) > 0
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.GitOpsRepoConditions[len(createdBinding.Status.GitOpsRepoConditions)-1].Message).Should(ContainSubstring(fmt.Sprintf("component %s does not belong to the application %s", componentName2, applicationName)))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)
			deleteHASCompCR(hasCompLookupKey2)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)
			hasAppLookupKey2 := types.NamespacedName{Name: applicationName2, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey2)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)

			// Delete the specified snapshot
			deleteSnapshot(appSnapshotLookupKey)

		})
	})

	Context("Create ApplicationSnapshotEnvironmentBinding and Snapshot referencing a wrong Application", func() {
		It("Should err out when Snapshot doesnt reference the same Application as the Binding", func() {
			ctx := context.Background()

			applicationName := HASAppName + "6"
			componentName := HASCompName + "6"
			componentName2 := HASCompName + "6-2"
			snapshotName := HASSnapshotName + "6"
			bindingName := HASBindingName + "6"

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: ComponentName,
					Application:   applicationName,
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
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			hasComp2 := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName2,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: ComponentName + "2",
					Application:   applicationName,
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: SampleRepoLink,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasComp2)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey2 := types.NamespacedName{Name: componentName2, Namespace: HASAppNamespace}
			createdHasComp2 := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey2, createdHasComp2)
				return len(createdHasComp2.Status.Conditions) > 0 && createdHasComp2.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp2.Status.Devfile).Should(Not(Equal("")))

			appSnapshot := &appstudioshared.ApplicationSnapshot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotSpec{
					Application:        applicationName,
					DisplayName:        "My Snapshot",
					DisplayDescription: "My Snapshot",
					Components: []appstudioshared.ApplicationSnapshotComponent{
						{
							Name:           componentName,
							ContainerImage: "image1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appSnapshot)).Should(Succeed())

			appSnapshotLookupKey := types.NamespacedName{Name: snapshotName, Namespace: HASAppNamespace}
			createdAppSnapshot := &appstudioshared.ApplicationSnapshot{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), appSnapshotLookupKey, createdAppSnapshot)
				return len(createdAppSnapshot.Spec.Components) > 0
			}, timeout, interval).Should(BeTrue())

			appBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: "staging",
					Snapshot:    snapshotName,
					Components: []appstudioshared.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudioshared.BindingComponentConfiguration{
								Replicas: int(replicas),
								Env: []appstudioshared.EnvVarPair{
									{
										Name:  "FOO",
										Value: "BAR",
									},
								},
								Resources: &corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("1"),
									},
								},
							},
						},
						{
							Name: componentName2,
							Configuration: appstudioshared.BindingComponentConfiguration{
								Replicas: int(replicas),
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, appBinding)).Should(Succeed())

			bindingLookupKey := types.NamespacedName{Name: bindingName, Namespace: HASAppNamespace}
			createdBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) > 0
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.GitOpsRepoConditions[len(createdBinding.Status.GitOpsRepoConditions)-1].Message).Should(ContainSubstring(fmt.Sprintf("application snapshot %s did not reference component %s", snapshotName, componentName2)))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)
			deleteHASCompCR(hasCompLookupKey2)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)

			// Delete the specified snapshot
			deleteSnapshot(appSnapshotLookupKey)

		})
	})

	Context("Update ApplicationSnapshotEnvironmentBinding with component configurations", func() {
		It("Should generate gitops overlays successfully", func() {
			ctx := context.Background()

			applicationName := HASAppName + "7"
			componentName := HASCompName + "7"
			snapshotName := HASSnapshotName + "7"
			bindingName := HASBindingName + "7"

			replicas := int32(3)
			newReplicas := int32(4)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: ComponentName,
					Application:   applicationName,
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
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			appSnapshot := &appstudioshared.ApplicationSnapshot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotSpec{
					Application:        applicationName,
					DisplayName:        "My Snapshot",
					DisplayDescription: "My Snapshot",
					Components: []appstudioshared.ApplicationSnapshotComponent{
						{
							Name:           componentName,
							ContainerImage: "image1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appSnapshot)).Should(Succeed())

			appSnapshotLookupKey := types.NamespacedName{Name: snapshotName, Namespace: HASAppNamespace}
			createdAppSnapshot := &appstudioshared.ApplicationSnapshot{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), appSnapshotLookupKey, createdAppSnapshot)
				return len(createdAppSnapshot.Spec.Components) > 0
			}, timeout, interval).Should(BeTrue())

			appBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: "staging",
					Snapshot:    snapshotName,
					Components: []appstudioshared.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudioshared.BindingComponentConfiguration{
								Replicas: int(replicas),
								Env: []appstudioshared.EnvVarPair{
									{
										Name:  "FOO",
										Value: "BAR",
									},
								},
								Resources: &corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("1"),
									},
								},
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, appBinding)).Should(Succeed())

			bindingLookupKey := types.NamespacedName{Name: bindingName, Namespace: HASAppNamespace}
			createdBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) == 1 && len(createdBinding.Status.Components) == 1
			}, timeout, interval).Should(BeTrue())

			createdBinding.Spec.Components[0].Configuration.Replicas = int(newReplicas)

			Expect(k8sClient.Update(ctx, createdBinding)).Should(Succeed())

			// Get the updated resource
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				// Return true if the most recent condition on the CR is updated
				return len(createdBinding.Status.GitOpsRepoConditions) == 1
			}, timeout, interval).Should(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)

			// Delete the specified snapshot
			deleteSnapshot(appSnapshotLookupKey)

		})
	})

	Context("Create ApplicationSnapshotEnvironmentBinding with bad Component GitOps URL", func() {
		It("Should err out", func() {
			ctx := context.Background()

			applicationName := HASAppName + "8"
			componentName := HASCompName + "8"
			snapshotName := HASSnapshotName + "8"
			bindingName := HASBindingName + "8"
			environmentName := "staging"

			replicas := int32(3)

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
						URL: "http://foo.com/?foo\nbar",
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

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: ComponentName,
					Application:   applicationName,
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
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			appSnapshot := &appstudioshared.ApplicationSnapshot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotSpec{
					Application:        applicationName,
					DisplayName:        "My Snapshot",
					DisplayDescription: "My Snapshot",
					Components: []appstudioshared.ApplicationSnapshotComponent{
						{
							Name:           componentName,
							ContainerImage: "image1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appSnapshot)).Should(Succeed())

			appSnapshotLookupKey := types.NamespacedName{Name: snapshotName, Namespace: HASAppNamespace}
			createdAppSnapshot := &appstudioshared.ApplicationSnapshot{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), appSnapshotLookupKey, createdAppSnapshot)
				return len(createdAppSnapshot.Spec.Components) > 0
			}, timeout, interval).Should(BeTrue())

			appBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: environmentName,
					Snapshot:    snapshotName,
					Components: []appstudioshared.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudioshared.BindingComponentConfiguration{
								Replicas: int(replicas),
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, appBinding)).Should(Succeed())

			bindingLookupKey := types.NamespacedName{Name: bindingName, Namespace: HASAppNamespace}
			createdBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) > 0
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.GitOpsRepoConditions[len(createdBinding.Status.GitOpsRepoConditions)-1].Message).Should(ContainSubstring("invalid control character in URL"))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)

			// Delete the specified snapshot
			deleteSnapshot(appSnapshotLookupKey)

		})
	})

})

// deleteBinding deletes the specified binding resource and verifies it was properly deleted
func deleteBinding(bindingLookupKey types.NamespacedName) {
	// Delete
	Eventually(func() error {
		f := &appstudioshared.ApplicationSnapshotEnvironmentBinding{}
		k8sClient.Get(context.Background(), bindingLookupKey, f)
		return k8sClient.Delete(context.Background(), f)
	}, timeout, interval).Should(Succeed())

	// Wait for delete to finish
	Eventually(func() error {
		f := &appstudioshared.ApplicationSnapshotEnvironmentBinding{}
		return k8sClient.Get(context.Background(), bindingLookupKey, f)
	}, timeout, interval).ShouldNot(Succeed())
}

// deleteSnapshot deletes the specified snapshot resource and verifies it was properly deleted
func deleteSnapshot(snapshotLookupKey types.NamespacedName) {
	// Delete
	Eventually(func() error {
		f := &appstudioshared.ApplicationSnapshot{}
		k8sClient.Get(context.Background(), snapshotLookupKey, f)
		return k8sClient.Delete(context.Background(), f)
	}, timeout, interval).Should(Succeed())

	// Wait for delete to finish
	Eventually(func() error {
		f := &appstudioshared.ApplicationSnapshot{}
		return k8sClient.Get(context.Background(), snapshotLookupKey, f)
	}, timeout, interval).ShouldNot(Succeed())
}
