package component

import (
	"fmt"

	devfileAPIV1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/devfile"
	corev1 "k8s.io/api/core/v1"
)

func (a *Adapter) updateComponentDevfileModel(hasCompDevfileData data.DevfileData, component appstudiov1alpha1.Component) error {

	log := a.Log.WithValues("Component", a.NamespacedName)

	// If DockerfileURL is set and the devfile contains references to a Dockerfile then update the devfile
	source := component.Spec.Source
	var err error
	if source.GitSource != nil && source.GitSource.DockerfileURL != "" {
		hasCompDevfileData, err = devfile.UpdateLocalDockerfileURItoAbsolute(hasCompDevfileData, source.GitSource.DockerfileURL)
		if err != nil {
			return fmt.Errorf("unable to convert local Dockerfile URIs to absolute in Component devfile %v", a.NamespacedName)
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
		currentReplica := 0
		if len(kubernetesComponent.Attributes) == 0 {
			kubernetesComponent.Attributes = attributes.Attributes{}
		} else {
			var err error
			currentReplica = int(kubernetesComponent.Attributes.GetNumber(devfile.ReplicaKey, &err))
			if err != nil {
				if _, ok := err.(*attributes.KeyNotFoundError); !ok {
					return err
				}
			}
		}
		if currentReplica != component.Spec.Replicas {
			log.Info(fmt.Sprintf("setting devfile component %s attribute component.Spec.Replicas to %v", kubernetesComponent.Name, component.Spec.Replicas))
			kubernetesComponent.Attributes = kubernetesComponent.Attributes.PutInteger(devfile.ReplicaKey, component.Spec.Replicas)
			compUpdateRequired = true
		}

		// Update for Port
		var err error
		currentPort := int(kubernetesComponent.Attributes.GetNumber(devfile.ContainerImagePortKey, &err))
		if err != nil {
			if _, ok := err.(*attributes.KeyNotFoundError); !ok {
				return err
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
					log.Info(fmt.Sprintf("setting devfileComponent %s env %s value to %v", kubernetesComponent.Name, devfileEnv.Name, value))
					devfileEnv.Value = value
					currentENV[i] = devfileEnv
				}
			}

			if !isPresent {
				log.Info(fmt.Sprintf("appending to devfile component %s env %s : %v", kubernetesComponent.Name, name, value))
				currentENV = append(currentENV, env)
			}
			var err error
			kubernetesComponent.Attributes = kubernetesComponent.Attributes.FromMap(map[string]interface{}{devfile.ContainerENVKey: currentENV}, &err)
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
