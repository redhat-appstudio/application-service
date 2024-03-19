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
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redhat-appstudio/application-service/pkg/metrics"

	"github.com/devfile/library/v2/pkg/devfile/parser"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	cdqanalysis "github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	devfilePkg "github.com/redhat-appstudio/application-service/pkg/devfile"
	spiapi "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Component controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		HASAppName           = "test-application"
		HASCompName          = "test-component"
		HASAppNamespace      = "default"
		DisplayName          = "petclinic"
		Description          = "Simple petclinic app"
		ComponentName        = "backend"
		SampleRepoLink       = "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
		SampleGitlabRepoLink = "https://gitlab.com/devfile-samples/devfile-sample-java-springboot-basic"
		gitToken             = "" //empty for public repo test
	)

	prometheus.MustRegister(metrics.GetComponentCreationTotalReqs(), metrics.GetComponentCreationFailed(), metrics.GetComponentCreationSucceeded())

	Context("Create Component with basic field set", func() {
		It("Should create successfully and update the Application", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "1"
			componentName := HASCompName + "1"

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
			// num(conditions) may still be < 2 (GeneratedGitOps, Created) on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 1 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Verify that the GitOpsGenerated status condition was also set
			// ToDo: Add helper func for accessing the status conditions in a better way
			gitopsCondition := createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-2]
			Expect(gitopsCondition.Type).To(Equal("GitOpsResourcesGenerated"))
			Expect(gitopsCondition.Status).To(Equal(metav1.ConditionTrue))

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
			_, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasComp.Status.Devfile)})
			Expect(err).Should(Not(HaveOccurred()))

			// Check the HAS Application devfile
			hasAppDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasApp.Status.Devfile)})

			Expect(err).Should(Not(HaveOccurred()))

			// gitOpsRepo and appModelRepo should both be set
			Expect(string(hasAppDevfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(Equal("")))
			Expect(string(hasAppDevfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(Not(Equal("")))

			// gitOpsRepo set in the component equal the repository in the app cr's devfile
			gitopsRepo := hasAppDevfile.GetMetadata().Attributes.GetString("gitOpsRepository.url", &err)
			Expect(err).Should(Not(HaveOccurred()))
			Expect(string(createdHasComp.Status.GitOps.RepositoryURL)).Should(Equal(gitopsRepo))

			// Commit ID should be set in the gitops repository and not be empty
			Expect(createdHasComp.Status.GitOps.CommitID).Should(Not(BeEmpty()))

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

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with basic field set including devfileURL", func() {
		It("Should create successfully on a valid url", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "2"
			componentName := HASCompName + "2"

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
								URL:        SampleRepoLink,
								DevfileURL: "https://raw.githubusercontent.com/devfile/registry/main/stacks/java-openliberty/devfile.yaml",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 2 (GeneratedGitOps, Created) on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 1
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
			hasCompDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasComp.Status.Devfile)})

			Expect(err).Should(Not(HaveOccurred()))

			// Check if its Liberty
			Expect(string(hasCompDevfile.GetMetadata().DisplayName)).Should(ContainSubstring("Liberty"))

			// Check the HAS Application devfile
			hasAppDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasApp.Status.Devfile)})

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

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with basic field set including devfileURL", func() {
		It("Should error out on a bad url", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "3"
			componentName := HASCompName + "3"

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
			Expect(strings.ToLower(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Message)).Should(ContainSubstring("error getting devfile"))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) == beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) > beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create a Component before an Application", func() {
		It("Should reconcile once the application is created", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

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
			}, timeout40s, interval).Should(BeTrue())

			// Make sure the err was set
			Expect(createdHasComp.Status.Devfile).Should(Equal(""))
			Expect(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Reason).Should(Equal("Error"))
			Expect(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Message).Should(ContainSubstring(fmt.Sprintf("unable to get the Application %s", hasComp.Spec.Application)))

			compAnnotations := createdHasComp.GetAnnotations()
			Expect(compAnnotations).ShouldNot(BeNil())
			Expect(compAnnotations[applicationFailCounterAnnotation]).Should(Not(Equal("")))

			// Now create the application resource that it references
			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			// Now fetch the Component resource and validate that eventually its status condition changes to succcess
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.Conditions[0].Reason == "OK"
			}, timeout40s, interval).Should(BeTrue())

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASAppCR(types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace})
			deleteHASCompCR(hasCompLookupKey)

		})
	})

	Context("Create Component with other field set", func() {
		It("Should create successfully and update the Application", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "5"
			componentName := HASCompName + "5"

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

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
					corev1.ResourceCPU:     core500mResource,
					corev1.ResourceMemory:  storage1GiResource,
					corev1.ResourceStorage: storage1GiResource,
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:     core500mResource,
					corev1.ResourceMemory:  storage1GiResource,
					corev1.ResourceStorage: storage1GiResource,
				},
			}

			updatedResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:     core800mResource,
					corev1.ResourceMemory:  storage2GiResource,
					corev1.ResourceStorage: storage2GiResource,
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:     core800mResource,
					corev1.ResourceMemory:  storage2GiResource,
					corev1.ResourceStorage: storage2GiResource,
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
					ComponentName:  ComponentName,
					Application:    applicationName,
					ContainerImage: "quay.io/test/test-image:latest",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: SampleRepoLink,
							},
						},
					},
					Replicas:   &originalReplica,
					TargetPort: originalPort,
					Route:      originalRoute,
					Env:        originalEnv,
					Resources:  originalResources,
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the component resource that was created.
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 1
			}, timeout40s, interval).Should(BeTrue())

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
			hasCompDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasComp.Status.Devfile)})
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
			hasAppDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasApp.Status.Devfile)})
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
			createdHasComp.Spec.Replicas = &updatedReplica
			createdHasComp.Spec.Route = updatedRoute
			createdHasComp.Spec.TargetPort = updatedPort
			createdHasComp.Spec.Env = updatedEnv
			createdHasComp.Spec.Resources = updatedResources
			createdHasComp.Spec.ContainerImage = "quay.io/newimage/newimage:latest"

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
			hasCompUpdatedDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(updatedHasComp.Status.Devfile)})

			Expect(err).Should(Not(HaveOccurred()))

			checklist = updateChecklist{
				route:     updatedRoute,
				replica:   updatedReplica,
				port:      updatedPort,
				env:       updatedEnv,
				resources: updatedResources,
			}

			verifyHASComponentUpdates(hasCompUpdatedDevfile, checklist, nil)

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with built container image set", func() {
		It("Should create successfully", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			beforeImportGitRepoTotalReqs := testutil.ToFloat64(metrics.ImportGitRepoTotalReqs)
			beforeImportGitRepoSucceeded := testutil.ToFloat64(metrics.ImportGitRepoSucceeded)
			beforeImportGitRepoFailed := testutil.ToFloat64(metrics.ImportGitRepoFailed)

			applicationName := HASAppName + "6"
			componentName := HASCompName + "6"

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
					ComponentName:  ComponentName,
					Application:    applicationName,
					ContainerImage: "quay.io/test/testimage:latest",
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
			// num(conditions) may still be < 2 (GitOpsGenerated, Created) on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 1 && createdHasComp.Status.ContainerImage != ""
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
			_, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasComp.Status.Devfile)})

			Expect(err).Should(Not(HaveOccurred()))

			// Make sure the component's built image is included in the status
			Expect(createdHasComp.Status.ContainerImage).Should(Equal("quay.io/test/testimage:latest"))

			Expect(testutil.ToFloat64(metrics.ImportGitRepoTotalReqs) > beforeImportGitRepoTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.ImportGitRepoSucceeded) > beforeImportGitRepoSucceeded).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.ImportGitRepoFailed) == beforeImportGitRepoFailed).To(BeTrue())

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Component with invalid devfile", func() {
		It("Should fail and have proper failure condition set", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "7"
			componentName := HASCompName + "7"

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
					ComponentName:  ComponentName,
					Application:    applicationName,
					ContainerImage: "quay.io/test/testimage:latest",
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

			// Look up the component resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Remove the component's devfile and update a field in the spec to trigger a reconcile
			createdHasComp.Status.Devfile = "a"
			Expect(k8sClient.Status().Update(ctx, createdHasComp)).Should(Succeed())

			createdHasComp.Spec.ContainerImage = "test"
			Expect(k8sClient.Update(ctx, createdHasComp)).Should(Succeed())

			updatedHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, updatedHasComp)
				return updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Type == "Updated"
			}, timeout, interval).Should(BeTrue())

			errCondition := updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1]
			Expect(errCondition.Status).Should(Equal(metav1.ConditionFalse))
			Expect(errCondition.Message).Should(ContainSubstring("cannot unmarshal string into Go value of type map[string]interface"))

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

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
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

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
			createdHasApp := &appstudiov1alpha1.Application{}
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0
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
					ComponentName:  ComponentName,
					Application:    applicationName,
					ContainerImage: "quay.io/test/testimage:latest",
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

			// Look up the component resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 1 && createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Status == metav1.ConditionFalse
			}, timeout, interval).Should(BeTrue())

			errCondition := createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1]
			Expect(errCondition.Status).Should(Equal(metav1.ConditionFalse))
			Expect(errCondition.Message).Should(ContainSubstring("Unable to generate gitops resources"))

			// ToDo: Add helper func for accessing the status conditions in a better way
			gitopsCondition := createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-2]
			Expect(gitopsCondition.Type).To(Equal("GitOpsResourcesGenerated"))
			Expect(gitopsCondition.Status).To(Equal(metav1.ConditionFalse))

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
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
					ComponentName:  ComponentName,
					Application:    applicationName,
					ContainerImage: "quay.io/test/testimage:latest",
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

			// Look up the component resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Remove the gitops status
			createdHasComp.Status.GitOps = appstudiov1alpha1.GitOpsStatus{}
			Expect(k8sClient.Status().Update(ctx, createdHasComp)).Should(Succeed())

			// Trigger a new reconcile
			createdHasComp.Spec.ContainerImage = "Newimage"
			Expect(k8sClient.Update(ctx, createdHasComp)).Should(Succeed())

			updatedHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, updatedHasComp)
				return updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Type == "Updated" && updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Status == metav1.ConditionFalse
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
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			var err error
			ctx := context.Background()

			applicationName := HASAppName + "10"
			componentName := HASCompName + "10"

			hasApp := createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)
			curDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{
				Data: []byte(hasApp.Status.Devfile),
			})
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
					ComponentName:  ComponentName,
					Application:    applicationName,
					ContainerImage: "quay.io/test/testimage:latest",
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

			// Look up the component resource that was created.
			// num(conditions) may still be < 1 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Status == metav1.ConditionFalse
			}, timeout, interval).Should(BeTrue())

			Expect(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Message).Should(ContainSubstring("unable to retrieve GitOps repository from Application CR devfile"))

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) == beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) > beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with invalid git url", func() {
		It("Should fail with error", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "11"
			componentName := HASCompName + "11"

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
								URL: "http://fds df &#%&%*$ jdnc/\\",
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

			// Confirm user error hasn't increase the failure metrics
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with non-exist git url", func() {
		It("Should fail with error", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			beforeImportGitRepoTotalReqs := testutil.ToFloat64(metrics.ImportGitRepoTotalReqs)
			beforeImportGitRepoSucceeded := testutil.ToFloat64(metrics.ImportGitRepoSucceeded)
			beforeImportGitRepoFailed := testutil.ToFloat64(metrics.ImportGitRepoFailed)

			applicationName := HASAppName + "-test-import-user-error"
			componentName := HASCompName + "-test-import-user-error"

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
								URL:      "http://github.com/non-exist-git-repo",
								Revision: "main",
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
			errCondition := createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1]
			Expect(errCondition.Status).Should(Equal(metav1.ConditionFalse))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}

			Expect(testutil.ToFloat64(metrics.ImportGitRepoTotalReqs) > beforeImportGitRepoTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.ImportGitRepoSucceeded) > beforeImportGitRepoSucceeded).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.ImportGitRepoFailed) == beforeImportGitRepoFailed).To(BeTrue())

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with invalid devfile url", func() {
		It("Should fail with error that devfile couldn't be unmarshalled", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "12"
			componentName := HASCompName + "12"

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
								URL:        SampleRepoLink,
								DevfileURL: "https://gist.githubusercontent.com/johnmcollier/f322819abaef77a4646a5d27279acb1a/raw/04cfa05bdd8a2f960fffd3cb2fe007efd597f059/component.yaml",
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
			errCondition := createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1]
			Expect(errCondition.Status).Should(Equal(metav1.ConditionFalse))
			Expect(errCondition.Message).Should(ContainSubstring("schemaVersion not present in devfile"))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	// Private Git Repo tests
	Context("Create Component with git secret field set to non-existent secret", func() {
		It("Should error out since the secret doesn't exist", func() {
			ctx := context.Background()

			applicationName := HASAppName + "13"
			componentName := HASCompName + "13"

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
					Secret:        "fake-secret",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL:        SampleRepoLink,
								DevfileURL: "https://github.com/test/repo",
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
			Expect(strings.ToLower(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Message)).Should(ContainSubstring("component create failed: secret \"fake-secret\" not found"))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with git secret field set to an invalid secret", func() {
		It("Should error out due parse error", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			// the secret exists but it's not a real one that we can use to access a live repo
			ctx := context.Background()

			applicationName := HASAppName + "14"
			componentName := HASCompName + "14"

			// Create a git secret
			tokenSecret := &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind: "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: HASAppNamespace,
				},
				StringData: map[string]string{
					"password": "sometoken",
				},
			}

			Expect(k8sClient.Create(ctx, tokenSecret)).Should(Succeed())

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
					Secret:        componentName,
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
			// num(conditions) may still be < 2 on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) == 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the err was set
			Expect(createdHasComp.Status.Devfile).Should(Equal(""))
			Expect(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Status).Should(Equal(metav1.ConditionFalse))
			// This test case uses an invalid token with a public URL.  The Devfile library expects an unset token and will error out trying to retrieve the devfile since it assumes it's from a private repo
			Expect(strings.ToLower(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Message)).Should(ContainSubstring("error getting devfile info from url: failed to retrieve"))
			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) == beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) > beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with private repo, but no devfile", func() {
		It("Should error out since no devfile exists", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "15"
			componentName := HASCompName + "15"

			// Create a git secret
			tokenSecret := &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind: "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: HASAppNamespace,
				},
				StringData: map[string]string{
					"password": "sometoken",
				},
			}

			Expect(k8sClient.Create(ctx, tokenSecret)).Should(Succeed())

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
					Secret:        componentName,
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "https://github.com/devfile-resources/test-error-response",
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
			Expect(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Status).Should(Equal(metav1.ConditionFalse))
			Expect(strings.ToLower(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Message)).Should(ContainSubstring("component create failed: unable to get default branch of github repo"))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) == beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) > beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with with context folder containing no devfile", func() {
		It("Should error out because a devfile cannot be found", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "16"
			componentName := HASCompName + "16"

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
								URL:     "https://github.com/devfile-samples/devfile-sample-python-basic",
								Context: "docker",
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
			Expect(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Reason).Should(Equal("Error"))
			Expect(strings.ToLower(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Message)).Should(ContainSubstring("unable to find devfile in the specified location https://raw.githubusercontent.com/devfile-samples/devfile-sample-python-basic/main/docker"))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with basic field set and test updates to replicas", func() {
		It("Should complete successfully", func() {
			ctx := context.Background()

			applicationName := HASAppName + "17"
			componentName := HASCompName + "17"

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
					ComponentName:  ComponentName,
					Application:    applicationName,
					ContainerImage: "an-image",
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 2 (GitOpsGenerated, Created) on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 1
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			// Make sure the component resource has been updated properly
			Expect(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Message).Should(ContainSubstring("successfully created"))
			Expect(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Reason).Should(Equal("OK"))

			//If replica is unset upon creation, then it should be nil
			Expect(createdHasComp.Spec.Replicas).Should(BeNil())

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return strings.Contains(createdHasApp.Status.Devfile, "containerImage/backend")
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Application
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))
			Expect(createdHasApp.Status.Devfile).Should(ContainSubstring("containerImage/backend"))

			// Trigger a new reconcile that is not related to the replica
			createdHasComp.Spec.ContainerImage = "Newimage"
			Expect(k8sClient.Update(ctx, createdHasComp)).Should(Succeed())

			updatedHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, updatedHasComp)
				return len(updatedHasComp.Status.Conditions) > 1 && updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Type == "Updated" && updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Status == metav1.ConditionTrue
			}, timeout, interval).Should(BeTrue())

			Expect(updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Status).Should(Equal(metav1.ConditionTrue))
			//replica should remain nil
			Expect(createdHasComp.Spec.Replicas).Should(BeNil())

			//Update replica
			updatedHasComp.Spec.Replicas = &oneReplica
			Expect(k8sClient.Update(ctx, updatedHasComp)).Should(Succeed())
			newUpdatedHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, newUpdatedHasComp)
				return len(newUpdatedHasComp.Status.Conditions) > 1 && newUpdatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Type == "Updated" && newUpdatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Status == metav1.ConditionTrue
			}, timeout, interval).Should(BeTrue())

			Expect(newUpdatedHasComp.Status.Conditions[len(newUpdatedHasComp.Status.Conditions)-1].Status).Should(Equal(metav1.ConditionTrue))
			//replica should not be nil and should have a value
			Expect(newUpdatedHasComp.Spec.Replicas).Should(Not(BeNil()))
			Expect(*newUpdatedHasComp.Spec.Replicas).Should(Equal(oneReplica))

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with Dockerfile URL set", func() {
		It("Should create successfully and update the Application", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "18"
			componentName := HASCompName + "18"

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
								URL:           SampleRepoLink,
								Context:       "context",
								DockerfileURL: "http://dockerfile.uri",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the component resource that was created.
			// num(conditions) may still be < 2 (GitOpsGenerated, Created) on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 1 && createdHasComp.Status.GitOps.RepositoryURL != ""
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
			hasCompDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasComp.Status.Devfile)})

			Expect(err).Should(Not(HaveOccurred()))

			dockerfileComponents, err := hasCompDevfile.GetComponents(common.DevfileOptions{})
			Expect(err).Should(Not(HaveOccurred()))
			Expect(len(dockerfileComponents)).Should(Equal(2))

			for _, component := range dockerfileComponents {
				Expect(component.Name).Should(BeElementOf([]string{"dockerfile-build", "kubernetes-deploy"}))
				if component.Image != nil && component.Image.Dockerfile != nil {
					Expect(component.Image.Dockerfile.Uri).Should(Equal(hasComp.Spec.Source.GitSource.DockerfileURL))
					Expect(component.Image.Dockerfile.BuildContext).Should(Equal("./"))
				} else if component.Kubernetes != nil {
					Expect(component.Kubernetes.Inlined).Should(ContainSubstring("Deployment"))
				}
			}

			// Check the HAS Application devfile
			hasAppDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasApp.Status.Devfile)})
			Expect(err).Should(Not(HaveOccurred()))

			// gitOpsRepo and appModelRepo should both be set
			Expect(string(hasAppDevfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(Equal("")))
			Expect(string(hasAppDevfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(Not(Equal("")))

			// gitOpsRepo set in the component equal the repository in the app cr's devfile
			gitopsRepo := hasAppDevfile.GetMetadata().Attributes.GetString("gitOpsRepository.url", &err)
			Expect(err).Should(Not(HaveOccurred()))
			Expect(string(createdHasComp.Status.GitOps.RepositoryURL)).Should(Equal(gitopsRepo))

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

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with setGitOpsGeneration to true", func() {
		It("Should create successfully and not create the GitOps resources, and generate the resources once set.", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "19"
			componentName := HASCompName + "19"

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			comp := &appstudiov1alpha1.Component{
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
					SkipGitOpsResourceGeneration: true,
				},
			}
			Expect(k8sClient.Create(ctx, comp)).Should(Succeed())

			// Look up the component resource that was created.
			// num(conditions) should be 1, and should only contain the "Created" condition.
			compLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), compLookupKey, createdComp)
				return len(createdComp.Status.Conditions) == 1 && createdComp.Status.Conditions[0].Type == "Created" && createdComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			Expect(createdComp.Spec.SkipGitOpsResourceGeneration).To(Equal(createdComp.Status.GitOps.ResourceGenerationSkipped))

			// Make sure the devfile model was properly set in Component
			Expect(createdComp.Status.Devfile).Should(Not(Equal("")))

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Now change skipGitOpsResourceGeneration to true and validate that the GitOps Resources are generated successfully (by validating the GitOpsResourcesGenerated status condition)
			createdComp.Spec.SkipGitOpsResourceGeneration = false
			Expect(k8sClient.Update(ctx, createdComp)).Should(Succeed())

			// Refetch the component and validate that the GitOps resources were created successfully
			updatedComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), compLookupKey, updatedComp)
				return len(updatedComp.Status.Conditions) > 2 && updatedComp.Status.Conditions[2].Type == "Updated" && updatedComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			Expect(updatedComp.Spec.SkipGitOpsResourceGeneration).To(Equal(updatedComp.Status.GitOps.ResourceGenerationSkipped))
			gitOpsCondition := updatedComp.Status.Conditions[1]
			Expect(gitOpsCondition.Type).To(Equal("GitOpsResourcesGenerated"))
			Expect(gitOpsCondition.Status).To(Equal(metav1.ConditionTrue))

			// Delete the specified HASComp resource
			deleteHASCompCR(compLookupKey)

			// Delete the specified HASApp resource
			appLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			deleteHASAppCR(appLookupKey)
		})
	})

	Context("Create Component with Dockerfile URL set for repo with devfile URL", func() {
		It("Should create successfully and override local Dockerfile URL references in the Devfile", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "20"
			componentName := HASCompName + "20"

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
								URL:           "https://github.com/devfile-resources/node-express-hello-no-devfile",
								DevfileURL:    "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/devfile.yaml",
								DockerfileURL: "https://raw.githubusercontent.com/nodeshift-starters/devfile-sample/main/Dockerfile",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the component resource that was created.
			// num(conditions) may still be < 2 (GitOpsGenerated, Created) on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 1 && createdHasComp.Status.GitOps.RepositoryURL != ""
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
			hasCompDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasComp.Status.Devfile)})
			Expect(err).Should(Not(HaveOccurred()))

			devfileComponents, err := hasCompDevfile.GetComponents(common.DevfileOptions{})
			Expect(err).Should(Not(HaveOccurred()))
			Expect(len(devfileComponents)).Should(Equal(3))

			for _, component := range devfileComponents {
				Expect(component.Name).Should(BeElementOf([]string{"image-build", "kubernetes-deploy", "runtime"}))
				if component.Image != nil && component.Image.Dockerfile != nil {
					Expect(component.Image.Dockerfile.Uri).Should(Equal(hasComp.Spec.Source.GitSource.DockerfileURL))
				}
			}

			// Check the HAS Application devfile
			hasAppDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasApp.Status.Devfile)})

			Expect(err).Should(Not(HaveOccurred()))

			// gitOpsRepo and appModelRepo should both be set
			Expect(string(hasAppDevfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(Equal("")))
			Expect(string(hasAppDevfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(Not(Equal("")))

			// gitOpsRepo set in the component equal the repository in the app cr's devfile
			gitopsRepo := hasAppDevfile.GetMetadata().Attributes.GetString("gitOpsRepository.url", &err)
			Expect(err).Should(Not(HaveOccurred()))
			Expect(string(createdHasComp.Status.GitOps.RepositoryURL)).Should(Equal(gitopsRepo))

			hasProjects, err := hasAppDevfile.GetProjects(common.DevfileOptions{})
			Expect(err).Should(Not(HaveOccurred()))
			Expect(len(hasProjects)).ShouldNot(Equal(0))

			nameMatched := false
			repoLinkMatched := false
			for _, project := range hasProjects {
				if project.Name == ComponentName {
					nameMatched = true
				}
				if project.Git != nil && project.Git.GitLikeProjectSource.Remotes["origin"] == "https://github.com/devfile-resources/node-express-hello-no-devfile" {
					repoLinkMatched = true
				}
			}
			Expect(nameMatched).Should(Equal(true))
			Expect(repoLinkMatched).Should(Equal(true))

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	// Cannot test combined failure and recovery scenario since mock test uses the component name which can't be changed
	Context("Component with empty GitOps repository", func() {
		It("Should error out", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "21"
			componentName := HASCompName + "test-git-error" + "21"

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			fetchedHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, fetchedHasApp)
				return fetchedHasApp.Status.Devfile != ""
			}, timeout, interval).Should(BeTrue())

			hasAppDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(fetchedHasApp.Status.Devfile)})
			Expect(err).Should(Not(HaveOccurred()))

			// Update the GitOps Repo to a URI that mocked API returns a dummy err
			hasAppDevfile.GetMetadata().Attributes.PutString("gitOpsRepository.url", "https://github.com/devfile-resources/test-error-response")

			devfileYaml, err := yaml.Marshal(hasAppDevfile)
			Expect(err).ToNot(HaveOccurred())
			fetchedHasApp.Status.Devfile = string(devfileYaml)
			Expect(k8sClient.Status().Update(ctx, fetchedHasApp)).Should(Succeed())

			fetchedHasApp = &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, fetchedHasApp)
				hasAppDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(fetchedHasApp.Status.Devfile)})
				Expect(err).Should(Not(HaveOccurred()))
				gitOpsRepoURL := hasAppDevfile.GetMetadata().Attributes.GetString("gitOpsRepository.url", &err)
				Expect(err).Should(Not(HaveOccurred()))
				return gitOpsRepoURL == "https://github.com/devfile-resources/test-error-response"
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
					Route: "oldroute",
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				var gitOpsRepCheck, createConditionCheck, gitOpsConditionCheck bool
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				if createdHasComp.Status.GitOps.RepositoryURL == "https://github.com/devfile-resources/test-error-response" {
					gitOpsRepCheck = true
				}
				for _, condition := range createdHasComp.Status.Conditions {
					if condition.Type == "Created" && condition.Status == metav1.ConditionFalse {
						createConditionCheck = true
					}
					if condition.Type == "GitOpsResourcesGenerated" && condition.Status == metav1.ConditionFalse {
						gitOpsConditionCheck = true
					}
				}
				return gitOpsRepCheck && createConditionCheck && gitOpsConditionCheck
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Update the devfile to empty to see if errors out
			createdHasComp.Status.Devfile = ""
			Expect(k8sClient.Status().Update(ctx, createdHasComp)).Should(Succeed())
			createdHasComp = &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				var devfileCheck, createConditionCheck, gitOpsConditionCheck bool
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				if createdHasComp.Status.Devfile == "" {
					devfileCheck = true
				}
				for _, condition := range createdHasComp.Status.Conditions {
					if condition.Type == "Created" && condition.Status == metav1.ConditionFalse {
						createConditionCheck = true
					}
					if condition.Type == "GitOpsResourcesGenerated" && condition.Status == metav1.ConditionFalse && strings.Contains(condition.Message, "cannot parse devfile without a src") {
						gitOpsConditionCheck = true
					}
				}
				return devfileCheck && createConditionCheck && gitOpsConditionCheck
			}, timeout, interval).Should(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})
	Context("Component with valid GitOps repository", func() {
		It("Should successfully update CR conditions and status", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "22"
			componentName := HASCompName + "22"

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			fetchedHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, fetchedHasApp)
				return fetchedHasApp.Status.Devfile != ""
			}, timeout, interval).Should(BeTrue())

			hasAppDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{
				Data: []byte(fetchedHasApp.Status.Devfile),
			})
			Expect(err).Should(Not(HaveOccurred()))

			// Update the GitOps Repo to a URI that mocked API returns a dummy err
			hasAppDevfile.GetMetadata().Attributes.PutString("gitOpsRepository.url", "https://github.com/devfile-resources/test-no-error")

			devfileYaml, err := yaml.Marshal(hasAppDevfile)
			Expect(err).ToNot(HaveOccurred())
			fetchedHasApp.Status.Devfile = string(devfileYaml)
			Expect(k8sClient.Status().Update(ctx, fetchedHasApp)).Should(Succeed())

			fetchedHasApp = &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, fetchedHasApp)
				hasAppDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{
					Data: []byte(fetchedHasApp.Status.Devfile),
				})
				Expect(err).Should(Not(HaveOccurred()))
				gitOpsRepoURL := hasAppDevfile.GetMetadata().Attributes.GetString("gitOpsRepository.url", &err)
				Expect(err).Should(Not(HaveOccurred()))
				return gitOpsRepoURL == "https://github.com/devfile-resources/test-no-error"
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
					Route: "oldroute",
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				var gitOpsRepCheck, createConditionCheck, gitOpsConditionCheck bool
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				if createdHasComp.Status.GitOps.RepositoryURL == "https://github.com/devfile-resources/test-no-error" {
					gitOpsRepCheck = true
				}
				for _, condition := range createdHasComp.Status.Conditions {
					if condition.Type == "Created" && condition.Status == metav1.ConditionTrue {
						createConditionCheck = true
					}
					if condition.Type == "GitOpsResourcesGenerated" && condition.Status == metav1.ConditionTrue {
						gitOpsConditionCheck = true
					}
				}

				return gitOpsRepCheck && createConditionCheck && gitOpsConditionCheck
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})
	Context("force generate gitops resource", func() {
		It("Should successfully update CR conditions and status", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "23"
			componentName := HASCompName + "23"

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
			// num(conditions) may still be < 2 (GeneratedGitOps, Created) on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 1 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())
			// Verify that the GitOpsGenerated status condition was also set
			gitopsCondition := createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-2]
			Expect(gitopsCondition.Type).To(Equal("GitOpsResourcesGenerated"))
			Expect(gitopsCondition.Status).To(Equal(metav1.ConditionTrue))

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// set the annotation and update a spec to force enter the reconcile
			setForceGenerateGitopsAnnotation(createdHasComp, "true")
			createdHasComp.Spec.TargetPort = 1111
			Expect(k8sClient.Update(ctx, createdHasComp)).Should(Succeed())

			createdHasComp = &appstudiov1alpha1.Component{}
			// Verify that the GitOpsResourcesForceGenerated status condition was set
			Eventually(func() bool {
				var gitOpsForceGenerateCheck bool

				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				for _, condition := range createdHasComp.Status.Conditions {
					if condition.Type == "GitOpsResourcesForceGenerated" && condition.Status == metav1.ConditionTrue {
						gitOpsForceGenerateCheck = true
					}
				}
				return gitOpsForceGenerateCheck
			}, timeout, interval).Should(BeTrue())
			gitopsCondition = createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1]
			Expect(gitopsCondition.Type).To(Equal("GitOpsResourcesForceGenerated"))
			Expect(gitopsCondition.Status).To(Equal(metav1.ConditionTrue))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Private Component with basic field set", func() {
		It("Should create successfully and update the Application", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "24"
			componentName := HASCompName + "24"

			// Create a git secret
			tokenSecret := &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind: "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: HASAppNamespace,
				},
				StringData: map[string]string{
					"password": "valid-token", // token tied to mock implementation in devfile/library. See https://github.com/devfile/library/blob/main/pkg/util/mock.go#L250
				},
			}

			Expect(k8sClient.Create(ctx, tokenSecret)).Should(Succeed())

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
					Secret:        componentName,
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "https://github.com/devfile-resources/devfile-sample-python-basic-private", // It doesn't matter if we are using pub/pvt repo here. We are mock testing the token, "valid-token" returns a mock devfile. See https://github.com/devfile/library/blob/main/pkg/util/mock.go#L250
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 2 (GeneratedGitOps, Created) on the first try, so retry until at least _some_ condition is set
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 1 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Verify that the GitOpsGenerated status condition was also set
			// ToDo: Add helper func for accessing the status conditions in a better way
			gitopsCondition := createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-2]
			Expect(gitopsCondition.Type).To(Equal("GitOpsResourcesGenerated"))
			Expect(gitopsCondition.Status).To(Equal(metav1.ConditionTrue))

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
			_, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasComp.Status.Devfile)})
			Expect(err).Should(Not(HaveOccurred()))

			// Check the HAS Application devfile
			hasAppDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasApp.Status.Devfile)})

			Expect(err).Should(Not(HaveOccurred()))

			// gitOpsRepo and appModelRepo should both be set
			Expect(string(hasAppDevfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(Equal("")))
			Expect(string(hasAppDevfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(Not(Equal("")))

			// gitOpsRepo set in the component equal the repository in the app cr's devfile
			gitopsRepo := hasAppDevfile.GetMetadata().Attributes.GetString("gitOpsRepository.url", &err)
			Expect(err).Should(Not(HaveOccurred()))
			Expect(string(createdHasComp.Status.GitOps.RepositoryURL)).Should(Equal(gitopsRepo))

			// Commit ID should be set in the gitops repository and not be empty
			Expect(createdHasComp.Status.GitOps.CommitID).Should(Not(BeEmpty()))

			hasProjects, err := hasAppDevfile.GetProjects(common.DevfileOptions{})
			Expect(err).Should(Not(HaveOccurred()))
			Expect(len(hasProjects)).ShouldNot(Equal(0))

			nameMatched := false
			repoLinkMatched := false
			for _, project := range hasProjects {
				if project.Name == ComponentName {
					nameMatched = true
				}
				if project.Git != nil && project.Git.GitLikeProjectSource.Remotes["origin"] == "https://github.com/devfile-resources/devfile-sample-python-basic-private" {
					repoLinkMatched = true
				}
			}
			Expect(nameMatched).Should(Equal(true))
			Expect(repoLinkMatched).Should(Equal(true))

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create private and public Components for the same Application with basic field set", func() {
		It("Should create SPI FCR resource and associate it with only the private Component", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "25"
			componentName := HASCompName + "25"
			componentPublicName := HASCompName + "public-25"

			// Create a git secret
			tokenSecret := &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind: "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: HASAppNamespace,
				},
				StringData: map[string]string{
					"password": "valid-token", // token tied to mock implementation in devfile/library. See https://github.com/devfile/library/blob/main/pkg/util/mock.go#L250
				},
			}

			Expect(k8sClient.Create(ctx, tokenSecret)).Should(Succeed())

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasCompPrivate := &appstudiov1alpha1.Component{
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
					Secret:        componentName,
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "http://github.com/dummy/create-spi-fcr-return-devfile",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasCompPrivate)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 2 (GeneratedGitOps, Created) on the first try, so retry until at least _some_ condition is set
			hasCompPrivateLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasPrivateComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompPrivateLookupKey, createdHasPrivateComp)
				return len(createdHasPrivateComp.Status.Conditions) > 1 && createdHasPrivateComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Verify that the GitOpsGenerated status condition was also set
			// ToDo: Add helper func for accessing the status conditions in a better way
			gitopsConditionPrivate := createdHasPrivateComp.Status.Conditions[len(createdHasPrivateComp.Status.Conditions)-2]
			Expect(gitopsConditionPrivate.Type).To(Equal("GitOpsResourcesGenerated"))
			Expect(gitopsConditionPrivate.Status).To(Equal(metav1.ConditionTrue))

			// Make sure the devfile model was properly set in Component
			Expect(createdHasPrivateComp.Status.Devfile).Should(Not(Equal("")))

			hasCompPublic := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentPublicName,
					Namespace: HASAppNamespace,
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "backend2",
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
			Expect(k8sClient.Create(ctx, hasCompPublic)).Should(Succeed())

			// Look up the has app resource that was created.
			// num(conditions) may still be < 2 (GeneratedGitOps, Created) on the first try, so retry until at least _some_ condition is set
			hasCompPublicLookupKey := types.NamespacedName{Name: componentPublicName, Namespace: HASAppNamespace}
			createdHasPublicComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompPublicLookupKey, createdHasPublicComp)
				return len(createdHasPublicComp.Status.Conditions) > 1 && createdHasPublicComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Verify that the GitOpsGenerated status condition was also set
			// ToDo: Add helper func for accessing the status conditions in a better way
			gitopsConditionPublic := createdHasPublicComp.Status.Conditions[len(createdHasPublicComp.Status.Conditions)-2]
			Expect(gitopsConditionPublic.Type).To(Equal("GitOpsResourcesGenerated"))
			Expect(gitopsConditionPublic.Status).To(Equal(metav1.ConditionTrue))

			// Make sure the devfile model was properly set in Component
			Expect(createdHasPublicComp.Status.Devfile).Should(Not(Equal("")))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ComponentName)
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Application
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			// Check both the Component devfile
			_, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasPrivateComp.Status.Devfile)})
			Expect(err).Should(Not(HaveOccurred()))
			_, err = cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasPublicComp.Status.Devfile)})
			Expect(err).Should(Not(HaveOccurred()))

			// check for the SPI FCR that got created for private component, its a mock test client, so the SPI FCR does not get processed besides getting created.
			createdSPIFCR := &spiapi.SPIFileContentRequest{}
			spiFCRQueryLookupKey := types.NamespacedName{Name: "spi-fcr-" + componentName + "0", Namespace: HASAppNamespace}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), spiFCRQueryLookupKey, createdSPIFCR)
				return createdSPIFCR.Spec.RepoUrl != ""
			}, timeout, interval).Should(BeTrue())

			// Check the HAS Application devfile
			hasAppDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasApp.Status.Devfile)})
			Expect(err).Should(Not(HaveOccurred()))

			// gitOpsRepo and appModelRepo should both be set
			Expect(string(hasAppDevfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(Equal("")))
			Expect(string(hasAppDevfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(Not(Equal("")))

			// gitOpsRepo set in the component equal the repository in the app cr's devfile
			gitopsRepo := hasAppDevfile.GetMetadata().Attributes.GetString("gitOpsRepository.url", &err)
			Expect(err).Should(Not(HaveOccurred()))
			Expect(string(createdHasPrivateComp.Status.GitOps.RepositoryURL)).Should(Equal(gitopsRepo))
			Expect(string(createdHasPublicComp.Status.GitOps.RepositoryURL)).Should(Equal(gitopsRepo))

			// Commit ID should be set in the gitops repository and not be empty
			Expect(createdHasPrivateComp.Status.GitOps.CommitID).Should(Not(BeEmpty()))
			Expect(createdHasPublicComp.Status.GitOps.CommitID).Should(Not(BeEmpty()))

			hasProjects, err := hasAppDevfile.GetProjects(common.DevfileOptions{})
			Expect(err).Should(Not(HaveOccurred()))
			Expect(len(hasProjects)).ShouldNot(Equal(0))

			privateNameMatched := false
			privateRepoLinkMatched := false
			publicNameMatched := false
			publicRepoLinkMatched := false
			for _, project := range hasProjects {
				if project.Name == ComponentName {
					privateNameMatched = true
				}
				if project.Git != nil && project.Git.GitLikeProjectSource.Remotes["origin"] == "http://github.com/dummy/create-spi-fcr-return-devfile" {
					privateRepoLinkMatched = true
				}
				if project.Name == "backend2" {
					publicNameMatched = true
				}
				if project.Git != nil && project.Git.GitLikeProjectSource.Remotes["origin"] == SampleRepoLink {
					publicRepoLinkMatched = true
				}
			}
			Expect(privateNameMatched).Should(Equal(true))
			Expect(privateRepoLinkMatched).Should(Equal(true))
			Expect(publicNameMatched).Should(Equal(true))
			Expect(publicRepoLinkMatched).Should(Equal(true))

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) == beforeCreateTotalReqs+2).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) == beforeCreateSucceedReqs+2).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified public HASComp resource
			deleteHASCompCR(hasCompPublicLookupKey)

			// Ensure the SPIFCR that is associated with the private component is still present
			createdSPIFCR = &spiapi.SPIFileContentRequest{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), spiFCRQueryLookupKey, createdSPIFCR)
				return createdSPIFCR.Spec.RepoUrl != ""
			}, timeout, interval).Should(BeTrue())

			// Delete the specified private HASComp resource
			deleteHASCompCR(hasCompPrivateLookupKey)

			// Ensure the SPIFCR that is associate with the private component has owner reference
			// Kube client created with a test environment config does not clean up Kube resources
			// with owner referneces.
			createdSPIFCR = &spiapi.SPIFileContentRequest{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), spiFCRQueryLookupKey, createdSPIFCR)
				ownerRefs := createdSPIFCR.GetOwnerReferences()
				if len(ownerRefs) == 1 {
					if ownerRefs[0].Name == componentName && ownerRefs[0].Kind == "Component" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})

		Context("Create Private Component with basic field set and a private parent uri", func() {
			It("Should create successfully and update the Application", func() {
				beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
				beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
				beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

				ctx := context.Background()

				applicationName := HASAppName + "26"
				componentName := HASCompName + "26"

				originalPort := 1111
				updatedPort := 2222

				// Create a git secret
				tokenSecret := &corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind: "Secret",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      componentName,
						Namespace: HASAppNamespace,
					},
					StringData: map[string]string{
						"password": "parent-devfile", // notsecret - see mock implementation in devfile/library https://github.com/devfile/library/blob/main/pkg/util/mock.go
					},
				}

				Expect(k8sClient.Create(ctx, tokenSecret)).Should(Succeed())

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
						Secret:        componentName,
						Source: appstudiov1alpha1.ComponentSource{
							ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
								GitSource: &appstudiov1alpha1.GitSource{
									URL: "https://github.com/devfile-resources/devfile-sample-python-basic-private", // It doesn't matter if we are using pub/pvt repo here. We are mock testing the token, "parent-devfile" returns a mock devfile and mock parent. See https://github.com/devfile/library/blob/main/pkg/util/mock.go
								},
							},
						},
						TargetPort: originalPort,
					},
				}
				Expect(k8sClient.Create(ctx, hasComp)).Should(Succeed())

				// Look up the has app resource that was created.
				// num(conditions) may still be < 2 (GeneratedGitOps, Created) on the first try, so retry until at least _some_ condition is set
				hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
				createdHasComp := &appstudiov1alpha1.Component{}
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
					return len(createdHasComp.Status.Conditions) > 1 && createdHasComp.Status.GitOps.RepositoryURL != ""
				}, timeout, interval).Should(BeTrue())

				// Verify that the GitOpsGenerated status condition was also set
				// ToDo: Add helper func for accessing the status conditions in a better way
				gitopsCondition := createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-2]
				Expect(gitopsCondition.Type).To(Equal("GitOpsResourcesGenerated"))
				Expect(gitopsCondition.Status).To(Equal(metav1.ConditionTrue))

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
				_, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasComp.Status.Devfile)})
				Expect(err).Should(Not(HaveOccurred()))

				// Check the HAS Application devfile
				hasAppDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasApp.Status.Devfile)})

				Expect(err).Should(Not(HaveOccurred()))

				// gitOpsRepo and appModelRepo should both be set
				Expect(string(hasAppDevfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(Equal("")))
				Expect(string(hasAppDevfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(Not(Equal("")))

				// gitOpsRepo set in the component equal the repository in the app cr's devfile
				gitopsRepo := hasAppDevfile.GetMetadata().Attributes.GetString("gitOpsRepository.url", &err)
				Expect(err).Should(Not(HaveOccurred()))
				Expect(string(createdHasComp.Status.GitOps.RepositoryURL)).Should(Equal(gitopsRepo))

				// Commit ID should be set in the gitops repository and not be empty
				Expect(createdHasComp.Status.GitOps.CommitID).Should(Not(BeEmpty()))

				hasProjects, err := hasAppDevfile.GetProjects(common.DevfileOptions{})
				Expect(err).Should(Not(HaveOccurred()))
				Expect(len(hasProjects)).ShouldNot(Equal(0))

				nameMatched := false
				repoLinkMatched := false
				for _, project := range hasProjects {
					if project.Name == ComponentName {
						nameMatched = true
					}
					if project.Git != nil && project.Git.GitLikeProjectSource.Remotes["origin"] == "https://github.com/devfile-resources/devfile-sample-python-basic-private" {
						repoLinkMatched = true
					}
				}
				Expect(nameMatched).Should(Equal(true))
				Expect(repoLinkMatched).Should(Equal(true))

				Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
				Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
				Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

				// Update Component
				createdHasComp.Spec.TargetPort = updatedPort

				Expect(k8sClient.Update(ctx, createdHasComp)).Should(Succeed())

				updatedHasComp := &appstudiov1alpha1.Component{}
				Eventually(func() bool {
					k8sClient.Get(context.Background(), hasCompLookupKey, updatedHasComp)
					return updatedHasComp.Status.Conditions[len(updatedHasComp.Status.Conditions)-1].Type == "Updated"
				}, timeout, interval).Should(BeTrue())

				// Make sure the devfile model was properly set in Component
				Expect(updatedHasComp.Status.Devfile).Should(Not(Equal("")))

				// Check the Component updated devfile
				hasCompUpdatedDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(updatedHasComp.Status.Devfile)})

				Expect(err).Should(Not(HaveOccurred()))

				checklist := updateChecklist{
					port: updatedPort,
				}

				verifyHASComponentUpdates(hasCompUpdatedDevfile, checklist, nil)

				// Delete the specified HASComp resource
				deleteHASCompCR(hasCompLookupKey)

				// Delete the specified HASApp resource
				deleteHASAppCR(hasAppLookupKey)
			})
		})
	})

	Context("Create private Component for an Application with basic field set", func() {
		It("Should create SPI FCR resource and persist it even though the associated private Component is in an error state", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())
			ctx := context.Background()

			applicationName := HASAppName + "27"
			componentName := HASCompName + "27"

			// Create a git secret
			tokenSecret := &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind: "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: HASAppNamespace,
				},
				StringData: map[string]string{
					"password": "invalid-token", // token tied to mock implementation in devfile/library. See https://github.com/devfile/library/blob/main/pkg/util/mock.go#L250
				},
			}

			Expect(k8sClient.Create(ctx, tokenSecret)).Should(Succeed())

			createAndFetchSimpleApp(applicationName, HASAppNamespace, DisplayName, Description)

			hasCompPrivate := &appstudiov1alpha1.Component{
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
					Secret:        componentName,
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "http://github.com/dummy/create-spi-fcr-return-devfile",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hasCompPrivate)).Should(Succeed())

			// Look up the has app resource that was created.
			hasCompPrivateLookupKey := types.NamespacedName{Name: componentName, Namespace: HASAppNamespace}
			createdHasPrivateComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompPrivateLookupKey, createdHasPrivateComp)
				return len(createdHasPrivateComp.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Make sure the err was set
			Expect(createdHasPrivateComp.Status.Devfile).Should(Equal(""))
			Expect(createdHasPrivateComp.Status.Conditions[len(createdHasPrivateComp.Status.Conditions)-1].Reason).Should(Equal("Error"))
			Expect(strings.ToLower(createdHasPrivateComp.Status.Conditions[len(createdHasPrivateComp.Status.Conditions)-1].Message)).Should(ContainSubstring("error getting devfile"))

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) == beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) > beforeCreateFailedReqs).To(BeTrue())

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ComponentName)
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Application
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			// Check the HAS Application devfile
			hasAppDevfile, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(createdHasApp.Status.Devfile)})
			Expect(err).Should(Not(HaveOccurred()))

			// gitOpsRepo and appModelRepo should both be set
			Expect(string(hasAppDevfile.GetMetadata().Attributes["gitOpsRepository.url"].Raw)).Should(Not(Equal("")))
			Expect(string(hasAppDevfile.GetMetadata().Attributes["appModelRepository.url"].Raw)).Should(Not(Equal("")))

			// check for the SPI FCR that got created for private component, its a mock test client, so the SPI FCR does not get processed besides getting created.
			createdSPIFCR := &spiapi.SPIFileContentRequest{}
			spiFCRQueryLookupKey := types.NamespacedName{Name: "spi-fcr-" + componentName + "0", Namespace: HASAppNamespace}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), spiFCRQueryLookupKey, createdSPIFCR)
				return createdSPIFCR.Spec.RepoUrl != ""
			}, timeout, interval).Should(BeTrue())

			// Delete the specified private HASComp resource
			deleteHASCompCR(hasCompPrivateLookupKey)

			// Ensure the SPIFCR that is associate with the private component has owner reference
			// Kube client created with a test environment config does not clean up Kube resources
			// with owner referneces.
			createdSPIFCR = &spiapi.SPIFileContentRequest{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), spiFCRQueryLookupKey, createdSPIFCR)
				ownerRefs := createdSPIFCR.GetOwnerReferences()
				if len(ownerRefs) == 1 {
					if ownerRefs[0].Name == componentName && ownerRefs[0].Kind == "Component" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})

	Context("Create Component with basic field set including devfileURL", func() {
		It("Should error out on a devfile that has incompatible data and mark it as an user error on the metrics", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "28"
			componentName := HASCompName + "28"

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
								URL:        SampleRepoLink,
								DevfileURL: "https://raw.githubusercontent.com/maysunfaisal/devfile-sample-go-basic-placeholder/main/devfile.yaml",
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
			Expect(strings.ToLower(createdHasComp.Status.Conditions[len(createdHasComp.Status.Conditions)-1].Message)).Should(ContainSubstring("error unmarshaling"))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

			// Delete the specified HASComp resource
			deleteHASCompCR(hasCompLookupKey)

			// Delete the specified HASApp resource
			deleteHASAppCR(hasAppLookupKey)
		})
	})
	Context("Create component having git source from gitlab", func() {
		It("Should not increase the component failure metrics", func() {
			beforeCreateTotalReqs := testutil.ToFloat64(metrics.GetComponentCreationTotalReqs())
			beforeCreateSucceedReqs := testutil.ToFloat64(metrics.GetComponentCreationSucceeded())
			beforeCreateFailedReqs := testutil.ToFloat64(metrics.GetComponentCreationFailed())

			ctx := context.Background()

			applicationName := HASAppName + "30"
			componentName := HASCompName + "30"

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
								URL: SampleGitlabRepoLink,
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
				k8sClient.Get(ctx, hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: HASAppNamespace}

			Expect(testutil.ToFloat64(metrics.GetComponentCreationTotalReqs()) > beforeCreateTotalReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationSucceeded()) > beforeCreateSucceedReqs).To(BeTrue())
			Expect(testutil.ToFloat64(metrics.GetComponentCreationFailed()) == beforeCreateFailedReqs).To(BeTrue())

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
			ComponentType: v1alpha2.KubernetesComponentType,
		},
	})
	if goPkgTest == nil {
		Expect(err).Should(Not(HaveOccurred()))
	} else if err != nil {
		goPkgTest.Error(err)
	}

	requests := checklist.resources.Requests
	limits := checklist.resources.Limits

	for _, component := range components {
		componentAttributes := component.Attributes
		var err error

		// Check the route
		if checklist.route != "" {
			route := componentAttributes.Get(devfilePkg.RouteKey, &err)
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
			replicas := componentAttributes.Get(devfilePkg.ReplicaKey, &err)
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
			storageLimit := componentAttributes.Get(devfilePkg.StorageLimitKey, &err)
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
			storageRequest := componentAttributes.Get(devfilePkg.StorageRequestKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(storageRequest).Should(Equal(storageRequestChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if storageRequest.(string) != storageRequestChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", storageRequestChecklist.String(), storageRequest)
			}
		}

		// Check the memory limit
		if _, ok := limits[corev1.ResourceMemory]; ok {
			memoryLimitChecklist := limits[corev1.ResourceMemory]
			memoryLimit := componentAttributes.Get(devfilePkg.MemoryLimitKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(memoryLimit.(string)).Should(Equal(memoryLimitChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if memoryLimit.(string) != memoryLimitChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", memoryLimitChecklist.String(), memoryLimit)
			}
		}

		// Check the memory request
		if _, ok := requests[corev1.ResourceMemory]; ok {
			memoryRequestChecklist := requests[corev1.ResourceMemory]
			memoryRequest := componentAttributes.Get(devfilePkg.MemoryRequestKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(memoryRequest).Should(Equal(memoryRequestChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if memoryRequest.(string) != memoryRequestChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", memoryRequestChecklist.String(), memoryRequest)
			}
		}

		// Check the cpu limit
		if _, ok := limits[corev1.ResourceCPU]; ok {
			cpuLimitChecklist := limits[corev1.ResourceCPU]
			cpuLimit := componentAttributes.Get(devfilePkg.CpuLimitKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(cpuLimit).Should(Equal(cpuLimitChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if cpuLimit.(string) != cpuLimitChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", cpuLimitChecklist.String(), cpuLimit)
			}
		}

		// Check the cpu request
		if _, ok := requests[corev1.ResourceCPU]; ok {
			cpuRequestChecklist := requests[corev1.ResourceCPU]
			cpuRequest := componentAttributes.Get(devfilePkg.CpuRequestKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(cpuRequest).Should(Equal(cpuRequestChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if cpuRequest.(string) != cpuRequestChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", cpuRequestChecklist.String(), cpuRequest)
			}
		}

		// Check for container port
		if checklist.port != 0 {
			containerPort := componentAttributes.Get(devfilePkg.ContainerImagePortKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(containerPort).Should(Equal(float64(checklist.port)))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if int(containerPort.(float64)) != checklist.port {
				goPkgTest.Errorf("expected: %v, got: %v", checklist.port, containerPort)
			}
		}
		// Check for env
		for _, checklistEnv := range checklist.env {
			isMatched := false
			var containerENVs []corev1.EnvVar
			err := componentAttributes.GetInto(devfilePkg.ContainerENVKey, &containerENVs)
			for _, containerEnv := range containerENVs {
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
