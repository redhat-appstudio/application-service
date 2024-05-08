/*
Copyright 2022-2023 Red Hat, Inc.

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
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("SnapshotEnvironmentBinding controller", func() {

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

	Context("Create SnapshotEnvironmentBinding with component configurations", func() {
		It("Should reconcile successfully", func() {
			ctx := context.Background()

			applicationName := HASAppName + "1"
			componentName := HASCompName + "1"
			snapshotName := HASSnapshotName + "1"
			bindingName := HASBindingName + "1"
			environmentName := "staging" + "1"

			replicas := 3

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

			appSnapshot := &appstudiov1alpha1.Snapshot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Snapshot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.SnapshotSpec{
					Application:        applicationName,
					DisplayName:        "My Snapshot",
					DisplayDescription: "My Snapshot",
					Components: []appstudiov1alpha1.SnapshotComponent{
						{
							Name:           componentName,
							ContainerImage: "image1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appSnapshot)).Should(Succeed())

			appSnapshotLookupKey := types.NamespacedName{Name: snapshotName, Namespace: HASAppNamespace}
			createdAppSnapshot := &appstudiov1alpha1.Snapshot{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), appSnapshotLookupKey, createdAppSnapshot)
				return len(createdAppSnapshot.Spec.Components) > 0
			}, timeout, interval).Should(BeTrue())

			stagingEnv := &appstudiov1alpha1.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudiov1alpha1.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudiov1alpha1.EnvironmentConfiguration{
						Env: []appstudiov1alpha1.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

			appBinding := &appstudiov1alpha1.SnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "SnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.SnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: environmentName,
					Snapshot:    snapshotName,
					Components: []appstudiov1alpha1.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudiov1alpha1.BindingComponentConfiguration{
								Replicas: &replicas,
								Env: []appstudiov1alpha1.EnvVarPair{
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
			createdBinding := &appstudiov1alpha1.SnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) > 0 && len(createdBinding.Status.Components) == 1
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.Components[0].Name).Should(Equal(componentName))

			bindingLabels := createdBinding.GetLabels()
			// If no prior labels exist, SEB controllers should only add 2 label entries
			Expect(len(bindingLabels)).Should(Equal(2))
			Expect(bindingLabels["appstudio.application"]).Should(Equal(applicationName))
			Expect(bindingLabels["appstudio.environment"]).Should(Equal(environmentName))

			// Delete the specified HASComp resource
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)

			// Delete the specified snapshot
			deleteSnapshot(appSnapshotLookupKey)

			// Delete the specified environment
			stagingEnvLookupKey := types.NamespacedName{Name: environmentName, Namespace: HASAppNamespace}
			deleteEnvironment(stagingEnvLookupKey)
		})
	})

	Context("Create SnapshotEnvironmentBinding with a missing component", func() {
		It("Should fail if there is no such component by name", func() {
			ctx := context.Background()

			applicationName := HASAppName + "2"
			componentName := HASCompName + "2"
			componentName2 := HASCompName + "2-2"
			snapshotName := HASSnapshotName + "2"
			bindingName := HASBindingName + "2"
			environmentName := "staging" + "2"

			replicas := 3

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

			appSnapshot := &appstudiov1alpha1.Snapshot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Snapshot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.SnapshotSpec{
					Application:        applicationName,
					DisplayName:        "My Snapshot",
					DisplayDescription: "My Snapshot",
					Components: []appstudiov1alpha1.SnapshotComponent{
						{
							Name:           componentName,
							ContainerImage: "image1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appSnapshot)).Should(Succeed())

			appSnapshotLookupKey := types.NamespacedName{Name: snapshotName, Namespace: HASAppNamespace}
			createdAppSnapshot := &appstudiov1alpha1.Snapshot{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), appSnapshotLookupKey, createdAppSnapshot)
				return len(createdAppSnapshot.Spec.Components) > 0
			}, timeout, interval).Should(BeTrue())

			stagingEnv := &appstudiov1alpha1.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudiov1alpha1.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudiov1alpha1.EnvironmentConfiguration{
						Env: []appstudiov1alpha1.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

			appBinding := &appstudiov1alpha1.SnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "SnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.SnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: environmentName,
					Snapshot:    snapshotName,
					Components: []appstudiov1alpha1.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudiov1alpha1.BindingComponentConfiguration{
								Replicas: &replicas,
								Env: []appstudiov1alpha1.EnvVarPair{
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
							Configuration: appstudiov1alpha1.BindingComponentConfiguration{
								Replicas: &replicas,
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, appBinding)).Should(Succeed())

			bindingLookupKey := types.NamespacedName{Name: bindingName, Namespace: HASAppNamespace}
			createdBinding := &appstudiov1alpha1.SnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) > 0
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.GitOpsRepoConditions[len(createdBinding.Status.GitOpsRepoConditions)-1].Message).Should(ContainSubstring(fmt.Sprintf("%q not found", componentName2)))

			// Delete the specified HASComp resource
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)

			// Delete the specified snapshot
			deleteSnapshot(appSnapshotLookupKey)

			// Delete the specified environment
			stagingEnvLookupKey := types.NamespacedName{Name: environmentName, Namespace: HASAppNamespace}
			deleteEnvironment(stagingEnvLookupKey)
		})
	})

	Context("Create SnapshotEnvironmentBinding with a missing snapshot", func() {
		It("Should fail if there is no such snapshot by name", func() {
			ctx := context.Background()

			applicationName := HASAppName + "3"
			componentName := HASCompName + "3"
			snapshotName := HASSnapshotName + "3"
			bindingName := HASBindingName + "3"
			environmentName := "staging" + "3"

			replicas := 3

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

			stagingEnv := &appstudiov1alpha1.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudiov1alpha1.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudiov1alpha1.EnvironmentConfiguration{
						Env: []appstudiov1alpha1.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

			appBinding := &appstudiov1alpha1.SnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "SnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.SnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: environmentName,
					Snapshot:    snapshotName,
					Components: []appstudiov1alpha1.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudiov1alpha1.BindingComponentConfiguration{
								Replicas: &replicas,
								Env: []appstudiov1alpha1.EnvVarPair{
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
			createdBinding := &appstudiov1alpha1.SnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) > 0
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.GitOpsRepoConditions[len(createdBinding.Status.GitOpsRepoConditions)-1].Message).Should(ContainSubstring(fmt.Sprintf("%q not found", snapshotName)))

			// Delete the specified HASComp resource
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)

			// Delete the specified environment
			stagingEnvLookupKey := types.NamespacedName{Name: environmentName, Namespace: HASAppNamespace}
			deleteEnvironment(stagingEnvLookupKey)
		})
	})

	Context("Create SnapshotEnvironmentBinding and Snapshot referencing a wrong Application", func() {
		It("Should err out when Snapshot doesnt reference the same Application as the Binding", func() {
			ctx := context.Background()

			applicationName := HASAppName + "4"
			applicationName2 := HASAppName + "4-2"
			componentName := HASCompName + "4"
			snapshotName := HASSnapshotName + "4"
			bindingName := HASBindingName + "4"
			environmentName := "staging" + "4"

			replicas := 3

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

			appSnapshot := &appstudiov1alpha1.Snapshot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Snapshot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.SnapshotSpec{
					Application:        applicationName2,
					DisplayName:        "My Snapshot",
					DisplayDescription: "My Snapshot",
					Components: []appstudiov1alpha1.SnapshotComponent{
						{
							Name:           componentName,
							ContainerImage: "image1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appSnapshot)).Should(Succeed())

			appSnapshotLookupKey := types.NamespacedName{Name: snapshotName, Namespace: HASAppNamespace}
			createdAppSnapshot := &appstudiov1alpha1.Snapshot{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), appSnapshotLookupKey, createdAppSnapshot)
				return len(createdAppSnapshot.Spec.Components) > 0
			}, timeout, interval).Should(BeTrue())

			stagingEnv := &appstudiov1alpha1.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudiov1alpha1.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudiov1alpha1.EnvironmentConfiguration{
						Env: []appstudiov1alpha1.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

			appBinding := &appstudiov1alpha1.SnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "SnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.SnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: environmentName,
					Snapshot:    snapshotName,
					Components: []appstudiov1alpha1.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudiov1alpha1.BindingComponentConfiguration{
								Replicas: &replicas,
								Env: []appstudiov1alpha1.EnvVarPair{
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
			createdBinding := &appstudiov1alpha1.SnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) > 0
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.GitOpsRepoConditions[len(createdBinding.Status.GitOpsRepoConditions)-1].Message).Should(ContainSubstring(fmt.Sprintf("application snapshot %s does not belong to the application %s", snapshotName, applicationName)))

			// Delete the specified HASComp resource
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)

			// Delete the specified snapshot
			deleteSnapshot(appSnapshotLookupKey)

			// Delete the specified environment
			stagingEnvLookupKey := types.NamespacedName{Name: environmentName, Namespace: HASAppNamespace}
			deleteEnvironment(stagingEnvLookupKey)
		})
	})

	Context("Create SnapshotEnvironmentBinding and Component referencing a wrong Application", func() {
		It("Should err out when Component doesnt reference the same Application as the Binding", func() {
			ctx := context.Background()

			applicationName := HASAppName + "5"
			applicationName2 := HASAppName + "5-2"
			componentName := HASCompName + "5"
			componentName2 := HASCompName + "5-2"
			snapshotName := HASSnapshotName + "5"
			bindingName := HASBindingName + "5"
			environmentName := "staging" + "5"

			replicas := 3

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)
			createAndFetchSimpleApp(applicationName2, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

			hasComp2 := createAndFetchSimpleComponent(componentName2, HASAppNamespace, ComponentName+"2", applicationName2, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp2.Status.Devfile).Should(Not(Equal("")))

			appSnapshot := &appstudiov1alpha1.Snapshot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Snapshot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.SnapshotSpec{
					Application:        applicationName,
					DisplayName:        "My Snapshot",
					DisplayDescription: "My Snapshot",
					Components: []appstudiov1alpha1.SnapshotComponent{
						{
							Name:           componentName,
							ContainerImage: "image1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appSnapshot)).Should(Succeed())

			appSnapshotLookupKey := types.NamespacedName{Name: snapshotName, Namespace: HASAppNamespace}
			createdAppSnapshot := &appstudiov1alpha1.Snapshot{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), appSnapshotLookupKey, createdAppSnapshot)
				return len(createdAppSnapshot.Spec.Components) > 0
			}, timeout, interval).Should(BeTrue())

			stagingEnv := &appstudiov1alpha1.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudiov1alpha1.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudiov1alpha1.EnvironmentConfiguration{
						Env: []appstudiov1alpha1.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

			appBinding := &appstudiov1alpha1.SnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "SnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.SnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: environmentName,
					Snapshot:    snapshotName,
					Components: []appstudiov1alpha1.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudiov1alpha1.BindingComponentConfiguration{
								Replicas: &replicas,
								Env: []appstudiov1alpha1.EnvVarPair{
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
							Configuration: appstudiov1alpha1.BindingComponentConfiguration{
								Replicas: &replicas,
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, appBinding)).Should(Succeed())

			bindingLookupKey := types.NamespacedName{Name: bindingName, Namespace: HASAppNamespace}
			createdBinding := &appstudiov1alpha1.SnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) > 0
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.GitOpsRepoConditions[len(createdBinding.Status.GitOpsRepoConditions)-1].Message).Should(ContainSubstring(fmt.Sprintf("component %s does not belong to the application %s", componentName2, applicationName)))

			// Delete the specified HASComp resource
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			hasCompLookupKey2 := types.NamespacedName{Name: componentName2, Namespace: HASAppNamespace}
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

			// Delete the specified environment
			stagingEnvLookupKey := types.NamespacedName{Name: environmentName, Namespace: HASAppNamespace}
			deleteEnvironment(stagingEnvLookupKey)
		})
	})

	Context("Create SnapshotEnvironmentBinding and Snapshot referencing a wrong Application", func() {
		It("Should err out when Snapshot doesnt reference the same Application as the Binding", func() {
			ctx := context.Background()

			applicationName := HASAppName + "6"
			componentName := HASCompName + "6"
			componentName2 := HASCompName + "6-2"
			snapshotName := HASSnapshotName + "6"
			bindingName := HASBindingName + "6"
			environmentName := "staging" + "6"

			replicas := 3

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

			hasComp2 := createAndFetchSimpleComponent(componentName2, HASAppNamespace, ComponentName+"2", applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp2.Status.Devfile).Should(Not(Equal("")))

			appSnapshot := &appstudiov1alpha1.Snapshot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Snapshot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.SnapshotSpec{
					Application:        applicationName,
					DisplayName:        "My Snapshot",
					DisplayDescription: "My Snapshot",
					Components: []appstudiov1alpha1.SnapshotComponent{
						{
							Name:           componentName,
							ContainerImage: "image1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appSnapshot)).Should(Succeed())

			appSnapshotLookupKey := types.NamespacedName{Name: snapshotName, Namespace: HASAppNamespace}
			createdAppSnapshot := &appstudiov1alpha1.Snapshot{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), appSnapshotLookupKey, createdAppSnapshot)
				return len(createdAppSnapshot.Spec.Components) > 0
			}, timeout, interval).Should(BeTrue())

			stagingEnv := &appstudiov1alpha1.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudiov1alpha1.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudiov1alpha1.EnvironmentConfiguration{
						Env: []appstudiov1alpha1.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

			appBinding := &appstudiov1alpha1.SnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "SnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.SnapshotEnvironmentBindingSpec{
					Application: applicationName,
					Environment: environmentName,
					Snapshot:    snapshotName,
					Components: []appstudiov1alpha1.BindingComponent{
						{
							Name: componentName,
							Configuration: appstudiov1alpha1.BindingComponentConfiguration{
								Replicas: &replicas,
								Env: []appstudiov1alpha1.EnvVarPair{
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
							Configuration: appstudiov1alpha1.BindingComponentConfiguration{
								Replicas: &replicas,
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, appBinding)).Should(Succeed())

			bindingLookupKey := types.NamespacedName{Name: bindingName, Namespace: HASAppNamespace}
			createdBinding := &appstudiov1alpha1.SnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				return len(createdBinding.Status.GitOpsRepoConditions) > 0
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.GitOpsRepoConditions[len(createdBinding.Status.GitOpsRepoConditions)-1].Message).Should(ContainSubstring(fmt.Sprintf("application snapshot %s did not reference component %s", snapshotName, componentName2)))

			// Delete the specified HASComp resource
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			hasCompLookupKey2 := types.NamespacedName{Name: componentName2, Namespace: HASAppNamespace}
			deleteHASCompCR(hasCompLookupKey)
			deleteHASCompCR(hasCompLookupKey2)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)

			// Delete the specified snapshot
			deleteSnapshot(appSnapshotLookupKey)

			// Delete the specified environment
			stagingEnvLookupKey := types.NamespacedName{Name: environmentName, Namespace: HASAppNamespace}
			deleteEnvironment(stagingEnvLookupKey)
		})
	})

})

// deleteBinding deletes the specified binding resource and verifies it was properly deleted
func deleteBinding(bindingLookupKey types.NamespacedName) {
	// Delete
	Eventually(func() error {
		f := &appstudiov1alpha1.SnapshotEnvironmentBinding{}
		k8sClient.Get(context.Background(), bindingLookupKey, f)
		return k8sClient.Delete(context.Background(), f)
	}, timeout, interval).Should(Succeed())

	// Wait for delete to finish
	Eventually(func() error {
		f := &appstudiov1alpha1.SnapshotEnvironmentBinding{}
		return k8sClient.Get(context.Background(), bindingLookupKey, f)
	}, timeout, interval).ShouldNot(Succeed())
}

// deleteSnapshot deletes the specified snapshot resource and verifies it was properly deleted
func deleteSnapshot(snapshotLookupKey types.NamespacedName) {
	// Delete
	Eventually(func() error {
		f := &appstudiov1alpha1.Snapshot{}
		k8sClient.Get(context.Background(), snapshotLookupKey, f)
		return k8sClient.Delete(context.Background(), f)
	}, timeout, interval).Should(Succeed())

	// Wait for delete to finish
	Eventually(func() error {
		f := &appstudiov1alpha1.Snapshot{}
		return k8sClient.Get(context.Background(), snapshotLookupKey, f)
	}, timeout, interval).ShouldNot(Succeed())
}

// deleteEnvironment deletes the specified Environment resource and verifies it was properly deleted
func deleteEnvironment(environmentLookupKey types.NamespacedName) {
	// Delete
	Eventually(func() error {
		f := &appstudiov1alpha1.Environment{}
		k8sClient.Get(context.Background(), environmentLookupKey, f)
		return k8sClient.Delete(context.Background(), f)
	}, timeout, interval).Should(Succeed())

	// Wait for delete to finish
	Eventually(func() error {
		f := &appstudiov1alpha1.Environment{}
		return k8sClient.Get(context.Background(), environmentLookupKey, f)
	}, timeout, interval).ShouldNot(Succeed())
}

func createAndFetchSimpleComponent(name, namespace, componentName, application, gitRepo string, skipGitOps bool) appstudiov1alpha1.Component {
	comp := &appstudiov1alpha1.Component{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName: componentName,
			Application:   application,
			Source: appstudiov1alpha1.ComponentSource{
				ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
					GitSource: &appstudiov1alpha1.GitSource{
						URL: gitRepo,
					},
				},
			},
			SkipGitOpsResourceGeneration: skipGitOps,
		},
	}
	Expect(k8sClient.Create(ctx, comp)).Should(Succeed())

	// Look up the has app resource that was created.
	// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
	hasCompLookupKey := types.NamespacedName{Name: name, Namespace: namespace}
	createdComp := &appstudiov1alpha1.Component{}
	Eventually(func() bool {
		k8sClient.Get(context.Background(), hasCompLookupKey, createdComp)
		return len(createdComp.Status.Conditions) > 0
	}, timeout, interval).Should(BeTrue())

	Expect(createdComp.Status.Devfile).To(Not(Equal("")))

	return *createdComp
}
