//
// Copyright 2023 Red Hat, Inc.
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

package generate

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"

	devfileParser "github.com/devfile/library/v2/pkg/devfile/parser"
	"github.com/devfile/library/v2/pkg/devfile/parser/data"
	"github.com/go-logr/logr"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/gitops-generator/pkg/devfile"
	"github.com/redhat-appstudio/application-service/gitops-generator/pkg/util"
	gitopsgenv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
	gitopsgen "github.com/redhat-developer/gitops-generator/pkg"
	"github.com/redhat-developer/gitops-generator/pkg/util/ioutils"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type GitOpsGenParams struct {
	Generator   gitopsgen.Generator
	DevfileData data.DevfileData
	RemoteURL   string
	Branch      string
	Context     string
	Token       string
}

func GenerateGitopsBase(ctx context.Context, log logr.Logger, client ctrlclient.Client, component appstudiov1alpha1.Component, appFs afero.Afero, gitopsParams GitOpsGenParams) error {
	// Create a temp folder to create the gitops resources in
	tempDir, err := ioutils.CreateTempPath(component.Name, appFs)
	if err != nil {
		return fmt.Errorf("unable to create temp directory for GitOps resources due to error: %v", err)
	}

	devfileSrc := devfile.DevfileSrc{
		Data: component.Status.Devfile,
	}
	compDevfileData, err := devfile.ParseDevfile(devfileSrc)
	if err != nil {
		return err
	}

	deployAssociatedComponents, err := devfileParser.GetDeployComponents(compDevfileData)
	if err != nil {
		log.Error(err, "unable to get deploy components")
		return err
	}

	kubernetesResources, err := devfile.GetResourceFromDevfile(log, compDevfileData, deployAssociatedComponents, component.Name, component.Spec.Application, component.Spec.ContainerImage, "")
	if err != nil {
		log.Error(err, "unable to get kubernetes resources from the devfile outerloop components")
		return err
	}

	// Generate and push the gitops resources
	mappedGitOpsComponent := util.GetMappedGitOpsComponent(component, kubernetesResources)

	//add the token name to the metrics.  When we add more tokens and rotate, we can determine how evenly distributed the requests are
	err = gitopsParams.Generator.CloneGenerateAndPush(tempDir, gitopsParams.RemoteURL, mappedGitOpsComponent, appFs, gitopsParams.Branch, gitopsParams.Context, false)
	if err != nil {
		log.Error(err, "unable to generate gitops resources due to error")
		return err
	}

	//Gitops functions return sanitized error messages
	err = gitopsParams.Generator.CommitAndPush(tempDir, "", gitopsParams.RemoteURL, mappedGitOpsComponent.Name, gitopsParams.Branch, "Generating GitOps resources")
	if err != nil {
		log.Error(err, "unable to commit and push gitops resources due to error")
		return err
	}

	// Get the commit ID for the gitops repository
	var commitID string
	repoPath := filepath.Join(tempDir, component.Name)
	if commitID, err = gitopsParams.Generator.GetCommitIDFromRepo(appFs, repoPath); err != nil {
		log.Error(err, "")
		return err
	}

	component.Status.GitOps.CommitID = commitID

	// Remove the temp folder that was created
	return appFs.RemoveAll(tempDir)
}

