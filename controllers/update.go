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
	"regexp"
	"strconv"
	"strings"

	"github.com/brianvoe/gofakeit/v6"
	devfileAPIV1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	data "github.com/devfile/library/pkg/devfile/parser/data"
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	"github.com/redhat-appstudio/application-service/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *ComponentReconciler) updateComponentDevfileModel(req ctrl.Request, hasCompDevfileData data.DevfileData, component appstudiov1alpha1.Component) error {

	log := r.Log.WithValues("Component", req.NamespacedName).WithValues("clusterName", req.ClusterName)

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
		var err error
		currentPort := int(devfileComponent.Attributes.GetNumber(containerImagePortKey, &err))
		if err != nil {
			if _, ok := err.(*attributes.KeyNotFoundError); !ok {
				return err
			}
		}
		if currentPort != component.Spec.TargetPort {
			log.Info(fmt.Sprintf("setting devfile component %s attribute component.Spec.TargetPort %v", devfileComponent.Name, component.Spec.TargetPort))
			devfileComponent.Attributes = devfileComponent.Attributes.PutInteger(containerImagePortKey, component.Spec.TargetPort)
			compUpdateRequired = true
		}

		// Update for Route
		if component.Spec.Route != "" {
			log.Info(fmt.Sprintf("setting devfile component %s attribute component.Spec.Route %s", devfileComponent.Name, component.Spec.Route))
			devfileComponent.Attributes = devfileComponent.Attributes.PutString(routeKey, component.Spec.Route)
			compUpdateRequired = true
		}

		// Update for Env
		currentENV := []corev1.EnvVar{}
		err = devfileComponent.Attributes.GetInto(containerENVKey, &currentENV)
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
			var err error
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
			if resourceCPULimit.String() != "" {
				log.Info(fmt.Sprintf("setting devfile component %s attribute cpu limit to %s", devfileComponent.Name, resourceCPULimit.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString(cpuLimitKey, resourceCPULimit.String())
				compUpdateRequired = true
			}

			// Memory Limit
			resourceMemoryLimit := limits[corev1.ResourceMemory]
			if resourceMemoryLimit.String() != "" {
				log.Info(fmt.Sprintf("setting devfile component %s attribute memory limit to %s", devfileComponent.Name, resourceMemoryLimit.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString(memoryLimitKey, resourceMemoryLimit.String())
				compUpdateRequired = true
			}

			// Storage Limit
			resourceStorageLimit := limits[corev1.ResourceStorage]
			if resourceStorageLimit.String() != "" {
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
			if resourceCPURequest.String() != "" {
				log.Info(fmt.Sprintf("updating devfile component %s attribute cpu request to %s", devfileComponent.Name, resourceCPURequest.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString(cpuRequestKey, resourceCPURequest.String())
				compUpdateRequired = true
			}

			// Memory Request
			resourceMemoryRequest := requests[corev1.ResourceMemory]
			if resourceMemoryRequest.String() != "" {
				log.Info(fmt.Sprintf("updating devfile component %s attribute memory request to %s", devfileComponent.Name, resourceMemoryRequest.String()))
				devfileComponent.Attributes = devfileComponent.Attributes.PutString(memoryRequestKey, resourceMemoryRequest.String())
				compUpdateRequired = true
			}

			// Storage Request
			resourceStorageRequest := requests[corev1.ResourceStorage]
			if resourceStorageRequest.String() != "" {
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
	} else if component.Spec.ContainerImage != "" {
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
		devSpec.Attributes = devfileAttributes.PutString(imageAttrString, component.Spec.ContainerImage)
		hasAppDevfileData.SetDevfileWorkspaceSpec(*devSpec)

	} else {
		return fmt.Errorf("component source is nil")
	}

	return nil
}

func (r *ComponentDetectionQueryReconciler) updateComponentStub(req ctrl.Request, componentDetectionQuery *appstudiov1alpha1.ComponentDetectionQuery, devfilesMap map[string][]byte, devfilesURLMap map[string]string, dockerfileContextMap map[string]string) error {

	if componentDetectionQuery == nil {
		return fmt.Errorf("componentDetectionQuery is nil")
	}

	log := r.Log.WithValues("ComponentDetectionQuery", req.NamespacedName).WithValues("clusterName", req.ClusterName)

	if len(componentDetectionQuery.Status.ComponentDetected) == 0 {
		componentDetectionQuery.Status.ComponentDetected = make(appstudiov1alpha1.ComponentDetectionMap)
	}

	log.Info(fmt.Sprintf("Devfiles detected: %v", len(devfilesMap)))

	for context, devfileBytes := range devfilesMap {
		log.Info(fmt.Sprintf("Currently reading the devfile for context %v", context))

		// Parse the Component Devfile
		compDevfileData, err := devfile.ParseDevfileModel(string(devfileBytes))
		if err != nil {
			return err
		}

		devfileMetadata := compDevfileData.GetMetadata()
		devfileKubernetesComponents, err := compDevfileData.GetComponents(common.DevfileOptions{
			ComponentOptions: common.ComponentOptions{
				ComponentType: devfileAPIV1.KubernetesComponentType,
			},
		})
		if err != nil {
			return err
		}

		// componentName := "component"
		gitSource := &appstudiov1alpha1.GitSource{
			Context:       context,
			URL:           componentDetectionQuery.Spec.GitSource.URL,
			DevfileURL:    devfilesURLMap[context],
			DockerfileURL: dockerfileContextMap[context],
		}
		componentName := getComponentName(gitSource)

		componentStub := appstudiov1alpha1.ComponentSpec{
			ComponentName: componentName,
			Application:   "insert-application-name",
			Source: appstudiov1alpha1.ComponentSource{
				ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
					GitSource: gitSource,
				},
			},
		}

		// Since a devfile can have N container components, we only try to populate the stub with the first Kubernetes component
		if len(devfileKubernetesComponents) != 0 {
			kubernetesComponentAttribute := devfileKubernetesComponents[0].Attributes

			// Devfile Env
			err := kubernetesComponentAttribute.GetInto(containerENVKey, &componentStub.Env)
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}

			// Devfile Port
			componentStub.TargetPort = int(kubernetesComponentAttribute.GetNumber(containerImagePortKey, &err))
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}

			// Devfile Route
			componentStub.Route = kubernetesComponentAttribute.GetString(routeKey, &err)
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}

			// Devfile Replica
			componentStub.Replicas = int(kubernetesComponentAttribute.GetNumber(replicaKey, &err))
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
			cpuLimitString := kubernetesComponentAttribute.GetString(cpuLimitKey, &err)
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}
			if cpuLimitString != "" {
				cpuLimit, err := resource.ParseQuantity(cpuLimitString)
				if err != nil {
					return err
				}
				limits[corev1.ResourceCPU] = cpuLimit
			}

			// Memory Limit
			memoryLimitString := kubernetesComponentAttribute.GetString(memoryLimitKey, &err)
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}
			if memoryLimitString != "" {
				memoryLimit, err := resource.ParseQuantity(memoryLimitString)
				if err != nil {
					return err
				}
				limits[corev1.ResourceMemory] = memoryLimit
			}

			// Storage Limit
			storageLimitString := kubernetesComponentAttribute.GetString(storageLimitKey, &err)
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
			cpuRequestString := kubernetesComponentAttribute.GetString(cpuRequestKey, &err)
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}
			if cpuRequestString != "" {
				cpuRequest, err := resource.ParseQuantity(cpuRequestString)
				if err != nil {
					return err
				}
				requests[corev1.ResourceCPU] = cpuRequest
			}

			// Memory Request
			memoryRequestString := kubernetesComponentAttribute.GetString(memoryRequestKey, &err)
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}
			if memoryRequestString != "" {
				memoryRequest, err := resource.ParseQuantity(memoryRequestString)
				if err != nil {
					return err
				}
				requests[corev1.ResourceMemory] = memoryRequest
			}

			// Storage Request
			storageRequestString := kubernetesComponentAttribute.GetString(storageRequestKey, &err)
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
			DevfileFound:  len(devfilesURLMap[context]) != 0, // if we did not find a devfile URL map for the given context, it means a devfile was not found in the context
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

		gitSource := &appstudiov1alpha1.GitSource{
			Context:       context,
			URL:           componentDetectionQuery.Spec.GitSource.URL,
			DockerfileURL: link,
		}
		componentName := getComponentName(gitSource)

		componentDetectionQuery.Status.ComponentDetected[componentName] = appstudiov1alpha1.ComponentDetectionDescription{
			DevfileFound: false, // always false since there is only a dockerfile present for these contexts
			Language:     "Dockerfile",
			ProjectType:  "Dockerfile",
			ComponentStub: appstudiov1alpha1.ComponentSpec{
				ComponentName: componentName,
				Application:   "insert-application-name",
				Source: appstudiov1alpha1.ComponentSource{
					ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
						GitSource: gitSource,
					},
				},
			},
		}
	}

	return nil
}

