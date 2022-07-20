//
// Copyright 2021-2022 Red Hat, Inc.
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

package gitops

import (
	"fmt"
	"path/filepath"

	routev1 "github.com/openshift/api/route/v1"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops/prepare"
	"github.com/redhat-appstudio/application-service/gitops/resources"
	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	"github.com/spf13/afero"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	yaml "github.com/redhat-appstudio/application-service/gitops/yaml"
)

const (
	kustomizeFileName       = "kustomization.yaml"
	deploymentFileName      = "deployment.yaml"
	deploymentPatchFileName = "deployment-patch.yaml"
	serviceFileName         = "service.yaml"
	routeFileName           = "route.yaml"
	repositoryFileName      = "repository.yaml"
)

// Generate takes in a given Component CR and
// spits out a deployment, service, and route file to disk
func Generate(fs afero.Afero, gitOpsFolder string, outputFolder string, component appstudiov1alpha1.Component, gitopsConfig prepare.GitopsConfig) error {
	deployment := generateDeployment(component)

	k := resources.Kustomization{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
	}
	k.AddResources(deploymentFileName)
	resources := map[string]interface{}{
		deploymentFileName: deployment,
	}

	// If a targetPort was specified, also generate a service and route
	if component.Spec.TargetPort != 0 {
		service := generateService(component)
		route := generateRoute(component)
		k.AddResources(deploymentFileName, serviceFileName, routeFileName)
		resources[serviceFileName] = service
		resources[routeFileName] = route
	}

	if component.Spec.Source.GitSource != nil && component.Spec.Source.GitSource.URL != "" {
		tektonResourcesDirName := ".tekton"
		k.AddResources(tektonResourcesDirName + "/")

		if err := GenerateBuild(fs, filepath.Join(outputFolder, tektonResourcesDirName), component, gitopsConfig); err != nil {
			return err
		}
	}

	resources[kustomizeFileName] = k

	_, err := yaml.WriteResources(fs, outputFolder, resources)
	if err != nil {
		return err
	}

	// Re-generate the parent kustomize file and return
	return GenerateParentKustomize(fs, gitOpsFolder)
}

// GenerateOverlays generates the overlays dir from a given BindingComponent
func GenerateOverlays(fs afero.Afero, gitOpsFolder string, outputFolder string, component appstudioshared.BindingComponent, environment appstudioshared.Environment, imageName, namespace string, componentGeneratedResources map[string][]string) error {
	kustomizeFileExist, err := fs.Exists(filepath.Join(outputFolder, kustomizeFileName))
	if err != nil {
		return err
	}
	// if kustomizeFile already exist, read in the content
	var originalKustomizeFileContent resources.Kustomization
	if kustomizeFileExist {
		err = yaml.UnMarshalItemFromFile(fs, filepath.Join(outputFolder, kustomizeFileName), &originalKustomizeFileContent)
		if err != nil {
			return fmt.Errorf("failed to unmarshal items from %q: %v", filepath.Join(outputFolder, kustomizeFileName), err)
		}
		err = fs.Remove(filepath.Join(outputFolder, kustomizeFileName))
		if err != nil {
			return fmt.Errorf("failed to delete %s file in folder %q: %s", kustomizeFileName, outputFolder, err)
		}
	}

	k := resources.Kustomization{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
	}

	deploymentPatch := generateDeploymentPatch(component, environment, imageName, namespace)

	k.AddResources("../../base")
	k.AddPatches(deploymentPatchFileName)
	if componentGeneratedResources == nil {
		componentGeneratedResources = make(map[string][]string)
	}
	componentGeneratedResources[component.Name] = append(componentGeneratedResources[component.Name], deploymentPatchFileName)

	// add back custom kustomization patches
	k.CompareDifferenceAndAddCustomPatches(originalKustomizeFileContent.Patches, componentGeneratedResources[component.Name])

	resources := map[string]interface{}{
		deploymentPatchFileName: deploymentPatch,
		kustomizeFileName:       k,
	}

	_, err = yaml.WriteResources(fs, outputFolder, resources)
	return err
}

// GenerateParentKustomize takes in a folder of components, and outputs a kustomize file to the outputFolder dir
// containing entries for each Component.
func GenerateParentKustomize(fs afero.Afero, gitOpsFolder string) error {
	componentsFolder := filepath.Join(gitOpsFolder, "components")
	k := resources.Kustomization{
		Kind:       "Kustomization",
		APIVersion: "kustomize.config.k8s.io/v1beta1",
	}

	resources := map[string]interface{}{}

	fInfo, err := fs.ReadDir(componentsFolder)
	if err != nil {
		return err
	}
	for _, file := range fInfo {
		if file.IsDir() {
			k.AddBases(filepath.Join("components", file.Name(), "base"))
		}
	}

	resources[kustomizeFileName] = k

	_, err = yaml.WriteResources(fs, gitOpsFolder, resources)
	return err
}

