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
	"regexp"
	"strings"
	"unicode"

	"github.com/devfile/library/v2/pkg/devfile/parser"

	"github.com/brianvoe/gofakeit/v6"
	devfileAPIV1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	"github.com/go-logr/logr"
	appstudiov1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"
	cdqanalysis "github.com/konflux-ci/application-service/cdq-analysis/pkg"
	devfile "github.com/konflux-ci/application-service/pkg/devfile"
	"github.com/konflux-ci/application-service/pkg/metrics"
	"github.com/konflux-ci/application-service/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *ComponentReconciler) updateComponentDevfileModel(req ctrl.Request, hasCompDevfileData data.DevfileData, component appstudiov1alpha1.Component) error {
	log := r.Log.WithValues("controllerKind", "Component").WithValues("name", req.NamespacedName.Name).WithValues("namespace", req.NamespacedName.Namespace)

	// If DockerfileURL is set and the devfile contains references to a Dockerfile then update the devfile
	source := component.Spec.Source
	var err error
	if source.GitSource != nil && source.GitSource.DockerfileURL != "" {
		hasCompDevfileData, err = devfile.UpdateLocalDockerfileURItoAbsolute(hasCompDevfileData, source.GitSource.DockerfileURL)
		if err != nil {
			return fmt.Errorf("unable to convert local Dockerfile URIs to absolute in Component devfile %v", req.NamespacedName)
		}
	}

	kubernetesComponents, err := hasCompDevfileData.GetComponents(common.DevfileOptions{
		ComponentOptions: common.ComponentOptions{
			ComponentType: devfileAPIV1.KubernetesComponentType,
		},
	})
	if err != nil {
		return err
	}

	for _, kubernetesComponent := range kubernetesComponents {
		compUpdateRequired := false
		// Update for Replica
		currentReplica := 1 // default value is 1
		keyFound := true

		if len(kubernetesComponent.Attributes) == 0 {
			kubernetesComponent.Attributes = attributes.Attributes{}
			keyFound = false
		} else {
			var err error
			currentReplica = int(kubernetesComponent.Attributes.GetNumber(devfile.ReplicaKey, &err))
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return &devfile.DevfileAttributeParse{Key: devfile.ReplicaKey, Err: err}
				} else {
					keyFound = false
					currentReplica = 1 //if an error is raised, it'll set currentReplica to 0 so we need to reset back to the default
				}
			}
		}

		numReplicas := 1 //default value
		if component.Spec.Replicas != nil {
			numReplicas = util.GetIntValue(component.Spec.Replicas)
			// Component.Spec.Replicas will override any other settings.
			// We will write the attribute if it doesn't exist for the initial creation case when comp.spec.replica is 1
			if currentReplica != numReplicas || !keyFound {
				log.Info(fmt.Sprintf("setting devfile component %s attribute %s to %v", kubernetesComponent.Name, devfile.ReplicaKey, numReplicas))
				kubernetesComponent.Attributes = kubernetesComponent.Attributes.PutInteger(devfile.ReplicaKey, numReplicas)
				compUpdateRequired = true
			}
		} else {
			//check to see if we have an inlined deployment
			isDeployReplicaSet := false
			inlined := kubernetesComponent.Kubernetes.Inlined
			if inlined != "" {
				log.Info(fmt.Sprintf("reading the kubernetes inline from component %s", component.Name))
				src := parser.YamlSrc{
					Data: []byte(inlined),
				}

				values, err := parser.ReadKubernetesYaml(src, nil, nil)
				if err != nil {
					return err
				}

				resources, err := parser.ParseKubernetesYaml(values)
				if err != nil {
					return err
				}

				if len(resources.Deployments) > 0 {
					replica := resources.Deployments[0].Spec.Replicas
					if replica != nil {
						isDeployReplicaSet = true
						//remove the deployment/replicas attribute which can be left behind if we go from a set value to an unset value
						if kubernetesComponent.Attributes.Exists(devfile.ReplicaKey) {
							var err error
							num := int(kubernetesComponent.Attributes.GetNumber(devfile.ReplicaKey, &err))

							if err != nil {
								if _, ok := err.(*attributes.KeyNotFoundError); !ok {
									return &devfile.DevfileAttributeParse{Key: devfile.ReplicaKey, Err: err}
								} else {
									log.Info(fmt.Sprintf("deleting %s attribute with value %v", devfile.ReplicaKey, num))
									delete(kubernetesComponent.Attributes, devfile.ReplicaKey)
								}
							}

						}
					}
				}
			}

			//set the default if replicas is unset in the component and deployment spec.
			if !isDeployReplicaSet {
				log.Info(fmt.Sprintf("setting devfile component %s attribute component.Spec.Replicas to %v", kubernetesComponent.Name, 1))
				kubernetesComponent.Attributes = kubernetesComponent.Attributes.PutInteger(devfile.ReplicaKey, 1)
				compUpdateRequired = true
			}
		}

		// Update for Port
		var err error
		currentPort := int(kubernetesComponent.Attributes.GetNumber(devfile.ContainerImagePortKey, &err))
		if err != nil {
			if _, ok := err.(*attributes.KeyNotFoundError); !ok {
				return &devfile.DevfileAttributeParse{Key: devfile.ContainerImagePortKey, Err: err}
			}
		}
		if currentPort != component.Spec.TargetPort {
			log.Info(fmt.Sprintf("setting devfile component %s attribute component.Spec.TargetPort %v", kubernetesComponent.Name, component.Spec.TargetPort))
			kubernetesComponent.Attributes = kubernetesComponent.Attributes.PutInteger(devfile.ContainerImagePortKey, component.Spec.TargetPort)
			compUpdateRequired = true
		}

		// Update for Route
		if component.Spec.Route != "" {
			log.Info(fmt.Sprintf("setting devfile component %s attribute component.Spec.Route %s", kubernetesComponent.Name, component.Spec.Route))
			kubernetesComponent.Attributes = kubernetesComponent.Attributes.PutString(devfile.RouteKey, component.Spec.Route)
			compUpdateRequired = true
		}

		// Update for Env
		currentENV := []corev1.EnvVar{}
		err = kubernetesComponent.Attributes.GetInto(devfile.ContainerENVKey, &currentENV)
		if err != nil {
			if _, ok := err.(*attributes.KeyNotFoundError); !ok {
				return &devfile.DevfileAttributeParse{Key: devfile.ContainerENVKey, Err: err}
			}
		}
		for _, env := range component.Spec.Env {
			if env.ValueFrom != nil {
				return &NotSupported{err: fmt.Errorf("env.ValueFrom is not supported at the moment, use env.value")}
			}

			name := env.Name
			value := env.Value
			isPresent := false

			for i, devfileEnv := range currentENV {
				if devfileEnv.Name == name {
					isPresent = true
					log.Info(fmt.Sprintf("setting devfileComponent %s env %s", kubernetesComponent.Name, devfileEnv.Name))
					devfileEnv.Value = value
					currentENV[i] = devfileEnv
				}
			}

			if !isPresent {
				log.Info(fmt.Sprintf("appending to devfile component %s env %s", kubernetesComponent.Name, name))
				currentENV = append(currentENV, env)
			}
			var err error
			kubernetesComponent.Attributes = kubernetesComponent.Attributes.FromMap(map[string]interface{}{devfile.ContainerENVKey: currentENV}, &err)
			if err != nil {
				return &devfile.DevfileAttributeParse{Key: devfile.ContainerENVKey, Err: err}
			}
			compUpdateRequired = true
		}

		// Update for limits
		limits := component.Spec.Resources.Limits
		if len(limits) > 0 {
			// CPU Limit
			resourceCPULimit := limits[corev1.ResourceCPU]
			if resourceCPULimit.String() != "" {
				log.Info(fmt.Sprintf("setting devfile component %s attribute cpu limit to %s", kubernetesComponent.Name, resourceCPULimit.String()))
				kubernetesComponent.Attributes = kubernetesComponent.Attributes.PutString(devfile.CpuLimitKey, resourceCPULimit.String())
				compUpdateRequired = true
			}

			// Memory Limit
			resourceMemoryLimit := limits[corev1.ResourceMemory]
			if resourceMemoryLimit.String() != "" {
				log.Info(fmt.Sprintf("setting devfile component %s attribute memory limit to %s", kubernetesComponent.Name, resourceMemoryLimit.String()))
				kubernetesComponent.Attributes = kubernetesComponent.Attributes.PutString(devfile.MemoryLimitKey, resourceMemoryLimit.String())
				compUpdateRequired = true
			}

			// Storage Limit
			resourceStorageLimit := limits[corev1.ResourceStorage]
			if resourceStorageLimit.String() != "" {
				log.Info(fmt.Sprintf("setting devfile component %s attribute storage limit to %s", kubernetesComponent.Name, resourceStorageLimit.String()))
				kubernetesComponent.Attributes = kubernetesComponent.Attributes.PutString(devfile.StorageLimitKey, resourceStorageLimit.String())
				compUpdateRequired = true
			}
		}

		// Update for requests
		requests := component.Spec.Resources.Requests
		if len(requests) > 0 {
			// CPU Request
			resourceCPURequest := requests[corev1.ResourceCPU]
			if len(kubernetesComponent.Attributes) == 0 {
				kubernetesComponent.Attributes = attributes.Attributes{}
			}
			if resourceCPURequest.String() != "" {
				log.Info(fmt.Sprintf("updating devfile component %s attribute cpu request to %s", kubernetesComponent.Name, resourceCPURequest.String()))
				kubernetesComponent.Attributes = kubernetesComponent.Attributes.PutString(devfile.CpuRequestKey, resourceCPURequest.String())
				compUpdateRequired = true
			}

			// Memory Request
			resourceMemoryRequest := requests[corev1.ResourceMemory]
			if resourceMemoryRequest.String() != "" {
				log.Info(fmt.Sprintf("updating devfile component %s attribute memory request to %s", kubernetesComponent.Name, resourceMemoryRequest.String()))
				kubernetesComponent.Attributes = kubernetesComponent.Attributes.PutString(devfile.MemoryRequestKey, resourceMemoryRequest.String())
				compUpdateRequired = true
			}

			// Storage Request
			resourceStorageRequest := requests[corev1.ResourceStorage]
			if resourceStorageRequest.String() != "" {
				log.Info(fmt.Sprintf("updating devfile component %s attribute storage request to %s", kubernetesComponent.Name, resourceStorageRequest.String()))
				kubernetesComponent.Attributes = kubernetesComponent.Attributes.PutString(devfile.StorageRequestKey, resourceStorageRequest.String())
				compUpdateRequired = true
			}
		}

		if compUpdateRequired {
			// Update the devfileComponent once it has been updated with the Component data
			log.Info(fmt.Sprintf("updating devfile component name %s ...", kubernetesComponent.Name))
			err := hasCompDevfileData.UpdateComponent(kubernetesComponent)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// addComponentsToApplicationDevfileModel updates the Application's devfile model to include all of the
func (r *ApplicationReconciler) addComponentsToApplicationDevfileModel(devSpec *devfileAPIV1.DevWorkspaceTemplateSpec, components []appstudiov1alpha1.Component) error {

	for _, component := range components {
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
			projects := devSpec.Projects
			for _, project := range projects {
				if project.Name == newProject.Name {
					return fmt.Errorf("application already has a component with name %s", newProject.Name)
				}
			}
			devSpec.Projects = append(devSpec.Projects, newProject)
		} else if component.Spec.ContainerImage != "" {
			var err error

			// Add the image as a top level attribute
			devfileAttributes := devSpec.Attributes
			if devfileAttributes == nil {
				devfileAttributes = attributes.Attributes{}
				devSpec.Attributes = devfileAttributes
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

		} else {
			return fmt.Errorf("component source is nil")
		}

	}

	return nil
}

// getAndAddComponentApplicationsToModel retrieves the list of components that belong to the application CR and adds them to the application's devfile model
func (r *ApplicationReconciler) getAndAddComponentApplicationsToModel(log logr.Logger, req reconcile.Request, applicationName string, devSpec *devfileAPIV1.DevWorkspaceTemplateSpec) error {

	// Find all components owned by the application
	var components []appstudiov1alpha1.Component
	var componentList appstudiov1alpha1.ComponentList
	var err error
	err = r.Client.List(ctx, &componentList, &client.ListOptions{
		Namespace: req.NamespacedName.Namespace,
	})
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to list Components for %v", req.NamespacedName))
		metrics.ApplicationCreationFailed.Inc()
		return err
	}

	for _, component := range componentList.Items {
		if component.Spec.Application == applicationName {
			components = append(components, component)
		}
	}

	// Add the components to the Devfile model
	err = r.addComponentsToApplicationDevfileModel(devSpec, components)
	if err != nil {
		// User error - so increment the "success" metric - since we're tracking only system errors
		metrics.ApplicationCreationSucceeded.Inc()
		log.Error(err, fmt.Sprintf("Error adding components to devfile for Application %v", req.NamespacedName))
		return err
	}

	return nil
}

