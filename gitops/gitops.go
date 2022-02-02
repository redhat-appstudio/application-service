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
	"github.com/spf13/afero"
)

type GitopsGeneratorFunc func(fs afero.Fs, outputFolder string, component appstudiov1alpha1.Component) error

type Executor interface {
	Execute(baseDir, command string, args ...string) ([]byte, error)
}

// GenerateAndPush takes in the flollowing args and generates the gitops resources for a given component
// 1. outputPath: Where to output the gitops resources to.
// 2. remote: A string of the form https://$token@github.com/<org>/<repo>. Corresponds to the component's gitops repository
// 3. component: A component struct corresponding to a single Component in an Application in AS
// 4. generator: A routine that generates gitops files
// 5. The executor to use to execute the git commands (either gitops.executor or gitops.mockExecutor)
// 6. The filesystem object used to create (either ioutils.NewFilesystem() or ioutils.NewMemoryFilesystem())
// 7. The branch to push to
// 8. The path within the repository to generate the resources in
// Adapted from https://github.com/redhat-developer/kam/blob/master/pkg/pipelines/utils.go#L79
func GenerateAndPush(outputPath string, remote string, component appstudiov1alpha1.Component, generator GitopsGeneratorFunc, e Executor, appFs afero.Fs, branch string, context string) error {
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

	// Generate the gitops resources
	componentPath := filepath.Join(repoPath, context, "components", componentName, "base")
	if err := generator(appFs, componentPath, component); err != nil {
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
		if out, err := e.Execute(repoPath, "git", "commit", "-m", "Generate GitOps resources"); err != nil {
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
