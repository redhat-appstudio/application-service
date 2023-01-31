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

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	gitops "github.com/redhat-appstudio/application-service/gitops-generator/pkg/gitops"
	"github.com/redhat-appstudio/application-service/gitops-generator/pkg/gitops/prepare"
	"github.com/redhat-appstudio/application-service/pkg/util"
	gitopsgenv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
	gitopsgen "github.com/redhat-developer/gitops-generator/pkg"
	gitopsutil "github.com/redhat-developer/gitops-generator/pkg/util"
	"github.com/redhat-developer/gitops-generator/pkg/util/ioutils"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type GitOpsGenParams struct {
	Generator gitopsgen.Generator
	RemoteURL string
	Branch    string
	Context   string
	Token     string
}

func GenerateGitopsBase(ctx context.Context, client ctrlclient.Client, component appstudiov1alpha1.Component, appFs afero.Afero, gitopsParams GitOpsGenParams) error {
	// Create a temp folder to create the gitops resources in
	tempDir, err := ioutils.CreateTempPath(component.Name, appFs)
	if err != nil {
		return fmt.Errorf("unable to create temp directory for GitOps resources due to error: %v", err)
	}

	deployAssociatedComponents, err := devfileParser.GetDeployComponents(compDevfileData)
	if err != nil {
		log.Error(err, "unable to get deploy components")
		return err
	}

	kubernetesResources, err := devfile.GetResourceFromDevfile(log, compDevfileData, deployAssociatedComponents, component.Name, component.Spec.ContainerImage)
	if err != nil {
		log.Error(err, "unable to get kubernetes resources from the devfile outerloop components")
		return err
	}

	// Generate and push the gitops resources
	gitopsConfig := prepare.PrepareGitopsConfig(ctx, r.Client, *component)
	mappedGitOpsComponent := util.GetMappedGitOpsComponent(*component, kubernetesResources)

	err = r.Generator.CloneGenerateAndPush(tempDir, gitOpsURL, mappedGitOpsComponent, r.AppFS, gitOpsBranch, gitOpsContext, false)
	if err != nil {
		log.Error(err, "unable to generate gitops resources due to error")
		return err
	}

	// Generate the Tekton resources and commit and push to GitOps repository
	err = gitops.GenerateTektonBuild(tempDir, component, appFs, gitopsParams.Context, gitopsConfig)
	if err != nil {
		return err
	}
	err = gitopsParams.Generator.CommitAndPush(tempDir, "", gitopsParams.RemoteURL, mappedGitOpsComponent.Name, gitopsParams.Branch, "Generating GitOps resources")
	if err != nil {
		return err
	}
	return nil
}

func GenerateGitopsOverlays(ctx context.Context, client ctrlclient.Client, appSnapshotEnvBinding appstudiov1alpha1.SnapshotEnvironmentBinding, appFs afero.Afero, gitopsParams GitOpsGenParams) error {
	// Create a temp folder to create the gitops resources in
	tempDir, err := ioutils.CreateTempPath(appSnapshotEnvBinding.Name, appFs)
	if err != nil {
		return fmt.Errorf("unable to create temp directory for GitOps resources due to error: %v", err)
	}

	patch := ctrlclient.MergeFrom(appSnapshotEnvBinding.DeepCopy())

	applicationName := appSnapshotEnvBinding.Spec.Application
	environmentName := appSnapshotEnvBinding.Spec.Environment
	snapshotName := appSnapshotEnvBinding.Spec.Snapshot
	components := appSnapshotEnvBinding.Spec.Components

	// Get the Environment CR
	environment := appstudiov1alpha1.Environment{}
	err = client.Get(ctx, types.NamespacedName{Name: environmentName, Namespace: appSnapshotEnvBinding.Namespace}, &environment)
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

		var imageName string
		for _, snapshotComponent := range appSnapshot.Spec.Components {
			if snapshotComponent.Name == componentName {
				imageName = snapshotComponent.ContainerImage
				break
			}
		}

		if imageName == "" {
			return fmt.Errorf("application snapshot %s did not reference component %s", snapshotName, componentName)
		}

		gitOpsRemoteURL, gitOpsBranch, gitOpsContext, err := util.ProcessGitOpsStatus(hasComponent.Status.GitOps, gitopsParams.Token)
		if err != nil {
			return err
		}
		isStatusUpdated := false
		for _, bindingStatusComponent := range appSnapshotEnvBinding.Status.Components {
			if bindingStatusComponent.Name == componentName {
				isStatusUpdated = true
				break
			}
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
			Name:          component.Name,
			Replicas:      component.Configuration.Replicas,
			Resources:     componentResources,
			BaseEnvVar:    envVars,
			OverlayEnvVar: environmentConfigEnvVars,
			K8sLabels:     kubeLabels,
		}
		err = gitopsParams.Generator.GenerateOverlaysAndPush(tempDir, clone, gitOpsRemoteURL, genOptions, applicationName, environmentName, imageName, appSnapshotEnvBinding.Namespace, appFs, gitOpsBranch, gitOpsContext, true, componentGeneratedResources)
		if err != nil {
			gitOpsErr := gitopsutil.SanitizeErrorMessage(err)
			return gitOpsErr
		}

		// Retrieve the commit ID
		var commitID string
		repoPath := filepath.Join(tempDir, applicationName)
		if commitID, err = gitopsParams.Generator.GetCommitIDFromRepo(appFs, repoPath); err != nil {
			gitOpsErr := gitopsutil.SanitizeErrorMessage(err)
			return gitOpsErr
		}

		if !isStatusUpdated {
			componentStatus := appstudiov1alpha1.BindingComponentStatus{
				Name: componentName,
				GitOpsRepository: appstudiov1alpha1.BindingComponentGitOpsRepository{
					URL:      hasComponent.Status.GitOps.RepositoryURL,
					Branch:   gitOpsBranch,
					Path:     filepath.Join(gitOpsContext, "components", componentName, "overlays", environmentName),
					CommitID: commitID,
				},
			}

			if _, ok := componentGeneratedResources[componentName]; ok {
				componentStatus.GitOpsRepository.GeneratedResources = componentGeneratedResources[componentName]
			}

			appSnapshotEnvBinding.Status.Components = append(appSnapshotEnvBinding.Status.Components, componentStatus)
		}

		// Set the clone to false, since we dont want to clone the repo again for the other components
		clone = false

		// Update the status in the resource via a patch operation
		err = client.Status().Patch(ctx, &appSnapshotEnvBinding, patch)
		if err != nil {
			return err
		}

	}
	return nil

}