func (r *ComponentDetectionQueryReconciler) updateComponentStub(req ctrl.Request, ctx context.Context, componentDetectionQuery *appstudiov1alpha1.ComponentDetectionQuery, devfilesMap map[string][]byte, devfilesURLMap map[string]string, dockerfileContextMap map[string]string, componentPortsMap map[string][]int) error {

	if componentDetectionQuery == nil {
		return fmt.Errorf("componentDetectionQuery is nil")
	}

	log := r.Log.WithValues("controllerKind", "ComponentDetectionQuery").WithValues("name", req.NamespacedName.Name).WithValues("namespace", req.NamespacedName.Namespace)

	if len(componentDetectionQuery.Status.ComponentDetected) == 0 {
		componentDetectionQuery.Status.ComponentDetected = make(appstudiov1alpha1.ComponentDetectionMap)
	}

	log.Info(fmt.Sprintf("Devfiles detected: %v", len(devfilesMap)))

	for context, devfileBytes := range devfilesMap {
		log.Info(fmt.Sprintf("Currently reading the devfile for context %v", context))
		// Parse the Component Devfile
		compDevfileData, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: devfileBytes})
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

		gitSource := &appstudiov1alpha1.GitSource{
			Context:       context,
			URL:           componentDetectionQuery.Spec.GitSource.URL,
			Revision:      componentDetectionQuery.Spec.GitSource.Revision,
			DevfileURL:    devfilesURLMap[context],
			DockerfileURL: dockerfileContextMap[context],
		}
		componentName := r.getComponentName(log, ctx, req.Namespace, gitSource, componentDetectionQuery.Spec.GenerateComponentName)

		componentStub := appstudiov1alpha1.ComponentSpec{
			ComponentName: componentName,
			Application:   "insert-application-name",
			Source: appstudiov1alpha1.ComponentSource{
				ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
					GitSource: gitSource,
				},
			},
		}

		if len(componentPortsMap[context]) != 0 {
			componentStub.TargetPort = componentPortsMap[context][0]
		}

		// Since a devfile can have N container components, we only try to populate the stub with the first Kubernetes component
		if len(devfileKubernetesComponents) != 0 {
			kubernetesComponentAttribute := devfileKubernetesComponents[0].Attributes

			// Devfile Env
			err := kubernetesComponentAttribute.GetInto(devfile.ContainerENVKey, &componentStub.Env)
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}

			// Devfile Port
			if componentStub.TargetPort == 0 {
				componentStub.TargetPort = int(kubernetesComponentAttribute.GetNumber(devfile.ContainerImagePortKey, &err))
				if err != nil {
					if _, ok := err.(*attributes.KeyNotFoundError); !ok {
						return err
					}
				}
			}

			// Devfile Route
			componentStub.Route = kubernetesComponentAttribute.GetString(devfile.RouteKey, &err)
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}

			// Devfile Replica
			numReplicas := int(kubernetesComponentAttribute.GetNumber(devfile.ReplicaKey, &err))
			componentStub.Replicas = &numReplicas
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
			cpuLimitString := kubernetesComponentAttribute.GetString(devfile.CpuLimitKey, &err)
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
			memoryLimitString := kubernetesComponentAttribute.GetString(devfile.MemoryLimitKey, &err)
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
			storageLimitString := kubernetesComponentAttribute.GetString(devfile.StorageLimitKey, &err)
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
			cpuRequestString := kubernetesComponentAttribute.GetString(devfile.CpuRequestKey, &err)
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
			memoryRequestString := kubernetesComponentAttribute.GetString(devfile.MemoryRequestKey, &err)
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
			storageRequestString := kubernetesComponentAttribute.GetString(devfile.StorageRequestKey, &err)
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

		// Once the Dockerfile has been processed, remove it
		delete(dockerfileContextMap, context)
	}

	log.Info(fmt.Sprintf("Dockerfiles detected: %v", len(dockerfileContextMap)))

	// process the dockefileMap that does not have an associated devfile with it
	for context, link := range dockerfileContextMap {
		log.Info(fmt.Sprintf("Currently reading the Dockerfile for context %v", context))

		gitSource := &appstudiov1alpha1.GitSource{
			Context:       context,
			URL:           componentDetectionQuery.Spec.GitSource.URL,
			Revision:      componentDetectionQuery.Spec.GitSource.Revision,
			DockerfileURL: link,
		}
		componentName := r.getComponentName(log, ctx, req.Namespace, gitSource, componentDetectionQuery.Spec.GenerateComponentName)

		detectComp := appstudiov1alpha1.ComponentDetectionDescription{
			DevfileFound: false, // always false since there is only a Dockerfile present for these contexts
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

		if len(componentPortsMap[context]) != 0 {
			detectComp.ComponentStub.TargetPort = componentPortsMap[context][0]
		}

		componentDetectionQuery.Status.ComponentDetected[componentName] = detectComp

	}

	return nil
}