func getComponentName(gitSource *appstudiov1alpha1.GitSource) string {
	var componentName string
	repoUrl := gitSource.URL

	if len(repoUrl) != 0 {
		// If the repository URL ends in a forward slash, remove it to avoid issues with parsing the repository name
		if string(repoUrl[len(repoUrl)-1]) == "/" {
			repoUrl = repoUrl[0 : len(repoUrl)-1]
		}
		lastElement := repoUrl[strings.LastIndex(repoUrl, "/")+1:]
		repoName := strings.Split(lastElement, ".git")[0]
		componentName = repoName
		context := gitSource.Context
		if context != "" && context != "./" && context != "." {
			componentName = fmt.Sprintf("%s-%s", context, repoName)
		}
	}

	// Return a sanitized version of the component name
	// If len(componentName) is 0, then it will also handle generating a random name for it.
	return sanitizeComponentName(componentName)
}

// sanitizeComponentName sanitizes component name with the following requirements:
// - Contain at most 63 characters
// - Contain only lowercase alphanumeric characters or ‘-’
// - Start with an alphanumeric character
// - End with an alphanumeric character
// - Must not contain all numeric values
func sanitizeComponentName(name string) string {
	exclusive := regexp.MustCompile(`[^a-zA-Z0-9-]`)
	// filter out invalid characters
	name = exclusive.ReplaceAllString(name, "")
	// Fallback: A proper Component name should never be an empty string, but in case it is, generate a random name for it.
	if name == "" {
		name = gofakeit.Noun()
	}
	_, err := strconv.ParseFloat(name, 64)
	if err != nil {
		// convert all Uppercase chars to lowercase
		name = strings.ToLower(name)
	} else {
		// contains only numeric values, prefix a character
		name = fmt.Sprintf("comp-%s", name)
	}
	if len(name) > 58 {
		name = name[0:58]
	}

	// to avoid name conflict with existing component, append random 4 chars at end of the name
	name = fmt.Sprintf("%s-%s", name, util.GetRandomString(4, true))

	return name
}
