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
			ComponentType: devfileAPIV1.KubernetesComponentType,
		},
	})
	if err != nil {
		return err
	}

	for _, devfileComponent := range devfileComponents {
		compUpdateRequired := false

		// Update for Route
		if component.Spec.Route != "" {
			if len(devfileComponent.Attributes) == 0 {
				devfileComponent.Attributes = attributes.Attributes{}
			}
			log.Info(fmt.Sprintf("setting devfile component %s attribute component.Spec.Route %s", devfileComponent.Name, component.Spec.Route))
			devfileComponent.Attributes = devfileComponent.Attributes.PutString(routeKey, component.Spec.Route)
			compUpdateRequired = true
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
		currentPort := 0
		if len(devfileComponent.Attributes) == 0 {
			devfileComponent.Attributes = attributes.Attributes{}
		} else {
			var err error
			currentPort = int(devfileComponent.Attributes.GetNumber(containerImagePortKey, &err))
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}
		}
		if currentPort != component.Spec.TargetPort {
			log.Info(fmt.Sprintf("setting devfile component %s attribute component.Spec.TargetPort %v", devfileComponent.Name, component.Spec.TargetPort))
			devfileComponent.Attributes = devfileComponent.Attributes.PutInteger(replicaKey, component.Spec.Replicas)
			compUpdateRequired = true
		}

		// Update for Env
		currentENV := []corev1.EnvVar{}
		err := devfileComponent.Attributes.GetInto(containerENVKey, &currentENV)
		if err != nil {
			if _, ok := err.(*attributes.KeyNotFoundError); !ok {
				return err
			}
		}
		for _, env := range component.Spec.Env {
			if env.ValueFrom != nil {
				return fmt.Errorf("env.ValueFrom is not supported at the moment, use env.value")
			}

			name := env.Name
			value := env.Value
			isPresent := false

			for i, devfileEnv := range currentENV {
				if devfileEnv.Name == name {
					isPresent = true
					log.Info(fmt.Sprintf("setting devfileComponent %s env %s value to %v", devfileComponent.Name, devfileEnv.Name, value))
					devfileEnv.Value = value
					currentENV[i] = devfileEnv
				}
			}

			if !isPresent {
				log.Info(fmt.Sprintf("appending to devfile component %s env %s : %v", devfileComponent.Name, name, value))
				currentENV = append(currentENV, env)
			}

			devfileComponent.Attributes = devfileComponent.Attributes.FromMap(map[string]interface{}{containerENVKey: currentENV}, &err)
			if err != nil {
				return err
			}
			compUpdateRequired = true
		}

		// Update for limits
		limits := component.Spec.Resources.Limits
		if len(limits) > 0 {
			// CPU Limit
			resourceCPULimit := limits[corev1.ResourceCPU]
			if len(devfileComponent.Attributes) == 0 {
				devfileComponent.Attributes = attributes.Attributes{}
			}
			if resourceCPULimit.String() != "0" {
				log.Info(fmt.Sprintf("setting devfile component %s attribute cpu limit to %s", devfileComponent.Name, resourceCPULimit.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString(cpuLimitKey, resourceCPULimit.String())
				compUpdateRequired = true
			}

			// Memory Limit
			resourceMemoryLimit := limits[corev1.ResourceMemory]
			if len(devfileComponent.Attributes) == 0 {
				devfileComponent.Attributes = attributes.Attributes{}
			}
			if resourceMemoryLimit.String() != "0" {
				log.Info(fmt.Sprintf("setting devfile component %s attribute memory limit to %s", devfileComponent.Name, resourceMemoryLimit.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString(memoryLimitKey, resourceMemoryLimit.String())
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
		}

		// Update for requests
		requests := component.Spec.Resources.Requests
		if len(requests) > 0 {
			// CPU Request
			resourceCPURequest := requests[corev1.ResourceCPU]
			if len(devfileComponent.Attributes) == 0 {
				devfileComponent.Attributes = attributes.Attributes{}
			}
			if resourceCPURequest.String() != "0" {
				log.Info(fmt.Sprintf("updating devfile component %s attribute cpu request to %s", devfileComponent.Name, resourceCPURequest.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString(cpuRequestKey, resourceCPURequest.String())
				compUpdateRequired = true
			}

			// Memory Request
			resourceMemoryRequest := requests[corev1.ResourceMemory]
			if len(devfileComponent.Attributes) == 0 {
				devfileComponent.Attributes = attributes.Attributes{}
			}
			if resourceMemoryRequest.String() != "0" {
				log.Info(fmt.Sprintf("updating devfile component %s attribute memory request to %s", devfileComponent.Name, resourceMemoryRequest.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString(memoryRequestKey, resourceMemoryRequest.String())
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

func (r *ComponentDetectionQueryReconciler) updateComponentStub(componentDetectionQuery *appstudiov1alpha1.ComponentDetectionQuery, devfilesMap map[string][]byte, devfilesURLMap map[string]string, dockerfileContextMap map[string]string) error {

	if componentDetectionQuery == nil {
		return fmt.Errorf("componentDetectionQuery is nil")
	}

	log := r.Log.WithValues("ComponentDetectionQuery", "updateComponentStub")

	if len(componentDetectionQuery.Status.ComponentDetected) == 0 {
		componentDetectionQuery.Status.ComponentDetected = make(appstudiov1alpha1.ComponentDetectionMap)
	}

	log.Info(fmt.Sprintf("Devfiles detected: %v", len(devfilesMap)))

	counter := 0
	componentName := "component"

	for context, devfileBytes := range devfilesMap {
		log.Info(fmt.Sprintf("Currently reading the devfile for context %v", context))

		// Parse the Component Devfile
		compDevfileData, err := devfile.ParseDevfileModel(string(devfileBytes))
		if err != nil {
			return err
		}

		devfileMetadata := compDevfileData.GetMetadata()
		devfileContainerComponents, err := compDevfileData.GetComponents(common.DevfileOptions{
			ComponentOptions: common.ComponentOptions{
				ComponentType: devfileAPIV1.KubernetesComponentType,
			},
		})
		if err != nil {
			return err
		}

		if len(devfileMetadata.Name) > 0 {
			componentName = devfileMetadata.Name
		}
		counter++
		componentName = fmt.Sprintf("%d", counter) + "-" + componentName

		componentStub := appstudiov1alpha1.ComponentSpec{
			ComponentName: componentName,
			Application:   "insert-application-name",
			Context:       context,
			Source: appstudiov1alpha1.ComponentSource{
				ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
					GitSource: &appstudiov1alpha1.GitSource{
						URL:           componentDetectionQuery.Spec.GitSource.URL,
						DevfileURL:    devfilesURLMap[context],
						DockerfileURL: dockerfileContextMap[context],
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
		}

		componentDetectionQuery.Status.ComponentDetected[componentName] = appstudiov1alpha1.ComponentDetectionDescription{
			DevfileFound:  len(devfilesURLMap[context]) == 0, // if we did not find a devfile URL map for the given context, it means a devfile was found in the context
			Language:      devfileMetadata.Language,
			ProjectType:   devfileMetadata.ProjectType,
			ComponentStub: componentStub,
		}

		// Once the dockerfile has been processed, remove it
		delete(dockerfileContextMap, context)
	}

	log.Info(fmt.Sprintf("Dockerfiles detected: %v", len(dockerfileContextMap)))

	// process the dockefileMap that does not have an associated devfile with it
	for context, link := range dockerfileContextMap {
		log.Info(fmt.Sprintf("Currently reading the Dockerfile for context %v", context))

		counter++
		componentName = fmt.Sprintf("%d", counter) + "-dockerfile"

		componentDetectionQuery.Status.ComponentDetected[componentName] = appstudiov1alpha1.ComponentDetectionDescription{
			DevfileFound: false, // always false since there is only a dockerfile present for these contexts
			Language:     "Dockerfile",
			ProjectType:  "Dockerfile",
			ComponentStub: appstudiov1alpha1.ComponentSpec{
				ComponentName: componentName,
				Application:   "insert-application-name",
				Context:       context,
				Source: appstudiov1alpha1.ComponentSource{
					ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
						GitSource: &appstudiov1alpha1.GitSource{
							URL:           componentDetectionQuery.Spec.GitSource.URL,
							DockerfileURL: link,
						},
					},
				},
			},
		}
	}

	return nil
}