func GenerateGitopsOverlays(ctx context.Context, log logr.Logger, client ctrlclient.Client, appSnapshotEnvBinding appstudiov1alpha1.SnapshotEnvironmentBinding, appFs afero.Afero, gitopsParams GitOpsGenParams) error {
	// Create a temp folder to create the gitops resources in

	applicationName := appSnapshotEnvBinding.Spec.Application
	environmentName := appSnapshotEnvBinding.Spec.Environment
	snapshotName := appSnapshotEnvBinding.Spec.Snapshot
	components := appSnapshotEnvBinding.Spec.Components

	// Get the Environment CR
	environment := appstudiov1alpha1.Environment{}
	err := client.Get(ctx, types.NamespacedName{Name: environmentName, Namespace: appSnapshotEnvBinding.Namespace}, &environment)
	if err != nil {
		return fmt.Errorf("unable to get the Environment %s", environmentName)
	}

	// Get the Snapshot CR
	appSnapshot := appstudiov1alpha1.Snapshot{}
	err = client.Get(ctx, types.NamespacedName{Name: snapshotName, Namespace: appSnapshotEnvBinding.Namespace}, &appSnapshot)
	if err != nil {
		return fmt.Errorf("unable to get the Application Snapshot %s", snapshotName)
	}
	if appSnapshot.Spec.Application != applicationName {
		return fmt.Errorf("application snapshot %s does not belong to the application %s", snapshotName, applicationName)
	}

	componentGeneratedResources := make(map[string][]string)
	var tempDir string
	clone := true

	for _, component := range components {
		componentName := component.Name

		// Get the Component CR
		hasComponent := appstudiov1alpha1.Component{}
		err = client.Get(ctx, types.NamespacedName{Name: componentName, Namespace: appSnapshotEnvBinding.Namespace}, &hasComponent)
		if err != nil {
			return fmt.Errorf("unable to get the Component %s", componentName)
		}

		if hasComponent.Spec.SkipGitOpsResourceGeneration {
			continue
		}

		// Sanity check to make sure the binding component has referenced the correct application
		if hasComponent.Spec.Application != applicationName {
			return fmt.Errorf("component %s does not belong to the application %s", componentName, applicationName)
		}

		var clusterIngressDomain string
		isKubernetesCluster := isKubernetesCluster(environment)
		unsupportedConfig := environment.Spec.UnstableConfigurationFields
		if unsupportedConfig != nil {
			clusterIngressDomain = unsupportedConfig.IngressDomain
		}

		// Safeguard if Ingress Domain is empty on Kubernetes
		if isKubernetesCluster && clusterIngressDomain == "" {
			err = fmt.Errorf("ingress domain cannot be empty on a Kubernetes cluster")
			return err
		}

		devfileSrc := devfile.DevfileSrc{
			Data: hasComponent.Status.Devfile,
		}
		compDevfileData, err := devfile.ParseDevfile(devfileSrc)
		if err != nil {
			return err
		}

		deployAssociatedComponents, err := devfileParser.GetDeployComponents(compDevfileData)
		if err != nil {
			return err
		}

		var hostname string
		if isKubernetesCluster {
			hostname, err = devfile.GetIngressHostName(hasComponent.Name, appSnapshotEnvBinding.Namespace, clusterIngressDomain)
			if err != nil {
				return err
			}
		}

		kubernetesResources, err := devfile.GetResourceFromDevfile(log, compDevfileData, deployAssociatedComponents, hasComponent.Name, hasComponent.Spec.Application, hasComponent.Spec.ContainerImage, hostname)
		if err != nil {
			return err
		}

		// Create a random, generated name for the route
		// ToDo: Ideally we wouldn't need to loop here, but since the Component status is a list, we can't avoid it
		var routeName string
		for _, compStatus := range appSnapshotEnvBinding.Status.Components {
			if compStatus.Name == componentName {
				if compStatus.GeneratedRouteName != "" {
					routeName = compStatus.GeneratedRouteName
					log.Info(fmt.Sprintf("route name for component is %s", routeName))
				}
				break
			}
		}
		if routeName == "" {
			routeName = util.GenerateRandomRouteName(hasComponent.Name)
			log.Info(fmt.Sprintf("generated route name %s", routeName))
		}

		// If a route is present, update the first instance's name
		if len(kubernetesResources.Routes) > 0 {
			kubernetesResources.Routes[0].ObjectMeta.Name = routeName
		}

		var imageName string

		for _, snapshotComponent := range appSnapshot.Spec.Components {
			if snapshotComponent.Name == componentName {
				imageName = snapshotComponent.ContainerImage
				break
			}
		}

		if imageName == "" {
			err := fmt.Errorf("application snapshot %s did not reference component %s", snapshotName, componentName)
			return err
		}

		gitOpsRemoteURL, gitOpsBranch, gitOpsContext, err := util.ProcessGitOpsStatus(hasComponent.Status.GitOps, gitopsParams.Token)
		if err != nil {
			return err
		}

		if clone {
			// Create a temp folder to create the gitops resources in
			tempDir, err = ioutils.CreateTempPath(appSnapshotEnvBinding.Name, appFs)
			if err != nil {
				return fmt.Errorf("unable to create temp directory for gitops resources due to error: %v", err)
			}
		}

		envVars := make([]corev1.EnvVar, 0)
		for _, env := range component.Configuration.Env {
			envVars = append(envVars, corev1.EnvVar{
				Name:  env.Name,
				Value: env.Value,
			})
		}

		environmentConfigEnvVars := make([]corev1.EnvVar, 0)
		for _, env := range environment.Spec.Configuration.Env {
			environmentConfigEnvVars = append(environmentConfigEnvVars, corev1.EnvVar{
				Name:  env.Name,
				Value: env.Value,
			})
		}
		componentResources := corev1.ResourceRequirements{}
		if component.Configuration.Resources != nil {
			componentResources = *component.Configuration.Resources
		}

		kubeLabels := map[string]string{
			"app.kubernetes.io/name":       componentName,
			"app.kubernetes.io/instance":   component.Name,
			"app.kubernetes.io/part-of":    applicationName,
			"app.kubernetes.io/managed-by": "kustomize",
			"app.kubernetes.io/created-by": "application-service",
		}
		genOptions := gitopsgenv1alpha1.GeneratorOptions{
			Name:                component.Name,
			RouteName:           routeName,
			Resources:           componentResources,
			BaseEnvVar:          envVars,
			OverlayEnvVar:       environmentConfigEnvVars,
			K8sLabels:           kubeLabels,
			IsKubernetesCluster: isKubernetesCluster,
			TargetPort:          hasComponent.Spec.TargetPort, // pass the target port to the gitops gen library as they may generate a route/ingress based on the target port if the devfile does not have an ingress/route or an endpoint
		}

		if component.Configuration.Replicas != nil {
			genOptions.Replicas = *component.Configuration.Replicas
		}

		if !reflect.DeepEqual(kubernetesResources, devfileParser.KubernetesResources{}) {
			genOptions.KubernetesResources.Routes = append(genOptions.KubernetesResources.Routes, kubernetesResources.Routes...)
			genOptions.KubernetesResources.Ingresses = append(genOptions.KubernetesResources.Ingresses, kubernetesResources.Ingresses...)
		}

		if isKubernetesCluster && len(genOptions.KubernetesResources.Ingresses) == 0 {
			// provide the hostname for the component if there are no ingresses
			// Gitops Generator Library will create the Ingress with the hostname
			genOptions.Route = hostname
		}

		//Gitops functions return sanitized error messages
		err = gitopsParams.Generator.GenerateOverlaysAndPush(tempDir, clone, gitOpsRemoteURL, genOptions, applicationName, environmentName, imageName, "", appFs, gitOpsBranch, gitOpsContext, true, componentGeneratedResources)
		if err != nil {
			return err
		}

		// Retrieve the commit ID
		var commitID string
		repoPath := filepath.Join(tempDir, applicationName)
		if commitID, err = gitopsParams.Generator.GetCommitIDFromRepo(appFs, repoPath); err != nil {
			return err
		}

		// Set the BindingComponent status
		componentStatus := appstudiov1alpha1.BindingComponentStatus{
			Name: componentName,
			GitOpsRepository: appstudiov1alpha1.BindingComponentGitOpsRepository{
				URL:      hasComponent.Status.GitOps.RepositoryURL,
				Branch:   gitOpsBranch,
				Path:     filepath.Join(gitOpsContext, "components", componentName, "overlays", environmentName),
				CommitID: commitID,
			},
		}

		// On OpenShift, we generate a unique route name for each Component, so include that in the status
		if !isKubernetesCluster {
			componentStatus.GeneratedRouteName = routeName
			log.Info(fmt.Sprintf("added RouteName %s for Component %s to status", routeName, componentName))
		}

		if _, ok := componentGeneratedResources[componentName]; ok {
			componentStatus.GitOpsRepository.GeneratedResources = componentGeneratedResources[componentName]
		}

		isNewComponent := true
		for i := range appSnapshotEnvBinding.Status.Components {
			if appSnapshotEnvBinding.Status.Components[i].Name == componentStatus.Name {
				appSnapshotEnvBinding.Status.Components[i] = componentStatus
				isNewComponent = false
				break
			}
		}
		if isNewComponent {
			appSnapshotEnvBinding.Status.Components = append(appSnapshotEnvBinding.Status.Components, componentStatus)
		}

		// Set the clone to false, since we dont want to clone the repo again for the other components
		clone = false

	}

	// Update the binding status to reflect the GitOps data
	err = client.Status().Update(ctx, &appSnapshotEnvBinding)
	if err != nil {
		return err
	}

	return appFs.RemoveAll(tempDir)
}

// isKubernetesCluster checks if its either a Kubernetes or an OpenShift cluster
// from the Environment custom resource
func isKubernetesCluster(environment appstudiov1alpha1.Environment) bool {
	unstableConfig := environment.Spec.UnstableConfigurationFields

	if unstableConfig != nil {
		if unstableConfig.ClusterType == appstudiov1alpha1.ConfigurationClusterType_Kubernetes {
			return true
		}
	}

	return false
}