func generateDeployment(component appstudiov1alpha1.Component) *appsv1.Deployment {
	var containerImage string
	if component.Spec.ContainerImage != "" {
		containerImage = component.Spec.ContainerImage
	}
	replicas := getReplicas(component)
	k8sLabels := generateK8sLabels(component)
	matchLabels := getMatchLabel(component)
	deployment := appsv1.Deployment{
		TypeMeta: v1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      component.Name,
			Namespace: component.Namespace,
			Labels:    k8sLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &v1.LabelSelector{
				MatchLabels: matchLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: matchLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "container-image",
							Image:           containerImage,
							ImagePullPolicy: corev1.PullAlways,
							Env:             component.Spec.Env,
							Resources:       component.Spec.Resources,
						},
					},
				},
			},
		},
	}

	// If a container image source was set in the component *and* a given secret was set for it,
	// Set the secret as an image pull secret, in case the component references a private image component
	if component.Spec.ContainerImage != "" && component.Spec.Secret != "" {
		deployment.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{
				Name: component.Spec.Secret,
			},
		}
	}

	// Set fields that may have been optionally configured by the component CR
	if component.Spec.TargetPort != 0 {
		deployment.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{
				ContainerPort: int32(component.Spec.TargetPort),
			},
		}
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(component.Spec.TargetPort),
				},
			},
		}
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Port: intstr.FromInt(component.Spec.TargetPort),
					Path: "/",
				},
			},
		}
	}

	return &deployment
}

func generateDeploymentPatch(component appstudioshared.BindingComponent, environment appstudioshared.Environment, imageName, namespace string) *appsv1.Deployment {

	deployment := appsv1.Deployment{
		TypeMeta: v1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      component.Name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container-image",
							Image: imageName,
						},
					},
				},
			},
		},
	}

	for _, env := range component.Configuration.Env {
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  env.Name,
			Value: env.Value,
		})
	}

	// only add the environment env configurations, if a deployment/binding env is not present with the same env name
	for _, env := range environment.Spec.Configuration.Env {
		isPresent := false
		for _, deploymentEnv := range deployment.Spec.Template.Spec.Containers[0].Env {
			if deploymentEnv.Name == env.Name {
				isPresent = true
				break
			}
		}

		if !isPresent {
			deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
				Name:  env.Name,
				Value: env.Value,
			})
		}
	}

	if component.Configuration.Replicas > 0 {
		replica := int32(component.Configuration.Replicas)
		deployment.Spec.Replicas = &replica
	}

	if component.Configuration.Resources != nil {
		deployment.Spec.Template.Spec.Containers[0].Resources = *component.Configuration.Resources
	}

	return &deployment
}

func generateService(component appstudiov1alpha1.Component) *corev1.Service {
	k8sLabels := generateK8sLabels(component)
	matchLabels := getMatchLabel(component)
	service := corev1.Service{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      component.Name,
			Namespace: component.Namespace,
			Labels:    k8sLabels,
		},
		Spec: corev1.ServiceSpec{
			Selector: matchLabels,
			Ports: []corev1.ServicePort{
				{
					Port:       int32(component.Spec.TargetPort),
					TargetPort: intstr.FromInt(component.Spec.TargetPort),
				},
			},
		},
	}

	return &service
}

func generateRoute(component appstudiov1alpha1.Component) *routev1.Route {
	k8sLabels := generateK8sLabels(component)
	weight := int32(100)
	route := routev1.Route{
		TypeMeta: v1.TypeMeta{
			Kind:       "Route",
			APIVersion: "route.openshift.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      component.Name,
			Namespace: component.Namespace,
			Labels:    k8sLabels,
		},
		Spec: routev1.RouteSpec{
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromInt(component.Spec.TargetPort),
			},
			TLS: &routev1.TLSConfig{
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				Termination:                   routev1.TLSTerminationEdge,
			},
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   component.Name,
				Weight: &weight,
			},
		},
	}

	// If the route field is set in the spec, set it to be the host for the route
	if component.Spec.Route != "" {
		route.Spec.Host = component.Spec.Route
	}

	return &route
}

// getReplicas returns the number of replicas to be created for the component
// If the field is not set, it returns a default value of 1
// ToDo: Handle as part of a defaulting webhook
func getReplicas(component appstudiov1alpha1.Component) int32 {
	if component.Spec.Replicas > 0 {
		return int32(component.Spec.Replicas)
	}
	return 1
}

// generateLabels returns a map containing the following common Kubernetes labels:
// app.kubernetes.io/name: "<component-name>"
// app.kubernetes.io/instance: "<component-cr-name>"
// app.kubernetes.io/part-of: "<application-name>"
// app.kubernetes.io/managed-by: "kustomize"
// app.kubernetes.io/created-by: "application-service"
func generateK8sLabels(component appstudiov1alpha1.Component) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       component.Spec.ComponentName,
		"app.kubernetes.io/instance":   component.Name,
		"app.kubernetes.io/part-of":    component.Spec.Application,
		"app.kubernetes.io/managed-by": "kustomize",
		"app.kubernetes.io/created-by": "application-service",
	}
}

// GetMatchLabel returns the label selector that will be used to tie deployments, services, and pods together
// For cleanliness, using just one unique label from the generateK8sLabels function
func getMatchLabel(component appstudiov1alpha1.Component) map[string]string {
	return map[string]string{
		"app.kubernetes.io/instance": component.Name,
	}
}
