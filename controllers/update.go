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
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
				devfileComponent.Attributes = devfileComponent.Attributes.PutString(routeKey, component.Spec.Route)
				compUpdateRequired = true
			}
		}

		// Update for Replica
		currentReplica := 0
		if len(devfileComponent.Attributes) == 0 {
			devfileComponent.Attributes = attributes.Attributes{}
		} else {
			var err error
			currentReplica = int(devfileComponent.Attributes.GetNumber(replicaKey, &err))
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}
		}
		if currentReplica != component.Spec.Replicas {
			log.Info(fmt.Sprintf("setting devfile component %s attribute component.Spec.Replicas to %v", devfileComponent.Name, component.Spec.Replicas))
			devfileComponent.Attributes = devfileComponent.Attributes.PutInteger(replicaKey, component.Spec.Replicas)
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
				devfileComponent.Attributes = devfileComponent.Attributes.PutString(storageLimitKey, resourceStorageLimit.String())
				compUpdateRequired = true
			}

			// Ephemeral Storage Limit
			resourceEphemeralStorageLimit := limits[corev1.ResourceEphemeralStorage]
			if len(devfileComponent.Attributes) == 0 {
				devfileComponent.Attributes = attributes.Attributes{}
			}
			if resourceEphemeralStorageLimit.String() != "0" {
				log.Info(fmt.Sprintf("updating devfile component %s attribute ephemeral storage limit to %s", devfileComponent.Name, resourceEphemeralStorageLimit.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString(ephemeralStorageLimitKey, resourceEphemeralStorageLimit.String())
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
				devfileComponent.Attributes = devfileComponent.Attributes.PutString(storageRequestKey, resourceStorageRequest.String())
				compUpdateRequired = true
			}

			// Ephemeral Storage Request
			resourceEphemeralStorageRequest := requests[corev1.ResourceEphemeralStorage]
			if len(devfileComponent.Attributes) == 0 {
				devfileComponent.Attributes = attributes.Attributes{}
			}
			if resourceEphemeralStorageRequest.String() != "0" {
				log.Info(fmt.Sprintf("setting devfile component %s attribute ephemeral storage limit to %s", devfileComponent.Name, resourceEphemeralStorageRequest.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString(ephemeralStorageRequestKey, resourceEphemeralStorageRequest.String())
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

	if component.Spec.Source.GitSource != nil {
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
				return fmt.Errorf("application already has a component with name %s", newProject.Name)
			}
		}
		err = hasAppDevfileData.AddProjects([]devfileAPIV1.Project{newProject})
		if err != nil {
			return err
		}
	} else if component.Spec.Source.ImageSource != nil {
		fmt.Println("*****")
		fmt.Println(component.Spec.Source.ImageSource.ContainerImage)

		var err error

		// Initialize the attributes
		devSpec := hasAppDevfileData.GetDevfileWorkspaceSpec()

		// Add the image as a top level attribute
		devfileAttributes := devSpec.Attributes
		if devfileAttributes == nil {
			devfileAttributes = attributes.Attributes{}
			devSpec.Attributes = devfileAttributes
			hasAppDevfileData.SetDevfileWorkspaceSpec(*devSpec)
		}
		imageAttrString := fmt.Sprintf("containerImage/%s", component.Spec.ComponentName)
		componentImage := devfileAttributes.GetString(imageAttrString, &err)
		if err != nil {
			if _, ok := err.(*attributes.KeyNotFoundError); !ok {
				return err
			}
		}
		if componentImage != "" {
			return fmt.Errorf("application already has a component with name %s", component.Name)
		}
		devSpec.Attributes = devfileAttributes.PutString(imageAttrString, component.Spec.Source.ImageSource.ContainerImage)
		hasAppDevfileData.SetDevfileWorkspaceSpec(*devSpec)

	} else {
		return fmt.Errorf("component source is nil")
	}

	return nil
}

func (r *ComponentDetectionQueryReconciler) updateComponentStub(componentDetectionQuery *appstudiov1alpha1.ComponentDetectionQuery, devfilesMap map[string][]byte, devfilesURLMap map[string]string) error {

	if componentDetectionQuery == nil {
		return fmt.Errorf("componentDetectionQuery is nil")
	}

	log := r.Log.WithValues("ComponentDetectionQuery", "updateComponentStub")

	if len(componentDetectionQuery.Status.ComponentDetected) == 0 {
		componentDetectionQuery.Status.ComponentDetected = make(appstudiov1alpha1.ComponentDetectionMap)
	}

	log.Info(fmt.Sprintf("Devfiles detected: %v", len(devfilesMap)))

	for context, devfileBytes := range devfilesMap {
		log.Info(fmt.Sprintf("Currently reading the devfile from context %v", context))

		// Parse the Component Devfile
		compDevfileData, err := devfile.ParseDevfileModel(string(devfileBytes))
		if err != nil {
			return err
		}

		devfileMetadata := compDevfileData.GetMetadata()
		devfileContainerComponents, err := compDevfileData.GetComponents(common.DevfileOptions{
			ComponentOptions: common.ComponentOptions{
				ComponentType: devfileAPIV1.ContainerComponentType,
			},
		})
		if err != nil {
			return err
		}

		componentStub := appstudiov1alpha1.ComponentSpec{
			ComponentName: devfileMetadata.Name,
			Application:   "insert-application-name",
			Context:       context,
			Source: appstudiov1alpha1.ComponentSource{
				ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
					GitSource: &appstudiov1alpha1.GitSource{
						URL:        componentDetectionQuery.Spec.GitSource.URL,
						DevfileURL: devfilesURLMap[context],
					},
				},
			},
		}

		// Since a devfile can have N container components, we only try to populate the stub with the first container component
		if len(devfileContainerComponents) != 0 {
			// Devfile Env
			for _, devfileEnv := range devfileContainerComponents[0].Container.Env {
				componentStub.Env = append(componentStub.Env, corev1.EnvVar{
					Name:  devfileEnv.Name,
					Value: devfileEnv.Value,
				})
			}

			// Devfile Port
			for i, endpoint := range devfileContainerComponents[0].Container.Endpoints {
				if i == 0 {
					componentStub.TargetPort = endpoint.TargetPort
					break
				}
			}

			// Devfile Route
			componentStub.Route = devfileContainerComponents[0].Attributes.GetString(routeKey, &err)
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}

			// Devfile Replica
			componentStub.Replicas = int(devfileContainerComponents[0].Attributes.GetNumber(replicaKey, &err))
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}

			// Devfile Limits
			if len(componentStub.Resources.Limits) == 0 {
				componentStub.Resources.Limits = make(corev1.ResourceList)
			}
			limits := componentStub.Resources.Limits

			// CPU Limit
			if devfileContainerComponents[0].Container.CpuLimit != "" {
				cpuLimit, err := resource.ParseQuantity(devfileContainerComponents[0].Container.CpuLimit)
				if err != nil {
					return err
				}
				limits[corev1.ResourceCPU] = cpuLimit
			}

			// Memory Limit
			if devfileContainerComponents[0].Container.MemoryLimit != "" {
				memoryLimit, err := resource.ParseQuantity(devfileContainerComponents[0].Container.MemoryLimit)
				if err != nil {
					return err
				}
				limits[corev1.ResourceMemory] = memoryLimit
			}

			// Storage Limit
			storageLimitString := devfileContainerComponents[0].Attributes.GetString(storageLimitKey, &err)
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}
			if storageLimitString != "" {
				storageLimit, err := resource.ParseQuantity(storageLimitString)
				if err != nil {
					return err
				}
				limits[corev1.ResourceStorage] = storageLimit
			}

			// Ephemeral Storage Limit
			ephemeralStorageLimitString := devfileContainerComponents[0].Attributes.GetString(ephemeralStorageLimitKey, &err)
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}
			if ephemeralStorageLimitString != "" {
				ephemeralStorageLimit, err := resource.ParseQuantity(ephemeralStorageLimitString)
				if err != nil {
					return err
				}
				limits[corev1.ResourceEphemeralStorage] = ephemeralStorageLimit
			}

			// Devfile Request
			if len(componentStub.Resources.Requests) == 0 {
				componentStub.Resources.Requests = make(corev1.ResourceList)
			}
			requests := componentStub.Resources.Requests

			// CPU Request
			if devfileContainerComponents[0].Container.CpuRequest != "" {
				CpuRequest, err := resource.ParseQuantity(devfileContainerComponents[0].Container.CpuRequest)
				if err != nil {
					return err
				}
				requests[corev1.ResourceCPU] = CpuRequest
			}

			// Memory Request
			if devfileContainerComponents[0].Container.MemoryRequest != "" {
				memoryRequest, err := resource.ParseQuantity(devfileContainerComponents[0].Container.MemoryRequest)
				if err != nil {
					return err
				}
				requests[corev1.ResourceMemory] = memoryRequest
			}

			// Storage Request
			storageRequestString := devfileContainerComponents[0].Attributes.GetString(storageRequestKey, &err)
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}
			if storageRequestString != "" {
				storageRequest, err := resource.ParseQuantity(storageRequestString)
				if err != nil {
					return err
				}
				requests[corev1.ResourceStorage] = storageRequest
			}

			// Ephemeral Storage Request
			ephemeralStorageRequestString := devfileContainerComponents[0].Attributes.GetString(ephemeralStorageRequestKey, &err)
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}
			if ephemeralStorageRequestString != "" {
				ephemeralStorageRequest, err := resource.ParseQuantity(ephemeralStorageRequestString)
				if err != nil {
					return err
				}
				requests[corev1.ResourceEphemeralStorage] = ephemeralStorageRequest
			}
		}

		componentDetectionQuery.Status.ComponentDetected[devfileMetadata.Name] = appstudiov1alpha1.ComponentDetectionDescription{
			DevfileFound:  true,
			Language:      devfileMetadata.Language,
			ProjectType:   devfileMetadata.ProjectType,
			ComponentStub: componentStub,
		}
	}

	return nil
}
