/*
Copyright 2022 Red Hat, Inc.

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
			environmentName := "staging" + "1"

			hasGitopsGeneratedResource := map[string]bool{
				"deployment-patch.yaml": true,
			}

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

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

			stagingEnv := &appstudioshared.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudioshared.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudioshared.EnvironmentConfiguration{
						Env: []appstudioshared.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

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
			Expect(createdBinding.Status.Components[0].GitOpsRepository.URL).Should(Equal(hasComp.Status.GitOps.RepositoryURL))
			Expect(createdBinding.Status.Components[0].GitOpsRepository.CommitID).Should(Equal("ca82a6dff817ec66f44342007202690a93763949"))

			// check the list of generated gitops resources to make sure we account for every one
			for _, generatedResource := range createdBinding.Status.Components[0].GitOpsRepository.GeneratedResources {
				Expect(hasGitopsGeneratedResource[generatedResource]).Should(BeTrue())
			}

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

	Context("Create ApplicationSnapshotEnvironmentBinding with a missing component", func() {
		It("Should fail if there is no such component by name", func() {
			ctx := context.Background()

			applicationName := HASAppName + "2"
			componentName := HASCompName + "2"
			componentName2 := HASCompName + "2-2"
			snapshotName := HASSnapshotName + "2"
			bindingName := HASBindingName + "2"
			environmentName := "staging" + "2"

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

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

			stagingEnv := &appstudioshared.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudioshared.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudioshared.EnvironmentConfiguration{
						Env: []appstudioshared.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

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

	Context("Create ApplicationSnapshotEnvironmentBinding with a missing snapshot", func() {
		It("Should fail if there is no such snapshot by name", func() {
			ctx := context.Background()

			applicationName := HASAppName + "3"
			componentName := HASCompName + "3"
			snapshotName := HASSnapshotName + "3"
			bindingName := HASBindingName + "3"
			environmentName := "staging" + "3"

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

			stagingEnv := &appstudioshared.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudioshared.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudioshared.EnvironmentConfiguration{
						Env: []appstudioshared.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

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

	Context("Create ApplicationSnapshotEnvironmentBinding and Snapshot referencing a wrong Application", func() {
		It("Should err out when Snapshot doesnt reference the same Application as the Binding", func() {
			ctx := context.Background()

			applicationName := HASAppName + "4"
			applicationName2 := HASAppName + "4-2"
			componentName := HASCompName + "4"
			snapshotName := HASSnapshotName + "4"
			bindingName := HASBindingName + "4"
			environmentName := "staging" + "4"

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

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

			stagingEnv := &appstudioshared.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudioshared.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudioshared.EnvironmentConfiguration{
						Env: []appstudioshared.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

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

	Context("Create ApplicationSnapshotEnvironmentBinding and Component referencing a wrong Application", func() {
		It("Should err out when Component doesnt reference the same Application as the Binding", func() {
			ctx := context.Background()

			applicationName := HASAppName + "5"
			applicationName2 := HASAppName + "5-2"
			componentName := HASCompName + "5"
			componentName2 := HASCompName + "5-2"
			snapshotName := HASSnapshotName + "5"
			bindingName := HASBindingName + "5"
			environmentName := "staging" + "5"

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)
			createAndFetchSimpleApp(applicationName2, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

			hasComp2 := createAndFetchSimpleComponent(componentName2, HASAppNamespace, ComponentName+"2", applicationName2, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp2.Status.Devfile).Should(Not(Equal("")))

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

			stagingEnv := &appstudioshared.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudioshared.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudioshared.EnvironmentConfiguration{
						Env: []appstudioshared.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

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

	Context("Create ApplicationSnapshotEnvironmentBinding and Snapshot referencing a wrong Application", func() {
		It("Should err out when Snapshot doesnt reference the same Application as the Binding", func() {
			ctx := context.Background()

			applicationName := HASAppName + "6"
			componentName := HASCompName + "6"
			componentName2 := HASCompName + "6-2"
			snapshotName := HASSnapshotName + "6"
			bindingName := HASBindingName + "6"
			environmentName := "staging" + "6"

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

			hasComp2 := createAndFetchSimpleComponent(componentName2, HASAppNamespace, ComponentName+"2", applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp2.Status.Devfile).Should(Not(Equal("")))

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

			stagingEnv := &appstudioshared.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudioshared.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudioshared.EnvironmentConfiguration{
						Env: []appstudioshared.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

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

	Context("Update ApplicationSnapshotEnvironmentBinding with component configurations", func() {
		It("Should generate gitops overlays successfully", func() {
			ctx := context.Background()

			applicationName := HASAppName + "7"
			componentName := HASCompName + "7"
			snapshotName := HASSnapshotName + "7"
			bindingName := HASBindingName + "7"
			environmentName := "staging" + "7"

			replicas := int32(3)
			newReplicas := int32(4)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

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

			stagingEnv := &appstudioshared.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudioshared.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudioshared.EnvironmentConfiguration{
						Env: []appstudioshared.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

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

	Context("Create ApplicationSnapshotEnvironmentBinding with bad Component GitOps URL", func() {
		It("Should err out", func() {
			ctx := context.Background()

			applicationName := HASAppName + "8"
			componentName := HASCompName + "8"
			snapshotName := HASSnapshotName + "8"
			bindingName := HASBindingName + "8"
			environmentName := "staging" + "8"

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

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

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

			stagingEnv := &appstudioshared.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudioshared.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudioshared.EnvironmentConfiguration{
						Env: []appstudioshared.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

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
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
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

	Context("Create ApplicationSnapshotEnvironmentBinding with a missing environment", func() {
		It("Should fail if there is no such environment by name", func() {
			ctx := context.Background()

			applicationName := HASAppName + "9"
			componentName := HASCompName + "9"
			snapshotName := HASSnapshotName + "9"
			bindingName := HASBindingName + "9"
			environmentName := "staging" + "9"

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

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
				return len(createdBinding.Status.GitOpsRepoConditions) > 0
			}, timeout, interval).Should(BeTrue())

			Expect(createdBinding.Status.GitOpsRepoConditions[len(createdBinding.Status.GitOpsRepoConditions)-1].Message).Should(ContainSubstring(fmt.Sprintf("Environment.appstudio.redhat.com %q not found", environmentName)))

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
		})
	})

	Context("Update Environment configurations", func() {
		It("Should successfully reconcile all the Bindings that reference the Environment", func() {
			ctx := context.Background()

			applicationName := HASAppName + "10"
			applicationName2 := HASAppName + "10-2"
			componentName := HASCompName + "10"
			componentName2 := HASCompName + "10-2"
			snapshotName := HASSnapshotName + "10"
			snapshotName2 := HASSnapshotName + "10-2"
			bindingName := HASBindingName + "10"
			bindingName2 := HASBindingName + "10-2"
			environmentName := "staging" + "10"
			environmentName2 := "staging" + "10-2"

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)
			createAndFetchSimpleApp(applicationName2, HASAppNamespace, DisplayName, Description)

			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

			hasComp2 := createAndFetchSimpleComponent(componentName2, HASAppNamespace, ComponentName, applicationName2, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp2.Status.Devfile).Should(Not(Equal("")))

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

			appSnapshot2 := &appstudioshared.ApplicationSnapshot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName2,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.ApplicationSnapshotSpec{
					Application:        applicationName2,
					DisplayName:        "My Snapshot",
					DisplayDescription: "My Snapshot",
					Components: []appstudioshared.ApplicationSnapshotComponent{
						{
							Name:           componentName2,
							ContainerImage: "image1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appSnapshot2)).Should(Succeed())

			appSnapshotLookupKey2 := types.NamespacedName{Name: snapshotName2, Namespace: HASAppNamespace}
			createdAppSnapshot2 := &appstudioshared.ApplicationSnapshot{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), appSnapshotLookupKey2, createdAppSnapshot2)
				return len(createdAppSnapshot2.Spec.Components) > 0
			}, timeout, interval).Should(BeTrue())

			stagingEnv := &appstudioshared.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudioshared.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudioshared.EnvironmentConfiguration{
						Env: []appstudioshared.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR_ENV",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

			stagingEnv2 := &appstudioshared.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName2,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudioshared.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudioshared.EnvironmentConfiguration{
						Env: []appstudioshared.EnvVarPair{
							{
								Name:  "FOO2",
								Value: "BAR2_ENV",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv2)).Should(Succeed())

			appBinding := &appstudioshared.ApplicationSnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: HASAppNamespace,
					Labels: map[string]string{
						"appstudio.environment": environmentName,
						"appstudio.application": applicationName,
					},
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
				return len(createdBinding.Status.GitOpsRepoConditions) == 1 && len(createdBinding.Status.Components) == 1
			}, timeout, interval).Should(BeTrue())

			appBinding2 := &appstudioshared.ApplicationSnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ApplicationSnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName2,
					Namespace: HASAppNamespace,
					Labels: map[string]string{
						"appstudio.environment": environmentName2,
						"appstudio.application": applicationName2,
					},
				},
				Spec: appstudioshared.ApplicationSnapshotEnvironmentBindingSpec{
					Application: applicationName2,
					Environment: environmentName2,
					Snapshot:    snapshotName2,
					Components: []appstudioshared.BindingComponent{
						{
							Name: componentName2,
							Configuration: appstudioshared.BindingComponentConfiguration{
								Replicas: int(replicas),
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, appBinding2)).Should(Succeed())

			bindingLookupKey2 := types.NamespacedName{Name: bindingName2, Namespace: HASAppNamespace}
			createdBinding2 := &appstudioshared.ApplicationSnapshotEnvironmentBinding{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey2, createdBinding2)
				return len(createdBinding2.Status.GitOpsRepoConditions) == 1 && len(createdBinding2.Status.Components) == 1
			}, timeout, interval).Should(BeTrue())

			stagingEnvLookupKey := types.NamespacedName{Name: environmentName, Namespace: HASAppNamespace}
			createdStagingEnv := &appstudioshared.Environment{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), stagingEnvLookupKey, createdStagingEnv)
				return len(createdStagingEnv.Spec.Configuration.Env) == 1
			}, timeout, interval).Should(BeTrue())

			// Update the Env CR
			createdStagingEnv.Spec.Configuration.Env = append(createdStagingEnv.Spec.Configuration.Env, appstudioshared.EnvVarPair{Name: "FOO1", Value: "BAR1_ENV"})
			Expect(k8sClient.Update(ctx, createdStagingEnv)).Should(Succeed())

			// Get the updated resource
			Eventually(func() bool {
				k8sClient.Get(context.Background(), stagingEnvLookupKey, createdStagingEnv)
				// Return true if the most recent condition on the CR is updated
				return len(createdStagingEnv.Spec.Configuration.Env) == 2
			}, timeout, interval).Should(BeTrue())

			// check the status of the Binding for the Watch() function reconcile
			// Get the updated resource
			Eventually(func() bool {
				k8sClient.Get(context.Background(), bindingLookupKey, createdBinding)
				// Return true if the most recent condition on the CR is updated
				return len(createdBinding.Status.GitOpsRepoConditions) == 1
			}, timeout, interval).Should(BeTrue())

			// Delete the specified HASComp resource
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			hasCompLookupKey2 := types.NamespacedName{Name: componentName2, Namespace: HASAppNamespace}
			deleteHASCompCR(hasCompLookupKey)
			deleteHASCompCR(hasCompLookupKey2)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			hasAppLookupKey2 := types.NamespacedName{Name: applicationName2, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)
			deleteHASAppCR(hasAppLookupKey2)

			// Delete the specified binding
			deleteBinding(bindingLookupKey)
			deleteBinding(bindingLookupKey2)

			// Delete the specified snapshot
			deleteSnapshot(appSnapshotLookupKey)
			deleteSnapshot(appSnapshotLookupKey2)

			// Delete the specified environment
			stagingEnvLookupKey2 := types.NamespacedName{Name: environmentName2, Namespace: HASAppNamespace}
			deleteEnvironment(stagingEnvLookupKey)
			deleteEnvironment(stagingEnvLookupKey2)
		})
	})

	Context("Create ApplicationSnapshotEnvironmentBinding with multiple component configurations", func() {
		It("Should not generate gitops overlays successfully for Components that skip gitops", func() {
			ctx := context.Background()

			applicationName := HASAppName + "11"
			componentName := HASCompName + "11"
			secondComponentName := HASCompName + "11-2"
			snapshotName := HASSnapshotName + "11"
			bindingName := HASBindingName + "11"
			environmentName := "staging"

			hasGitopsGeneratedResource := map[string]bool{
				"deployment-patch.yaml": true,
			}

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)
			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, true)
			secondComp := createAndFetchSimpleComponent(secondComponentName, HASAppNamespace, secondComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))
			Expect(secondComp.Status.Devfile).Should(Not(Equal("")))

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
						{
							Name:           secondComponentName,
							ContainerImage: "image2",
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

			stagingEnv := &appstudioshared.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudioshared.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudioshared.EnvironmentConfiguration{
						Env: []appstudioshared.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

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
						{
							Name: secondComponentName,
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

			// Validate that the GitOps resources for the bound component(s) were generated, but not for any that explicitly had skipGitOpsResourceGeneration set
			Expect(createdBinding.Status.GitOpsRepoConditions[0].Message).Should(Equal("GitOps repository sync successful"))
			Expect(len(createdBinding.Status.Components)).Should(Equal(1))
			Expect(createdBinding.Status.Components[0].Name).Should(Equal(secondComponentName))
			Expect(createdBinding.Status.Components[0].GitOpsRepository.Path).Should(Equal(fmt.Sprintf("components/%s/overlays/%s", secondComponentName, environmentName)))
			Expect(createdBinding.Status.Components[0].GitOpsRepository.URL).Should(Equal(secondComp.Status.GitOps.RepositoryURL))

			// check the list of generated gitops resources to make sure we account for every one
			for _, generatedResource := range createdBinding.Status.Components[0].GitOpsRepository.GeneratedResources {
				Expect(hasGitopsGeneratedResource[generatedResource]).Should(BeTrue())
			}

			// Delete the specified Component resources
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			deleteHASCompCR(hasCompLookupKey)

			hasCompLookupKey = types.NamespacedName{Name: secondComponentName, Namespace: HASAppNamespace}
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified App resource
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

	Context("Create ApplicationSnapshotEnvironmentBinding with error retrieving git commit id", func() {
		It("Should return error with the proper message set", func() {
			ctx := context.Background()

			applicationName := HASAppName + "test-git-error" + "12"
			componentName := HASCompName + "12"
			snapshotName := HASSnapshotName + "12"
			bindingName := HASBindingName + "12"
			environmentName := "staging"

			replicas := int32(3)

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)
			hasComp := createAndFetchSimpleComponent(componentName, HASAppNamespace, ComponentName, applicationName, SampleRepoLink, false)
			// Make sure the devfile model was properly set in Component
			Expect(hasComp.Status.Devfile).Should(Not(Equal("")))

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

			stagingEnv := &appstudioshared.Environment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Environment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      environmentName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudioshared.EnvironmentSpec{
					Type:               "POC",
					DisplayName:        DisplayName,
					DeploymentStrategy: appstudioshared.DeploymentStrategy_AppStudioAutomated,
					Configuration: appstudioshared.EnvironmentConfiguration{
						Env: []appstudioshared.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, stagingEnv)).Should(Succeed())

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
				return len(createdBinding.Status.GitOpsRepoConditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Validate that the GitOps resources for the bound component(s) were generated, but not for any that explicitly had skipGitOpsResourceGeneration set
			Expect(createdBinding.Status.GitOpsRepoConditions[0].Status).Should(Equal(metav1.ConditionFalse))
			Expect(createdBinding.Status.GitOpsRepoConditions[0].Reason).Should(Equal("GenerateError"))
			Expect(createdBinding.Status.GitOpsRepoConditions[0].Message).Should(ContainSubstring("failed to retrieve commit id for repository"))

			// Delete the specified Component resources
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified App resource
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

// deleteEnvironment deletes the specified Environment resource and verifies it was properly deleted
func deleteEnvironment(environmentLookupKey types.NamespacedName) {
	// Delete
	Eventually(func() error {
		f := &appstudioshared.Environment{}
		k8sClient.Get(context.Background(), environmentLookupKey, f)
		return k8sClient.Delete(context.Background(), f)
	}, timeout, interval).Should(Succeed())

	// Wait for delete to finish
	Eventually(func() error {
		f := &appstudioshared.Environment{}
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
		return len(createdComp.Status.Conditions) > 0 && createdComp.Status.GitOps.RepositoryURL != ""
	}, timeout, interval).Should(BeTrue())

	Expect(createdComp.Status.Devfile).To(Not(Equal("")))

	return *createdComp
}
