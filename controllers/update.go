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

func (r *HASComponentReconciler) updateComponentDevfileModel(hasCompDevfileData data.DevfileData, hasComponent appstudiov1alpha1.HASComponent) (bool, error) {

	log := r.Log.WithValues("HASComponent", "updateComponentDevfileModel")

	components, err := hasCompDevfileData.GetComponents(common.DevfileOptions{
		ComponentOptions: common.ComponentOptions{
			ComponentType: devfileAPIV1.ContainerComponentType,
		},
	})
	if err != nil {
		return false, err
	}

	isUpdated := false

	for i, component := range components {
		compUpdateRequired := false

		// Update for Route
		if hasComponent.Spec.Route != "" {
			currentRoute := ""
			if len(component.Attributes) == 0 {
				component.Attributes = attributes.Attributes{}
			} else {
				var err error
				currentRoute = component.Attributes.GetString("appstudio.has/route", &err)
				if err != nil {
					if _, ok := err.(*attributes.KeyNotFoundError); !ok {
						return false, err
					}
				}
			}
			if currentRoute != hasComponent.Spec.Route {
				log.Info(fmt.Sprintf("updating component %s attribute hasComponent.Spec.Route from %s to %s", component.Name, currentRoute, hasComponent.Spec.Route))
				component.Attributes = component.Attributes.PutString("appstudio.has/route", hasComponent.Spec.Route)
				compUpdateRequired = true
			}
		}

		// Update for Replica
		currentReplica := 0
		if len(component.Attributes) == 0 {
			component.Attributes = attributes.Attributes{}
		} else {
			var err error
			currentReplica = int(component.Attributes.GetNumber("appstudio.has/replicas", &err))
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return false, err
				}
			}
		}
		if currentReplica != hasComponent.Spec.Replicas {
			log.Info(fmt.Sprintf("updating component %s attribute hasComponent.Spec.Replicas from %v to %v", component.Name, currentReplica, hasComponent.Spec.Replicas))
			component.Attributes = component.Attributes.PutInteger("appstudio.has/replicas", hasComponent.Spec.Replicas)
			compUpdateRequired = true
		}

		// Update for Port
		if i == 0 && hasComponent.Spec.TargetPort > 0 {
			for i, endpoint := range component.Container.Endpoints {
				if endpoint.TargetPort != hasComponent.Spec.TargetPort {
					log.Info(fmt.Sprintf("updating component %s endpoint %s port from %v to %v", component.Name, endpoint.Name, endpoint.TargetPort, hasComponent.Spec.TargetPort))
					endpoint.TargetPort = hasComponent.Spec.TargetPort
					compUpdateRequired = true
					component.Container.Endpoints[i] = endpoint
				}
			}
		}

		// Update for Env
		for _, env := range hasComponent.Spec.Env {
			if env.ValueFrom != nil {
				return false, fmt.Errorf("env.ValueFrom is not supported at the moment, use env.value")
			}

			name := env.Name
			value := env.Value
			isPresent := false

			for i, devfileEnv := range component.Container.Env {
				if devfileEnv.Name == name {
					isPresent = true
					if devfileEnv.Value != value {
						log.Info(fmt.Sprintf("updating component %s env %s value from %v to %v", component.Name, devfileEnv.Name, devfileEnv.Value, value))
						devfileEnv.Value = value
						component.Container.Env[i] = devfileEnv
						compUpdateRequired = true
					}
				}
			}

			if !isPresent {
				log.Info(fmt.Sprintf("appending to component %s env %s : %v", component.Name, name, value))
				component.Container.Env = append(component.Container.Env, devfileAPIV1.EnvVar{Name: name, Value: value})
				compUpdateRequired = true
			}

		}

		// Update for limits
		limits := hasComponent.Spec.Resources.Limits
		if len(limits) > 0 {
			// CPU Limit
			resourceCPULimit := limits[corev1.ResourceCPU]
			if resourceCPULimit.String() != "" && component.Container.CpuLimit != resourceCPULimit.String() {
				log.Info(fmt.Sprintf("updating component %s cpu limit from %s to %s", component.Name, component.Container.CpuLimit, resourceCPULimit.String()))
				component.Container.CpuLimit = resourceCPULimit.String()
				compUpdateRequired = true
			}

			// Memory Limit
			resourceMemoryLimit := limits[corev1.ResourceMemory]
			if resourceMemoryLimit.String() != "" && component.Container.MemoryLimit != resourceMemoryLimit.String() {
				log.Info(fmt.Sprintf("updating component %s memory limit from %s to %s", component.Name, component.Container.MemoryLimit, resourceMemoryLimit.String()))
				component.Container.MemoryLimit = resourceMemoryLimit.String()
				compUpdateRequired = true
			}

			// Storage Limit
			resourceStorageLimit := limits[corev1.ResourceStorage]
			currentStorageLimit := "0"
			if len(component.Attributes) == 0 {
				component.Attributes = attributes.Attributes{}
			} else {
				var err error
				currentStorageLimit = component.Attributes.GetString("appstudio.has/storageLimit", &err)
				if err != nil {
					if _, ok := err.(*attributes.KeyNotFoundError); !ok {
						return false, err
					}
				}
				if currentStorageLimit == "" {
					currentStorageLimit = "0"
				}
			}
			if currentStorageLimit != resourceStorageLimit.String() {
				log.Info(fmt.Sprintf("updating component %s attribute storage limit from %s to %s", component.Name, currentStorageLimit, resourceStorageLimit.String()))
				component.Attributes = component.Attributes.PutString("appstudio.has/storageLimit", resourceStorageLimit.String())
				compUpdateRequired = true
			}

			// Ephermetal Storage Limit
			resourceEphermeralStorageLimit := limits[corev1.ResourceEphemeralStorage]
			currentEphemeralStorageLimit := "0"
			if len(component.Attributes) == 0 {
				component.Attributes = attributes.Attributes{}
			} else {
				var err error
				currentEphemeralStorageLimit = component.Attributes.GetString("appstudio.has/ephermealStorageLimit", &err)
				if err != nil {
					if _, ok := err.(*attributes.KeyNotFoundError); !ok {
						return false, err
					}
				}
				if currentEphemeralStorageLimit == "" {
					currentEphemeralStorageLimit = "0"
				}
			}
			if currentEphemeralStorageLimit != resourceEphermeralStorageLimit.String() {
				log.Info(fmt.Sprintf("updating component %s attribute ephermeal storage limit from %s to %s", component.Name, currentEphemeralStorageLimit, resourceEphermeralStorageLimit.String()))
				component.Attributes = component.Attributes.PutString("appstudio.has/ephermealStorageLimit", resourceEphermeralStorageLimit.String())
				compUpdateRequired = true
			}
		}

		// Update for requests
		requests := hasComponent.Spec.Resources.Requests
		if len(requests) > 0 {
			// CPU Request
			resourceCPURequest := requests[corev1.ResourceCPU]
			if resourceCPURequest.String() != "" && component.Container.CpuRequest != resourceCPURequest.String() {
				log.Info(fmt.Sprintf("updating component %s cpu request from %s to %s", component.Name, component.Container.CpuRequest, resourceCPURequest.String()))
				component.Container.CpuRequest = resourceCPURequest.String()
				compUpdateRequired = true
			}

			// Memory Request
			resourceMemoryRequest := requests[corev1.ResourceMemory]
			if resourceMemoryRequest.String() != "" && component.Container.MemoryRequest != resourceMemoryRequest.String() {
				log.Info(fmt.Sprintf("updating component %s memory request from %s to %s", component.Name, component.Container.MemoryRequest, resourceMemoryRequest.String()))
				component.Container.MemoryRequest = resourceMemoryRequest.String()
				compUpdateRequired = true
			}

			// Storage Request
			resourceStorageRequest := requests[corev1.ResourceStorage]
			currentStorageRequest := "0"
			if len(component.Attributes) == 0 {
				component.Attributes = attributes.Attributes{}
			} else {
				var err error
				currentStorageRequest = component.Attributes.GetString("appstudio.has/storageRequest", &err)
				if err != nil {
					if _, ok := err.(*attributes.KeyNotFoundError); !ok {
						return false, err
					}
				}
				if currentStorageRequest == "" {
					currentStorageRequest = "0"
				}
			}
			if currentStorageRequest != resourceStorageRequest.String() {
				log.Info(fmt.Sprintf("updating component %s attribute storage request from %s to %s", component.Name, currentStorageRequest, resourceStorageRequest.String()))
				component.Attributes = component.Attributes.PutString("appstudio.has/storageRequest", resourceStorageRequest.String())
				compUpdateRequired = true
			}

			// Ephermetal Storage Request
			resourceEphermeralStorageRequest := requests[corev1.ResourceEphemeralStorage]
			currentEphemeralStorageRequest := "0"
			if len(component.Attributes) == 0 {
				component.Attributes = attributes.Attributes{}
			} else {
				var err error
				currentEphemeralStorageRequest = component.Attributes.GetString("appstudio.has/ephermealStorageRequest", &err)
				if err != nil {
					if _, ok := err.(*attributes.KeyNotFoundError); !ok {
						return false, err
					}
				}
				if currentEphemeralStorageRequest == "" {
					currentEphemeralStorageRequest = "0"
				}
			}
			if currentEphemeralStorageRequest != resourceEphermeralStorageRequest.String() {
				log.Info(fmt.Sprintf("updating component %s attribute ephermeal storage limit from %s to %s", component.Name, currentEphemeralStorageRequest, resourceEphermeralStorageRequest.String()))
				component.Attributes = component.Attributes.PutString("appstudio.has/ephermealStorageRequest", resourceEphermeralStorageRequest.String())
				compUpdateRequired = true
			}
		}

		if compUpdateRequired {
			// Update the component once it has been updated with the HAS Component data
			log.Info(fmt.Sprintf("updating component name %s ...", component.Name))
			err := hasCompDevfileData.UpdateComponent(component)
			if err != nil {
				return false, err
			}
			isUpdated = true
		}
	}

	return isUpdated, nil
}

func (r *HASComponentReconciler) updateApplicationDevfileModel(hasAppDevfileData data.DevfileData, hasComponent appstudiov1alpha1.HASComponent) error {

	newProject := devfileAPIV1.Project{
		Name: hasComponent.Spec.ComponentName,
		ProjectSource: devfileAPIV1.ProjectSource{
			Git: &devfileAPIV1.GitProjectSource{
				GitLikeProjectSource: devfileAPIV1.GitLikeProjectSource{
					Remotes: map[string]string{
						"origin": hasComponent.Spec.Source.GitSource.URL,
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
			return fmt.Errorf("HASApplication already has a project with name %s", newProject.Name)
		}
	}
	err = hasAppDevfileData.AddProjects([]devfileAPIV1.Project{newProject})
	if err != nil {
		return err
	}

	return nil
}
