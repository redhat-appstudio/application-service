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
	"os/exec"
	"path/filepath"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops/prepare"
	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
)

type Executor interface {
	Execute(baseDir, command string, args ...string) ([]byte, error)
	GenerateParentKustomize(fs afero.Afero, gitOpsFolder string, commonStoragePVC *corev1.PersistentVolumeClaim) error
}

// GenerateAndPush takes in the following args and generates the gitops resources for a given component
// 1. outputPath: Where to output the gitops resources to
// 2. remote: A string of the form https://$token@github.com/<org>/<repo>. Corresponds to the component's gitops repository
// 2. component: A component struct corresponding to a single Component in an Application in AS
// 4. The executor to use to execute the git commands (either gitops.executor or gitops.mockExecutor)
// 5. The filesystem object used to create (either ioutils.NewFilesystem() or ioutils.NewMemoryFilesystem())
// 6. The branch to push to
// 7. The path within the repository to generate the resources in
// 8. The gitops config containing the build bundle;
// Adapted from https://github.com/redhat-developer/kam/blob/master/pkg/pipelines/utils.go#L79
func GenerateAndPush(outputPath string, remote string, component appstudiov1alpha1.Component, e Executor, appFs afero.Afero, branch string, context string, gitopsConfig prepare.GitopsConfig) error {
	componentName := component.Name
	if out, err := e.Execute(outputPath, "git", "clone", remote, componentName); err != nil {
		return fmt.Errorf("failed to clone git repository in %q %q: %s", outputPath, string(out), err)
	}

	repoPath := filepath.Join(outputPath, componentName)

	// Checkout the specified branch
	if _, err := e.Execute(repoPath, "git", "switch", branch); err != nil {
		if out, err := e.Execute(repoPath, "git", "checkout", "-b", branch); err != nil {
			return fmt.Errorf("failed to checkout branch %q in %q %q: %s", branch, repoPath, string(out), err)
		}
	}

	if out, err := e.Execute(repoPath, "rm", "-rf", filepath.Join("components", componentName)); err != nil {
		return fmt.Errorf("failed to delete %q folder in repository in %q %q: %s", filepath.Join("components", componentName), repoPath, string(out), err)
	}

	// Generate the gitops resources and update the parent kustomize yaml file
	gitopsFolder := filepath.Join(repoPath, context)
	componentPath := filepath.Join(gitopsFolder, "components", componentName, "base")
	if err := Generate(appFs, gitopsFolder, componentPath, component, gitopsConfig); err != nil {
		return fmt.Errorf("failed to generate the gitops resources in %q for component %q: %s", componentPath, componentName, err)
	}

	if out, err := e.Execute(repoPath, "git", "add", "."); err != nil {
		return fmt.Errorf("failed to add files for component %q to repository in %q %q: %s", componentName, repoPath, string(out), err)
	}

	// See if any files changed, and if so, commit and push them up to the repository
	if out, err := e.Execute(repoPath, "git", "--no-pager", "diff", "--cached"); err != nil {
		return fmt.Errorf("failed to check git diff in repository %q %q: %s", repoPath, string(out), err)
	} else if string(out) != "" {
		// Commit the changes and push
		if out, err := e.Execute(repoPath, "git", "commit", "-m", fmt.Sprintf("Generate GitOps base resources for component %s", componentName)); err != nil {
			return fmt.Errorf("failed to commit files to repository in %q %q: %s", repoPath, string(out), err)
		}
		if out, err := e.Execute(repoPath, "git", "push", "origin", branch); err != nil {
			return fmt.Errorf("failed push remote to repository %q %q: %s", remote, string(out), err)
		}
	}

	return nil
}

// GenerateOverlaysAndPush generates the overlays kustomize from App Env Snapshot Binding Spec
func GenerateOverlaysAndPush(outputPath string, clone bool, remote string, component appstudioshared.BindingComponent, applicationName, environmentName, imageName, namespace string, e Executor, appFs afero.Afero, branch string, context string, componentGeneratedResources map[string][]string) error {
	componentName := component.Name
	repoPath := filepath.Join(outputPath, applicationName)

	if clone {
		if out, err := e.Execute(outputPath, "git", "clone", remote, applicationName); err != nil {
			return fmt.Errorf("failed to clone git repository in %q %q: %s", outputPath, string(out), err)
		}

		// Checkout the specified branch
		if _, err := e.Execute(repoPath, "git", "switch", branch); err != nil {
			if out, err := e.Execute(repoPath, "git", "checkout", "-b", branch); err != nil {
				return fmt.Errorf("failed to checkout branch %q in %q %q: %s", branch, repoPath, string(out), err)
			}
		}
	}

	// Generate the gitops resources and update the parent kustomize yaml file
	gitopsFolder := filepath.Join(repoPath, context)
	componentEnvOverlaysPath := filepath.Join(gitopsFolder, "components", componentName, "overlays", environmentName)
	if err := GenerateOverlays(appFs, gitopsFolder, componentEnvOverlaysPath, component, imageName, namespace, componentGeneratedResources); err != nil {
		return fmt.Errorf("failed to generate the gitops resources in overlays dir %q for component %q: %s", componentEnvOverlaysPath, componentName, err)
	}

	if out, err := e.Execute(repoPath, "git", "add", "."); err != nil {
		return fmt.Errorf("failed to add files for component %q to repository in %q %q: %s", componentName, repoPath, string(out), err)
	}

	// See if any files changed, and if so, commit and push them up to the repository
	if out, err := e.Execute(repoPath, "git", "--no-pager", "diff", "--cached"); err != nil {
		return fmt.Errorf("failed to check git diff in repository %q %q: %s", repoPath, string(out), err)
	} else if string(out) != "" {
		// Commit the changes and push
		if out, err := e.Execute(repoPath, "git", "commit", "-m", fmt.Sprintf("Generate %s environment overlays for component %s", environmentName, componentName)); err != nil {
			return fmt.Errorf("failed to commit files to repository in %q %q: %s", repoPath, string(out), err)
		}
		if out, err := e.Execute(repoPath, "git", "push", "origin", branch); err != nil {
			return fmt.Errorf("failed push remote to repository %q %q: %s", remote, string(out), err)
		}
	}

	return nil
}

