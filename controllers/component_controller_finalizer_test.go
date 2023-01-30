//
// Copyright 2022-2023 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Application controller finalizer counter tests", func() {

	const (
		AppName        = "test-application-finalizer"
		CompName       = "test-component-finalizer"
		AppNamespace   = "default"
		DisplayName    = "petclinic"
		Description    = "Simple petclinic app"
		ComponentName  = "backend"
		SampleRepoLink = "https://github.com/devfile-samples/devfile-sample-java-springboot-basic"
	)

	Context("Delete Component CR with an invalid Application devfile", func() {
		It("Should delete successfully even when finalizer fails", func() {
			applicationName := AppName + "1"
			componentName := CompName + "1"

			// Create a simple Application CR and get its devfile
			createAndFetchSimpleApp(applicationName, AppNamespace, DisplayName, Description)

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: AppNamespace,
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
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: AppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: AppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ComponentName)
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Application
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			createdHasApp.Status.Devfile = "a"
			Expect(k8sClient.Status().Update(context.Background(), createdHasApp)).Should(Succeed())

			// Get the updated resource
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				// Return true if the most recent condition on the CR is updated
				return createdHasApp.Status.Devfile == "a"
			}, timeout, interval).Should(BeTrue())

			// Delete the Component resource
			Eventually(func() error {
				return k8sClient.Delete(context.Background(), createdHasComp)
			}, timeout, interval).Should(Succeed())

			// Wait for delete to finish
			Eventually(func() error {
				f := &appstudiov1alpha1.Component{}
				return k8sClient.Get(context.Background(), hasCompLookupKey, f)
			}, timeout, interval).ShouldNot(Succeed())

			// Delete the Application resource
			Eventually(func() error {
				return k8sClient.Delete(context.Background(), createdHasApp)
			}, timeout, interval).Should(Succeed())

			// Wait for delete to finish
			Eventually(func() error {
				f := &appstudiov1alpha1.Application{}
				return k8sClient.Get(context.Background(), hasAppLookupKey, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})

	Context("Delete Component CR with project missing in the Application devfile", func() {
		It("Should delete successfully even when finalizer fails", func() {
			applicationName := AppName + "2"
			componentName := CompName + "2"

			// Create a simple Application CR and get its devfile
			createAndFetchSimpleApp(applicationName, AppNamespace, DisplayName, Description)

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: AppNamespace,
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
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: AppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: AppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ComponentName)
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Application
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			// delete the project so that the component delete finalizer fails
			devfileSrc := devfile.DevfileSrc{
				Data: createdHasApp.Status.Devfile,
			}
			appDevfile, err := devfile.ParseDevfile(devfileSrc)
			Expect(err).ToNot(HaveOccurred())

			err = appDevfile.DeleteProject(ComponentName)
			Expect(err).ToNot(HaveOccurred())

			appDevfileYaml, err := yaml.Marshal(appDevfile)
			Expect(err).ToNot(HaveOccurred())

			createdHasApp.Status.Devfile = string(appDevfileYaml)
			Expect(k8sClient.Status().Update(context.Background(), createdHasApp)).Should(Succeed())

			// Get the updated resource
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				// Return true if the most recent condition on the CR is updated
				return !strings.Contains(createdHasApp.Status.Devfile, ComponentName)
			}, timeout, interval).Should(BeTrue())

			// Delete the Component resource
			Eventually(func() error {
				return k8sClient.Delete(context.Background(), createdHasComp)
			}, timeout, interval).Should(Succeed())

			// Wait for delete to finish
			Eventually(func() error {
				f := &appstudiov1alpha1.Component{}
				return k8sClient.Get(context.Background(), hasCompLookupKey, f)
			}, timeout, interval).ShouldNot(Succeed())

			// Delete the Application resource
			Eventually(func() error {
				return k8sClient.Delete(context.Background(), createdHasApp)
			}, timeout, interval).Should(Succeed())

			// Wait for delete to finish
			Eventually(func() error {
				f := &appstudiov1alpha1.Application{}
				return k8sClient.Get(context.Background(), hasAppLookupKey, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})

	Context("Delete Component CR with missing gitops repo url", func() {
		It("Should delete successfully even when finalizer fails", func() {
			applicationName := AppName + "3"
			componentName := CompName + "3"

			// Create a simple Application CR and get its devfile
			createAndFetchSimpleApp(applicationName, AppNamespace, DisplayName, Description)

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: AppNamespace,
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
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: AppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 1 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: AppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ComponentName)
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Application
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			createdHasComp.Status.GitOps.RepositoryURL = ""
			Expect(k8sClient.Status().Update(context.Background(), createdHasComp)).Should(Succeed())

			// Get the updated resource
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				// Return true if the most recent condition on the CR is updated
				return createdHasComp.Status.GitOps.RepositoryURL == ""
			}, timeout, interval).Should(BeTrue())

			// Delete the Component resource
			Eventually(func() error {
				return k8sClient.Delete(context.Background(), createdHasComp)
			}, timeout, interval).Should(Succeed())

			// Wait for delete to finish
			Eventually(func() error {
				f := &appstudiov1alpha1.Component{}
				return k8sClient.Get(context.Background(), hasCompLookupKey, f)
			}, timeout, interval).ShouldNot(Succeed())

			// Delete the Application resource
			Eventually(func() error {
				return k8sClient.Delete(context.Background(), createdHasApp)
			}, timeout, interval).Should(Succeed())

			// Wait for delete to finish
			Eventually(func() error {
				f := &appstudiov1alpha1.Application{}
				return k8sClient.Get(context.Background(), hasAppLookupKey, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})

	Context("Delete Component CR with specified git branch and context", func() {
		It("Should delete successfully", func() {
			applicationName := AppName + "4"
			componentName := CompName + "4"

			// Create a simple Application CR and get its devfile
			createAndFetchSimpleApp(applicationName, AppNamespace, DisplayName, Description)

			hasComp := &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      componentName,
					Namespace: AppNamespace,
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
			hasCompLookupKey := types.NamespacedName{Name: componentName, Namespace: AppNamespace}
			createdHasComp := &appstudiov1alpha1.Component{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				return len(createdHasComp.Status.Conditions) > 0 && createdHasComp.Status.GitOps.RepositoryURL != ""
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Component
			Expect(createdHasComp.Status.Devfile).Should(Not(Equal("")))

			hasAppLookupKey := types.NamespacedName{Name: applicationName, Namespace: AppNamespace}
			createdHasApp := &appstudiov1alpha1.Application{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasAppLookupKey, createdHasApp)
				return len(createdHasApp.Status.Conditions) > 0 && strings.Contains(createdHasApp.Status.Devfile, ComponentName)
			}, timeout, interval).Should(BeTrue())

			// Make sure the devfile model was properly set in Application
			Expect(createdHasApp.Status.Devfile).Should(Not(Equal("")))

			createdHasComp.Status.GitOps.Branch = "main"
			createdHasComp.Status.GitOps.Context = ""
			Expect(k8sClient.Status().Update(context.Background(), createdHasComp)).Should(Succeed())

			// Get the updated resource
			Eventually(func() bool {
				k8sClient.Get(context.Background(), hasCompLookupKey, createdHasComp)
				// Return true if the most recent condition on the CR is updated
				return createdHasComp.Status.GitOps.Branch == "main" && createdHasComp.Status.GitOps.Context == ""
			}, timeout, interval).Should(BeTrue())

			// Delete the Component resource
			Eventually(func() error {
				return k8sClient.Delete(context.Background(), createdHasComp)
			}, timeout, interval).Should(Succeed())

			// Wait for delete to finish
			Eventually(func() error {
				f := &appstudiov1alpha1.Component{}
				return k8sClient.Get(context.Background(), hasCompLookupKey, f)
			}, timeout, interval).ShouldNot(Succeed())

			// Delete the Application resource
			Eventually(func() error {
				return k8sClient.Delete(context.Background(), createdHasApp)
			}, timeout, interval).Should(Succeed())

			// Wait for delete to finish
			Eventually(func() error {
				f := &appstudiov1alpha1.Application{}
				return k8sClient.Get(context.Background(), hasAppLookupKey, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})
})
