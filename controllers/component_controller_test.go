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
	"strings"
	"testing"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	data "github.com/devfile/library/pkg/devfile/parser/data"
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Component controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		HASAppName      = "test-application"
		HASCompName     = "test-component"
		HASAppNamespace = "default"
		DisplayName     = "petclinic"
		Description     = "Simple petclinic app"
		ComponentName   = "backend"
		SampleRepoLink  = "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	)

	Context("Create Component with basic field set", func() {
		It("Should create successfully and update the Application", func() {
			ctx := context.Background()

			applicationName := HASAppName + "1"
			componentName := HASCompName + "1"

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
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.GetAnnotations()["gitOpsRepository.url"] != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ComponentName)
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Application
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			// Check the Component devfile
			_, err := devfile.ParseDevfileModel(createdHasComp.Status.Devfile)
			Expect(err).Should(Not(HaveOccurred()))

			// Check the HAS Application devfile
			hasAppDevfile, err := devfile.ParseDevfileModel(createdHasApp.Status.Devfile)
			Expect(err).Should(Not(HaveOccurred()))

			// gitOpsRepo and appModelRepo should both be set
			Expect(string(hasAppDevfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(Equal("")))
			Expect(string(hasAppDevfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(Not(Equal("")))

			// gitOpsRepo set in the component equal the repository in the app cr's devfile
			gitopsRepo := hasAppDevfile.GetMetadata().Attributes.GetString("gitOpsRepository.url", &err)
			Expect(err).Should(Not(HaveOccurred()))
			Expect(string(createdHasComp.GetObjectMeta().GetAnnotations()["gitOpsRepository.url"])).Should(Equal(gitopsRepo))

			hasProjects, err := hasAppDevfile.GetProjects(common.DevfileOptions{})
			Expect(err).Should(Not(HaveOccurred()))
			Expect(len(hasProjects)).ShouldNot(Equal(0))

			nameMatched := false
			repoLinkMatched := false
			for _, project := range hasProjects {
				if project.Name == ComponentName {
					nameMatched = true
				}
				if project.Git != nil && project.Git.GitLikeProjectSource.Remotes["origin"] == SampleRepoLink {
					repoLinkMatched = true
				}
			}
			Expect(nameMatched).Should(Equal(true))
			Expect(repoLinkMatched).Should(Equal(true))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with basic field set including devfileURL", func() {
		It("Should create successfully on a valid url", func() {
			ctx := context.Background()

			applicationName := HASAppName + "2"
			componentName := HASCompName + "2"

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
								URL:        SampleRepoLink,
								DevfileURL: "https://raw.githubusercontent.com/devfile/registry/main/stacks/java-openliberty/devfile.yaml",
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

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ComponentName)
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Application
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			// Check the Component devfile
			hasCompDevfile, err := devfile.ParseDevfileModel(createdHasComp.Status.Devfile)
			Expect(err).Should(Not(HaveOccurred()))

			// Check if its Liberty
			Expect(string(hasCompDevfile.GetMetadata().DisplayName)).Should(ContainSubstring("Liberty"))

			// Check the HAS Application devfile
			hasAppDevfile, err := devfile.ParseDevfileModel(createdHasApp.Status.Devfile)
			Expect(err).Should(Not(HaveOccurred()))

			// gitOpsRepo and appModelRepo should both be set
			Expect(string(hasAppDevfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(Equal("")))
			Expect(string(hasAppDevfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(Not(Equal("")))

			hasProjects, err := hasAppDevfile.GetProjects(common.DevfileOptions{})
			Expect(err).Should(Not(HaveOccurred()))
			Expect(len(hasProjects)).ShouldNot(Equal(0))

			nameMatched := false
			repoLinkMatched := false
			for _, project := range hasProjects {
				if project.Name == ComponentName {
					nameMatched = true
				}
				if project.Git != nil && project.Git.GitLikeProjectSource.Remotes["origin"] == SampleRepoLink {
					repoLinkMatched = true
				}
			}
			Expect(nameMatched).Should(Equal(true))
			Expect(repoLinkMatched).Should(Equal(true))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with basic field set including devfileURL", func() {
		It("Should error out on a bad url", func() {
			ctx := context.Background()

			applicationName := HASAppName + "3"
			componentName := HASCompName + "3"

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
								URL:        SampleRepoLink,
								DevfileURL: "https://bad/devfile.yaml",
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

			// Make sure the err was set
			Expect(createdHasComp.Status.Devfile).Should(Equal(""))
			Expect(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Reason).Should(Equal("Error"))
			Expect(strings.ToLower(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Message)).Should(ContainSubstring("unable to get"))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create a Component before an Application", func() {
		It("Should error out because an Application is missing", func() {
			ctx := context.Background()

			applicationName := HASAppName + "4"
			componentName := HASCompName + "4"

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

			// Make sure the err was set
			Expect(createdHasComp.Status.Devfile).Should(Equal(""))
			Expect(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Reason).Should(Equal("Error"))
			Expect(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Message).Should(ContainSubstring(fmt.Sprintf("%q not found", hasComp.Spec.Application)))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

		})
	})

	Context("Create Component with other field set", func() {
		It("Should create successfully and update the Application", func() {
			ctx := context.Background()

			applicationName := HASAppName + "5"
			componentName := HASCompName + "5"

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

			originalRoute := "route-endpoint-url"
			updatedRoute := "route-endpoint-url-2"

			originalReplica := 1
			updatedReplica := 2

			originalPort := 1111
			updatedPort := 2222

			storage1GiResource, err := resource.ParseQuantity("1Gi")
			Expect(err).Should(Not(HaveOccurred()))

			storage2GiResource, err := resource.ParseQuantity("2Gi")
			Expect(err).Should(Not(HaveOccurred()))

			core500mResource, err := resource.ParseQuantity("500m")
			Expect(err).Should(Not(HaveOccurred()))

			core800mResource, err := resource.ParseQuantity("800m")
			Expect(err).Should(Not(HaveOccurred()))

			originalEnv := []corev1.EnvVar{
				{
					Name:  "FOO",
					Value: "foo",
				},
				{
					Name:  "BAR",
					Value: "bar",
				},
			}

			updatedEnv := []corev1.EnvVar{
				{
					Name:  "FOO",
					Value: "foo1",
				},
				{
					Name:  "BAR",
					Value: "bar1",
				},
			}

			originalResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:              core500mResource,
					corev1.ResourceMemory:           storage1GiResource,
					corev1.ResourceStorage:          storage1GiResource,
					corev1.ResourceEphemeralStorage: storage1GiResource,
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              core500mResource,
					corev1.ResourceMemory:           storage1GiResource,
					corev1.ResourceStorage:          storage1GiResource,
					corev1.ResourceEphemeralStorage: storage1GiResource,
				},
			}

			updatedResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:              core800mResource,
					corev1.ResourceMemory:           storage2GiResource,
					corev1.ResourceStorage:          storage2GiResource,
					corev1.ResourceEphemeralStorage: storage2GiResource,
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              core800mResource,
					corev1.ResourceMemory:           storage2GiResource,
					corev1.ResourceStorage:          storage2GiResource,
					corev1.ResourceEphemeralStorage: storage2GiResource,
				},
			}

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
					Build: appstudiov1alpha1.Build{
						ContainerImage: "quay.io/test/test-image:latest",
					},
					Replicas:   originalReplica,
					TargetPort: originalPort,
					Route:      originalRoute,
					Env:        originalEnv,
					Resources:  originalResources,
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the has app resource that was created.
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.GetLabels()["appstudio.has/component"] != ""
			}, timeout, interval).Should(BeTrue())

			// Validate that the built container image was set in the status
			Expect(createdHasComp.Status.ContainerImage).Should(Equal("quay.io/test/test-image:latest"))

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ComponentName)
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Application
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			// Check the Component devfile
			hasCompDevfile, err := devfile.ParseDevfileModel(createdHasComp.Status.Devfile)
			Expect(err).Should(Not(HaveOccurred()))

			checklist := updateChecklist{
				route:     originalRoute,
				replica:   originalReplica,
				port:      originalPort,
				env:       originalEnv,
				resources: originalResources,
			}

			verifyHASComponentUpdates(hasCompDevfile, checklist, nil)

			// Check the HAS Application devfile
			hasAppDevfile, err := devfile.ParseDevfileModel(createdHasApp.Status.Devfile)
			Expect(err).Should(Not(HaveOccurred()))

			// gitOpsRepo and appModelRepo should both be set
			Expect(string(hasAppDevfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(Equal("")))
			Expect(string(hasAppDevfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(Not(Equal("")))

			// project should be set in hasApp
			hasProjects, err := hasAppDevfile.GetProjects(common.DevfileOptions{})
			Expect(err).Should(Not(HaveOccurred()))
			Expect(len(hasProjects)).ShouldNot(Equal(0))

			nameMatched := false
			repoLinkMatched := false
			for _, project := range hasProjects {
				if project.Name == ComponentName {
					nameMatched = true
				}
				if project.Git != nil && project.Git.GitLikeProjectSource.Remotes["origin"] == SampleRepoLink {
					repoLinkMatched = true
				}
			}
			Expect(nameMatched).Should(Equal(true))
			Expect(repoLinkMatched).Should(Equal(true))

			// update the hasComp and apply
			createdHasComp.Spec.Replicas = updatedReplica
			createdHasComp.Spec.Route = updatedRoute
			createdHasComp.Spec.TargetPort = updatedPort
			createdHasComp.Spec.Env = updatedEnv
			createdHasComp.Spec.Resources = updatedResources
			createdHasComp.Spec.Build.ContainerImage = "quay.io/newimage/newimage:latest"

			Expect(k8sClient.Update(ctx, createdHasComp)).Should(Succeed())

			updatedHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, updatedHasComp)
				return updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Type == "Updated"
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(updatedHasComp.Status.ContainerImage).Should(Equal("quay.io/newimage/newimage:latest"))

			// Make sure the devfile model was properly set in Component
			Expect(updatedHasComp.Status.Devfile).Should(Not(Equal("")))

			// Check the Component updated devfile
			hasCompUpdatedDevfile, err := devfile.ParseDevfileModel(updatedHasComp.Status.Devfile)
			Expect(err).Should(Not(HaveOccurred()))

			checklist = updateChecklist{
				route:     updatedRoute,
				replica:   updatedReplica,
				port:      updatedPort,
				env:       updatedEnv,
				resources: updatedResources,
			}

			verifyHASComponentUpdates(hasCompUpdatedDevfile, checklist, nil)

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with built container image set", func() {
		It("Should create successfully", func() {
			ctx := context.Background()

			applicationName := HASAppName + "6"
			componentName := HASCompName + "6"

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
					Build: appstudiov1alpha1.Build{
						ContainerImage: "quay.io/test/testimage:latest",
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
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.ContainerImage != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ComponentName)
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Application
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			// Check the Component devfile
			_, err := devfile.ParseDevfileModel(createdHasComp.Status.Devfile)
			Expect(err).Should(Not(HaveOccurred()))

			// Make sure the component's built image is included in the status
			Expect(createdHasComp.Status.ContainerImage).Should(Equal("quay.io/test/testimage:latest"))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Component with invalid devfile", func() {
		It("Should fail and have proper failure condition set", func() {
			ctx := context.Background()

			applicationName := HASAppName + "7"
			componentName := HASCompName + "7"

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
					Build: appstudiov1alpha1.Build{
						ContainerImage: "quay.io/test/testimage:latest",
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the component resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.GetLabels()["appstudio.has/component"] != ""
			}, timeout, interval).Should(BeTrue())

			// Remove the component's devfile and update a field in the spec to trigger a reconcile
			createdHasComp.Status.Devfile = "a"
			Expect(k8sClient.Status().Update(ctx, createdHasComp)).Should(Succeed())

			createdHasComp.Spec.Build.ContainerImage = "test"
			Expect(k8sClient.Update(ctx, createdHasComp)).Should(Succeed())

			updatedHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, updatedHasComp)
				return updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Type == "Updated"
			}, timeout, interval).Should(BeTrue())

			errCondition := updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1]
			Expect(errCondition.Status).Should(Equal(metav1.ConditionFalse))
			Expect(errCondition.Message).Should(ContainSubstring("failed to decode devfile json"))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	// The following two tests test that we properly return an error when the gitops resource generation errors out for some reason
	// To trigger a gitops generation failure we can:
	// 1. Use an invalid gitops repo url,
	// 2. Remove the gitops repository annotations, or
	// 3. Create a mock executor to emulate exec failures (difficult to do with current test setup)
	// This first test will just use an invalid gitops repository url for the component
	Context("Component with gitops resource generation failure", func() {
		It("Should have proper failure condition set", func() {
			ctx := context.Background()

			applicationName := HASAppName + "8"
			componentName := HASCompName + "8"

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
						URL: "https://github.com/redhat-appstudio-appdata/!@#$%U%I$F    DFDN##",
					},
				},
			}

			Expect(k8sClient.Create(ctx, hasApp)).Should(Succeed())

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
					Build: appstudiov1alpha1.Build{
						ContainerImage: "quay.io/test/testimage:latest",
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the component resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Status == metav1.ConditionFalse
			}, timeout, interval).Should(BeTrue())

			errCondition := createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1]
			Expect(errCondition.Status).Should(Equal(metav1.ConditionFalse))
			Expect(errCondition.Message).Should(ContainSubstring("Unable to generate gitops resources"))
			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	// This test will create an Application and a Component, then remove the gitops repository annotation from the component and update it
	// The gitops generation should fail due to the gitops repository annotation missing
	Context("Component updated with missing gitops annotation", func() {
		It("Should have gitops generation failure and set proper error condition", func() {
			ctx := context.Background()

			applicationName := HASAppName + "9"
			componentName := HASCompName + "9"

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
					Build: appstudiov1alpha1.Build{
						ContainerImage: "quay.io/test/testimage:latest",
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the component resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.GetLabels()["appstudio.has/component"] != ""
			}, timeout, interval).Should(BeTrue())

			// Remove the gitops notifications
			createdHasComp.Spec.Build.ContainerImage = "Newimage"
			createdHasComp.SetAnnotations(nil)

			Expect(k8sClient.Update(ctx, createdHasComp)).Should(Succeed())

			updatedHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, updatedHasComp)
				return updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Type == "Updated"
			}, timeout, interval).Should(BeTrue())

			Expect(updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Status).Should(Equal(metav1.ConditionFalse))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	// This test will create an Application and a Component, then remove the gitops repository annotation from the component and update it
	// The gitops generation should fail due to the gitops repository annotation missing
	Context("Component created with App with missing gitops repository", func() {
		It("Should fail since Application has no gitops repository", func() {
			var err error
			ctx := context.Background()

			applicationName := HASAppName + "10"
			componentName := HASCompName + "10"

			hasApp := createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)
			curDevfile, err := devfile.ParseDevfileModel(hasApp.Status.Devfile)
			Expect(err).ToNot(HaveOccurred())

			// Remove the gitops URL and update the status of the resource
			devfileMeta := curDevfile.GetMetadata()
			devfileMeta.Attributes = attributes.Attributes{}
			curDevfile.SetMetadata(devfileMeta)
			devfileYaml, err := yaml.Marshal(curDevfile)
			Expect(err).ToNot(HaveOccurred())
			hasApp.Status.Devfile = string(devfileYaml)
			Expect(k8sClient.Status().Update(context.Background(), hasApp)).Should(Succeed())

			// Wait for the application resource to be updated
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, hasApp)

				// Return true if the fetched resource has our "updated" devfile status
				return hasApp.Status.Devfile == string(devfileYaml)
			}, timeout, interval).Should(BeTrue())

			// Create the hasComp resource
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
					Build: appstudiov1alpha1.Build{
						ContainerImage: "quay.io/test/testimage:latest",
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the component resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Status == metav1.ConditionFalse
			}, timeout, interval).Should(BeTrue())

			Expect(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Message).Should(ContainSubstring("unable to retrieve GitOps repository from Application CR devfile"))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

})

type updateChecklist struct {
	route     string
	port      int
	replica   int
	env       []corev1.EnvVar
	resources corev1.ResourceRequirements
}

// verifyHASComponentUpdates verifies if the devfile data has been properly updated with the Component CR values
func verifyHASComponentUpdates(devfile data.DevfileData, checklist updateChecklist, goPkgTest *testing.T) {
	// container component should be updated with the necessary hasComp properties
	components, err := devfile.GetComponents(common.DevfileOptions{
		ComponentOptions: common.ComponentOptions{
			ComponentType: v1alpha2.ContainerComponentType,
		},
	})
	if goPkgTest == nil {
		Expect(err).Should(Not(HaveOccurred()))
	} else if err != nil {
		goPkgTest.Error(err)
	}

	requests := checklist.resources.Requests
	limits := checklist.resources.Limits

	for i, component := range components {
		attributes := component.Attributes
		var err error

		// Check the route
		if checklist.route != "" {
			route := attributes.Get(routeKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(route).Should(Equal(checklist.route))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if route != checklist.route {
				goPkgTest.Errorf("expected: %v, got: %v", checklist.route, route)
			}
		}

		// Check the replica
		if checklist.replica != 0 {
			replicas := attributes.Get(replicaKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(replicas).Should(Equal(float64(checklist.replica)))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if int(replicas.(float64)) != checklist.replica {
				goPkgTest.Errorf("expected: %v, got: %v", checklist.replica, replicas)
			}
		}

		// Check the storage limit
		if _, ok := limits[corev1.ResourceStorage]; ok {
			storageLimitChecklist := limits[corev1.ResourceStorage]
			storageLimit := attributes.Get(storageLimitKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(storageLimit).Should(Equal(storageLimitChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if storageLimit.(string) != storageLimitChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", storageLimitChecklist.String(), storageLimit)
			}
		}

		// Check the storage request
		if _, ok := requests[corev1.ResourceStorage]; ok {
			storageRequestChecklist := requests[corev1.ResourceStorage]
			storageRequest := attributes.Get(storageRequestKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(storageRequest).Should(Equal(storageRequestChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if storageRequest.(string) != storageRequestChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", storageRequestChecklist.String(), storageRequest)
			}
		}

		// Check the ephemeral storage limit
		if _, ok := limits[corev1.ResourceEphemeralStorage]; ok {
			ephemeralStorageLimitChecklist := limits[corev1.ResourceEphemeralStorage]
			ephemeralStorageLimit := attributes.Get(ephemeralStorageLimitKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(ephemeralStorageLimit).Should(Equal(ephemeralStorageLimitChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if ephemeralStorageLimit.(string) != ephemeralStorageLimitChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", ephemeralStorageLimitChecklist.String(), ephemeralStorageLimit)
			}
		}

		// Check the ephemeral storage request
		if _, ok := requests[corev1.ResourceEphemeralStorage]; ok {
			ephemeralStorageRequestChecklist := requests[corev1.ResourceEphemeralStorage]
			ephemeralStorageRequest := attributes.Get(ephemeralStorageRequestKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(ephemeralStorageRequest).Should(Equal(ephemeralStorageRequestChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if ephemeralStorageRequest.(string) != ephemeralStorageRequestChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", ephemeralStorageRequestChecklist.String(), ephemeralStorageRequest)
			}
		}

		// Check the memory limit
		if _, ok := limits[corev1.ResourceMemory]; ok {
			memoryLimitChecklist := limits[corev1.ResourceMemory]
			if goPkgTest == nil {
				Expect(component.Container.MemoryLimit).Should(Equal(memoryLimitChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if component.Container.MemoryLimit != memoryLimitChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", memoryLimitChecklist.String(), component.Container.MemoryLimit)
			}
		}

		// Check the memory request
		if _, ok := requests[corev1.ResourceMemory]; ok {
			memoryRequestChecklist := requests[corev1.ResourceMemory]
			if goPkgTest == nil {
				Expect(component.Container.MemoryRequest).Should(Equal(memoryRequestChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if component.Container.MemoryRequest != memoryRequestChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", memoryRequestChecklist.String(), component.Container.MemoryRequest)
			}
		}

		// Check the cpu limit
		if _, ok := limits[corev1.ResourceCPU]; ok {
			cpuLimitChecklist := limits[corev1.ResourceCPU]
			if goPkgTest == nil {
				Expect(component.Container.CpuLimit).Should(Equal(cpuLimitChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if component.Container.CpuLimit != cpuLimitChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", cpuLimitChecklist.String(), component.Container.CpuLimit)
			}
		}

		// Check the cpu request
		if _, ok := requests[corev1.ResourceCPU]; ok {
			cpuRequestChecklist := requests[corev1.ResourceCPU]
			if goPkgTest == nil {
				Expect(component.Container.CpuRequest).Should(Equal(cpuRequestChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if component.Container.CpuRequest != cpuRequestChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", cpuRequestChecklist.String(), component.Container.CpuRequest)
			}
		}

		// Check for container endpoint only for the first container
		if i == 0 && checklist.port > 0 {
			for _, endpoint := range component.Container.Endpoints {
				if goPkgTest == nil {
					Expect(endpoint.TargetPort).Should(Equal(checklist.port))
				} else if err != nil {
					goPkgTest.Error(err)
				} else if endpoint.TargetPort != checklist.port {
					goPkgTest.Errorf("expected: %v, got: %v", checklist.port, endpoint.TargetPort)
				}
			}
		}

		// Check for env
		for _, checklistEnv := range checklist.env {
			isMatched := false
			for _, containerEnv := range component.Container.Env {
				if containerEnv.Name == checklistEnv.Name && containerEnv.Value == checklistEnv.Value {
					isMatched = true
				}
			}
			if goPkgTest == nil {
				Expect(isMatched).Should(Equal(true))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if !isMatched {
				goPkgTest.Errorf("expected: %v, got: %v", true, isMatched)
			}
		}
	}
}

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