// RemoveAndPush takes in the following args and updates the gitops resources by removing the given component
// 1. outputPath: Where to output the gitops resources to
// 2. remote: A string of the form https://$token@github.com/<org>/<repo>. Corresponds to the component's gitops repository
// 2. component: A component struct corresponding to a single Component in an Application in AS
// 4. The executor to use to execute the git commands (either gitops.executor or gitops.mockExecutor)
// 5. The filesystem object used to create (either ioutils.NewFilesystem() or ioutils.NewMemoryFilesystem())
// 6. The branch to push to
// 7. The path within the repository to generate the resources in
func RemoveAndPush(outputPath string, remote string, component appstudiov1alpha1.Component, e Executor, appFs afero.Afero, branch string, context string) error {
	componentName := component.Name
	if out, err := e.Execute(outputPath, "git", "clone", remote, componentName); err != nil {
		return fmt.Errorf("failed to clone git repository in %q %q: %s", outputPath, string(out), err)
	}

	repoPath := filepath.Join(outputPath, componentName)

	// Checkout the specified branch
	if _, err := e.Execute(repoPath, "git", "switch", branch); err != nil {
		if out, err := e.Execute(repoPath, "git", "checkout", "-b", branch); err != nil {
			return fmt.Errorf("failed to checkout branch %q in %q %q: %s", branch, repoPath, string(out), err)
		}
	}

	// Generate the gitops resources and update the parent kustomize yaml file
	gitopsFolder := filepath.Join(repoPath, context)
	componentPath := filepath.Join(gitopsFolder, "components", componentName)
	if out, err := e.Execute(repoPath, "rm", "-rf", componentPath); err != nil {
		return fmt.Errorf("failed to delete %q folder in repository in %q %q: %s", componentPath, repoPath, string(out), err)
	}
	if err := e.GenerateParentKustomize(appFs, gitopsFolder, nil); err != nil {
		return fmt.Errorf("failed to re-generate the gitops resources in %q for component %q: %s", componentPath, componentName, err)
	}

	if out, err := e.Execute(repoPath, "git", "add", "."); err != nil {
		return fmt.Errorf("failed to add files for component %q to repository in %q %q: %s", componentName, repoPath, string(out), err)
	}

	// See if any files changed, and if so, commit and push them up to the repository
	if out, err := e.Execute(repoPath, "git", "--no-pager", "diff", "--cached"); err != nil {
		return fmt.Errorf("failed to check git diff in repository %q %q: %s", repoPath, string(out), err)
	} else if string(out) != "" {
		// Commit the changes and push
		if out, err := e.Execute(repoPath, "git", "commit", "-m", fmt.Sprintf("Removed component %s", componentName)); err != nil {
			return fmt.Errorf("failed to commit files to repository in %q %q: %s", repoPath, string(out), err)
		}
		if out, err := e.Execute(repoPath, "git", "push", "origin", branch); err != nil {
			return fmt.Errorf("failed push remote to repository %q %q: %s", remote, string(out), err)
		}
	}

	return nil
}

// NewCmdExecutor creates and returns an executor implementation that uses
// exec.Command to execute the commands.
func NewCmdExecutor() CmdExecutor {
	return CmdExecutor{}
}

type CmdExecutor struct {
}

func (e CmdExecutor) Execute(baseDir, command string, args ...string) ([]byte, error) {
	c := exec.Command(command, args...)
	c.Dir = baseDir
	output, err := c.CombinedOutput()
	return output, err
}

func (e CmdExecutor) GenerateParentKustomize(fs afero.Afero, gitOpsFolder string, commonStoragePVC *corev1.PersistentVolumeClaim) error {
	return GenerateParentKustomize(fs, gitOpsFolder, commonStoragePVC)
}
