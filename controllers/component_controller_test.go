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
	"strings"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	data "github.com/devfile/library/pkg/devfile/parser/data"
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Component controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		HASAppName      = "test-hasapplication-123"
		HASCompName     = "test-hascomponent-123"
		HASAppNamespace = "default"
		DisplayName     = "petclinic"
		Description     = "Simple petclinic app"
		ComponentName   = "backend"
		SampleRepoLink  = "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	)

	Context("Create Component with basic field set", func() {
		It("Should create successfully and update the Application", func() {
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

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      HASCompName,
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
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: HASCompName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			hasAppLookupKey := types.NamespacedName{Name: HASAppName, Namespace: HASAppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ComponentName)
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Application
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			// Check the HAS Component devfile
			_, err := devfile.ParseDevfileModel(createdHasComp.Status.Devfile)
			Expect(err).Should(Not(HaveOccurred()))

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

	Context("Create Component with other field set", func() {
		It("Should create successfully and update the Application", func() {
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
					Name:      HASCompName,
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
					Replicas:   originalReplica,
					TargetPort: originalPort,
					Route:      originalRoute,
					Env:        originalEnv,
					Resources:  originalResources,
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the has app resource that was created.
			hasCompLookupKey := types.NamespacedName{Name: HASCompName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			hasAppLookupKey := types.NamespacedName{Name: HASAppName, Namespace: HASAppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ComponentName)
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Application
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			// Check the HAS Component devfile
			hasCompDevfile, err := devfile.ParseDevfileModel(createdHasComp.Status.Devfile)
			Expect(err).Should(Not(HaveOccurred()))

			checklist := updateChecklist{
				route:     originalRoute,
				replica:   originalReplica,
				port:      originalPort,
				env:       originalEnv,
				resources: originalResources,
			}

			verifyHASComponentUpdates(hasCompDevfile, checklist)

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
			Expect(k8sClient.Update(ctx, createdHasComp)).Should(Succeed())

			updatedHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, updatedHasComp)
				return updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Type == "Updated"
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(updatedHasComp.Status.Devfile).Should(Not(Equal("")))

			// Check the HAS Component updated devfile
			hasCompUpdatedDevfile, err := devfile.ParseDevfileModel(updatedHasComp.Status.Devfile)
			Expect(err).Should(Not(HaveOccurred()))

			checklist = updateChecklist{
				route:     updatedRoute,
				replica:   updatedReplica,
				port:      updatedPort,
				env:       updatedEnv,
				resources: updatedResources,
			}

			verifyHASComponentUpdates(hasCompUpdatedDevfile, checklist)

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

func verifyHASComponentUpdates(devfile data.DevfileData, checklist updateChecklist) {
	// container component should be updated with the necessary hasComp properties
	components, err := devfile.GetComponents(common.DevfileOptions{
		ComponentOptions: common.ComponentOptions{
			ComponentType: v1alpha2.ContainerComponentType,
		},
	})
	Expect(err).Should(Not(HaveOccurred()))

	requests := checklist.resources.Requests
	limits := checklist.resources.Limits

	for i, component := range components {
		attributes := component.Attributes
		var err error

		// Check the route
		route := attributes.Get("appstudio.has/route", &err)
		Expect(err).Should(Not(HaveOccurred()))
		Expect(route).Should(Equal(checklist.route))

		// Check the replica
		replicas := attributes.Get("appstudio.has/replicas", &err)
		Expect(err).Should(Not(HaveOccurred()))
		Expect(replicas).Should(Equal(float64(checklist.replica)))

		// Check the storage limit
		storageLimitChecklist := limits[corev1.ResourceStorage]
		storageLimit := attributes.Get("appstudio.has/storageLimit", &err)
		Expect(err).Should(Not(HaveOccurred()))
		Expect(storageLimit).Should(Equal(storageLimitChecklist.String()))

		// Check the storage request
		storageRequestChecklist := requests[corev1.ResourceStorage]
		storageRequest := attributes.Get("appstudio.has/storageRequest", &err)
		Expect(err).Should(Not(HaveOccurred()))
		Expect(storageRequest).Should(Equal(storageRequestChecklist.String()))

		// Check the ephemereal storage limit
		ephemeralStorageLimitChecklist := limits[corev1.ResourceEphemeralStorage]
		ephemeralStorageLimit := attributes.Get("appstudio.has/ephermealStorageLimit", &err)
		Expect(err).Should(Not(HaveOccurred()))
		Expect(ephemeralStorageLimit).Should(Equal(ephemeralStorageLimitChecklist.String()))

		// Check the ephemereal storage request
		ephemeralStorageRequestChecklist := requests[corev1.ResourceEphemeralStorage]
		ephemeralStorageRequest := attributes.Get("appstudio.has/ephermealStorageRequest", &err)
		Expect(err).Should(Not(HaveOccurred()))
		Expect(ephemeralStorageRequest).Should(Equal(ephemeralStorageRequestChecklist.String()))

		// Check the memory limit
		memoryLimitChecklist := limits[corev1.ResourceMemory]
		Expect(component.Container.MemoryLimit).Should(Equal(memoryLimitChecklist.String()))

		// Check the memory request
		memoryRequestChecklist := requests[corev1.ResourceMemory]
		Expect(component.Container.MemoryRequest).Should(Equal(memoryRequestChecklist.String()))

		// Check the cpu limit
		cpuLimitChecklist := limits[corev1.ResourceCPU]
		Expect(component.Container.CpuLimit).Should(Equal(cpuLimitChecklist.String()))

		// Check the cpu request
		cpuRequestChecklist := requests[corev1.ResourceCPU]
		Expect(component.Container.CpuRequest).Should(Equal(cpuRequestChecklist.String()))

		// Check for container endpoint only for the first container
		if i == 0 {
			for _, endpoint := range component.Container.Endpoints {
				Expect(endpoint.TargetPort).Should(Equal(checklist.port))
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
			Expect(isMatched).Should(Equal(true))
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