func (r *ComponentDetectionQueryReconciler) getComponentName(log logr.Logger, ctx context.Context, namespace string, gitSource *appstudiov1alpha1.GitSource, generateComponentName bool) string {
	var componentName string
	repoUrl := gitSource.URL

	generateCompName := false
	if generateComponentName {
		generateCompName = generateComponentName
	}

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
	componentName = sanitizeComponentName(componentName)
	if generateCompName {
		componentName = fmt.Sprintf("%s-%s", componentName, util.GetRandomString(4, true))
	} else {
		compNamespacedName := types.NamespacedName{
			Namespace: namespace,
			Name:      componentName,
		}
		// Fetch the Component instance
		var tempComp appstudiov1alpha1.Component
		err := r.Get(ctx, compNamespacedName, &tempComp)
		if err == nil || !errors.IsNotFound(err) {
			log.Info(fmt.Sprintf("the component %v already exist, appending random chars at the end...", compNamespacedName))
			componentName = fmt.Sprintf("%s-%s", componentName, util.GetRandomString(4, true))
		}
	}

	return componentName
}

// sanitizeComponentName sanitizes component name with the following requirements:
// - Contain at most 63 characters
// - Contain only lowercase alphanumeric characters or ‘-’
// - Start with an alphabet character
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
	if unicode.IsDigit(rune(name[0])) {
		// starts with numeric values, prefix a character
		name = fmt.Sprintf("comp-%s", name)
	}
	name = strings.ToLower(name)
	if len(name) > 58 {
		name = name[0:58]
	}

	return name
}
