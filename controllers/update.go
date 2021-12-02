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
	"fmt"

	devfileAPIV1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	data "github.com/devfile/library/pkg/devfile/parser/data"
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func (r *ComponentReconciler) updateComponentDevfileModel(hasCompDevfileData data.DevfileData, component appstudiov1alpha1.Component) error {

	log := r.Log.WithValues("Component", "updateComponentDevfileModel")

	devfileComponents, err := hasCompDevfileData.GetComponents(common.DevfileOptions{
		ComponentOptions: common.ComponentOptions{
			ComponentType: devfileAPIV1.ContainerComponentType,
		},
	})
	if err != nil {
		return err
	}

	for i, devfileComponent := range devfileComponents {
		compUpdateRequired := false

		// Update for Route
		if component.Spec.Route != "" {
			if len(devfileComponent.Attributes) == 0 {
				devfileComponent.Attributes = attributes.Attributes{}
			}
			if component.Spec.Route != "" {
				log.Info(fmt.Sprintf("setting devfile component %s attribute component.Spec.Route %s", devfileComponent.Name, component.Spec.Route))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString("appstudio.has/route", component.Spec.Route)
				compUpdateRequired = true
			}
		}

		// Update for Replica
		currentReplica := 0
		if len(devfileComponent.Attributes) == 0 {
			devfileComponent.Attributes = attributes.Attributes{}
		} else {
			var err error
			currentReplica = int(devfileComponent.Attributes.GetNumber("appstudio.has/replicas", &err))
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}
		}
		if currentReplica != component.Spec.Replicas {
			log.Info(fmt.Sprintf("setting devfile component %s attribute component.Spec.Replicas to %v", devfileComponent.Name, component.Spec.Replicas))
			devfileComponent.Attributes = devfileComponent.Attributes.PutInteger("appstudio.has/replicas", component.Spec.Replicas)
			compUpdateRequired = true
		}

		// Update for Port
		if i == 0 && component.Spec.TargetPort > 0 {
			for i, endpoint := range devfileComponent.Container.Endpoints {
				log.Info(fmt.Sprintf("setting devfile component %s endpoint %s port to %v", devfileComponent.Name, endpoint.Name, component.Spec.TargetPort))
				endpoint.TargetPort = component.Spec.TargetPort
				compUpdateRequired = true
				devfileComponent.Container.Endpoints[i] = endpoint
			}
		}

		// Update for Env
		for _, env := range component.Spec.Env {
			if env.ValueFrom != nil {
				return fmt.Errorf("env.ValueFrom is not supported at the moment, use env.value")
			}

			name := env.Name
			value := env.Value
			isPresent := false

			for i, devfileEnv := range devfileComponent.Container.Env {
				if devfileEnv.Name == name {
					isPresent = true
					log.Info(fmt.Sprintf("setting devfileComponent %s env %s value to %v", devfileComponent.Name, devfileEnv.Name, value))
					devfileEnv.Value = value
					devfileComponent.Container.Env[i] = devfileEnv
					compUpdateRequired = true
				}
			}

			if !isPresent {
				log.Info(fmt.Sprintf("appending to devfile component %s env %s : %v", devfileComponent.Name, name, value))
				devfileComponent.Container.Env = append(devfileComponent.Container.Env, devfileAPIV1.EnvVar{Name: name, Value: value})
				compUpdateRequired = true
			}

		}

		// Update for limits
		limits := component.Spec.Resources.Limits
		if len(limits) > 0 {
			// CPU Limit
			resourceCPULimit := limits[corev1.ResourceCPU]
			if resourceCPULimit.String() != "" {
				log.Info(fmt.Sprintf("setting devfile component %s cpu limit to %s", devfileComponent.Name, resourceCPULimit.String()))
				devfileComponent.Container.CpuLimit = resourceCPULimit.String()
				compUpdateRequired = true
			}

			// Memory Limit
			resourceMemoryLimit := limits[corev1.ResourceMemory]
			if resourceMemoryLimit.String() != "" {
				log.Info(fmt.Sprintf("setting devfile component %s memory limit to %s", devfileComponent.Name, resourceMemoryLimit.String()))
				devfileComponent.Container.MemoryLimit = resourceMemoryLimit.String()
				compUpdateRequired = true
			}

			// Storage Limit
			resourceStorageLimit := limits[corev1.ResourceStorage]
			if len(devfileComponent.Attributes) == 0 {
				devfileComponent.Attributes = attributes.Attributes{}
			}
			if resourceStorageLimit.String() != "0" {
				log.Info(fmt.Sprintf("setting devfile component %s attribute storage limit to %s", devfileComponent.Name, resourceStorageLimit.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString("appstudio.has/storageLimit", resourceStorageLimit.String())
				compUpdateRequired = true
			}

			// Ephermetal Storage Limit
			resourceEphermeralStorageLimit := limits[corev1.ResourceEphemeralStorage]
			if len(devfileComponent.Attributes) == 0 {
				devfileComponent.Attributes = attributes.Attributes{}
			}
			if resourceEphermeralStorageLimit.String() != "0" {
				log.Info(fmt.Sprintf("updating devfile component %s attribute ephermeal storage limit to %s", devfileComponent.Name, resourceEphermeralStorageLimit.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString("appstudio.has/ephermealStorageLimit", resourceEphermeralStorageLimit.String())
				compUpdateRequired = true
			}
		}

		// Update for requests
		requests := component.Spec.Resources.Requests
		if len(requests) > 0 {
			// CPU Request
			resourceCPURequest := requests[corev1.ResourceCPU]
			if resourceCPURequest.String() != "" {
				log.Info(fmt.Sprintf("setting devfile component %s cpu request  to %s", devfileComponent.Name, resourceCPURequest.String()))
				devfileComponent.Container.CpuRequest = resourceCPURequest.String()
				compUpdateRequired = true
			}

			// Memory Request
			resourceMemoryRequest := requests[corev1.ResourceMemory]
			if resourceMemoryRequest.String() != "" {
				log.Info(fmt.Sprintf("setting devfile component %s memory request to %s", devfileComponent.Name, resourceMemoryRequest.String()))
				devfileComponent.Container.MemoryRequest = resourceMemoryRequest.String()
				compUpdateRequired = true
			}

			// Storage Request
			resourceStorageRequest := requests[corev1.ResourceStorage]
			if len(devfileComponent.Attributes) == 0 {
				devfileComponent.Attributes = attributes.Attributes{}
			}
			if resourceStorageRequest.String() != "0" {
				log.Info(fmt.Sprintf("updating devfile component %s attribute storage request to %s", devfileComponent.Name, resourceStorageRequest.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString("appstudio.has/storageRequest", resourceStorageRequest.String())
				compUpdateRequired = true
			}

			// Ephermetal Storage Request
			resourceEphermeralStorageRequest := requests[corev1.ResourceEphemeralStorage]
			if len(devfileComponent.Attributes) == 0 {
				devfileComponent.Attributes = attributes.Attributes{}
			}
			if resourceEphermeralStorageRequest.String() != "0" {
				log.Info(fmt.Sprintf("setting devfile component %s attribute ephermeal storage limit to %s", devfileComponent.Name, resourceEphermeralStorageRequest.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString("appstudio.has/ephermealStorageRequest", resourceEphermeralStorageRequest.String())
				compUpdateRequired = true
			}
		}

		if compUpdateRequired {
			// Update the devfileComponent once it has been updated with the Component data
			log.Info(fmt.Sprintf("updating devfile component name %s ...", devfileComponent.Name))
			err := hasCompDevfileData.UpdateComponent(devfileComponent)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ComponentReconciler) updateApplicationDevfileModel(hasAppDevfileData data.DevfileData, component appstudiov1alpha1.Component) error {

	if component.Spec.Source.GitSource == nil {
		return fmt.Errorf("component git source is nil")
	}

	newProject := devfileAPIV1.Project{
		Name: component.Spec.ComponentName,
		ProjectSource: devfileAPIV1.ProjectSource{
			Git: &devfileAPIV1.GitProjectSource{
				GitLikeProjectSource: devfileAPIV1.GitLikeProjectSource{
					Remotes: map[string]string{
						"origin": component.Spec.Source.GitSource.URL,
					},
				},
			},
		},
	}
	projects, err := hasAppDevfileData.GetProjects(common.DevfileOptions{})
	if err != nil {
		return err
	}
	for _, project := range projects {
		if project.Name == newProject.Name {
			return fmt.Errorf("application already has a project with name %s", newProject.Name)
		}
	}
	err = hasAppDevfileData.AddProjects([]devfileAPIV1.Project{newProject})
	if err != nil {
		return err
	}

	return nil
}
