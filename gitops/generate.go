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

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops/prepare"
	gitopsgen "github.com/redhat-developer/gitops-generator/pkg"
	"github.com/spf13/afero"
)

const (
	kustomizeFileName = "kustomization.yaml"
)

func GenerateTektonBuild(outputPath string, component appstudiov1alpha1.Component, appFs afero.Afero, context string, gitopsConfig prepare.GitopsConfig) error {
	componentName := component.Name
	repoPath := filepath.Join(outputPath, componentName)
	gitopsFolder := filepath.Join(repoPath, context)
	componentPath := filepath.Join(gitopsFolder, "components", componentName, "base")

	if component.Spec.Source.GitSource != nil && component.Spec.Source.GitSource.URL != "" {
		tektonResourcesDirName := ".tekton"

		if err := GenerateBuild(appFs, filepath.Join(componentPath, tektonResourcesDirName), component, gitopsConfig); err != nil {
			return fmt.Errorf("failed to generate tekton build in %q for component %q: %s", componentPath, componentName, err)
		}
		// Update the kustomize file and return
		if err := gitopsgen.UpdateExistingKustomize(appFs, componentPath); err != nil {
			return fmt.Errorf("failed to update kustomize file for tekton build in %q for component %q: %s", componentPath, componentName, err)
		}
	}
	return nil
}
