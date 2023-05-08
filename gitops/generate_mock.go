//
// Copyright 2022-2023 Red Hat, Inc.
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
	"strings"

	gitopsv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
	gitops "github.com/redhat-developer/gitops-generator/pkg"
	"github.com/redhat-developer/gitops-generator/pkg/testutils"
	"github.com/spf13/afero"
)

type MockGenerator struct {
	Outputs  *testutils.OutputStack
	Errors   *testutils.ErrorStack
	Executed []testutils.Execution
}

func NewMockGenerator(outputs ...[]byte) *MockGenerator {
	return &MockGenerator{
		Outputs:  testutils.NewOutputs(outputs...),
		Errors:   testutils.NewErrors(),
		Executed: []testutils.Execution{},
	}
}

var outputStack = testutils.NewOutputs()
var errorStack = testutils.NewErrors()

// CloneGenerateAndPush succeeds if there's no errors in the errorStack, fails otherwise
func (m *MockGenerator) CloneGenerateAndPush(outputPath string, remote string, options gitopsv1alpha1.GeneratorOptions, appFs afero.Afero, branch string, context string, doPush bool) error {
	if len(m.Errors.Errors) > 0 {
		return fmt.Errorf("failed to clone git repository")
	}
	return nil
}

// GenerateOverlaysAndPush is a simplified version of the real method.  It's intended to invoke a pass/fail response from GenerateOverlays and nothing else
func (m *MockGenerator) GenerateOverlaysAndPush(outputPath string, clone bool, remote string, options gitopsv1alpha1.GeneratorOptions, applicationName, environmentName, imageName, namespace string, appFs afero.Afero, branch string, context string, doPush bool, componentGeneratedResources map[string][]string) error {
	componentName := options.Name
	repoPath := filepath.Join(outputPath, applicationName)

	// Generate the gitops resources and update the parent kustomize yaml file
	gitopsFolder := filepath.Join(repoPath, context)
	componentEnvOverlaysPath := filepath.Join(gitopsFolder, "components", componentName, "overlays", environmentName)
	if err := gitops.GenerateOverlays(appFs, gitopsFolder, componentEnvOverlaysPath, options, imageName, namespace, componentGeneratedResources); err != nil {
		return fmt.Errorf("failed to generate the gitops resources in overlays dir %q for component %q: %s", componentEnvOverlaysPath, componentName, err)
	}

	return nil
}

func (m *MockGenerator) GetCommitIDFromRepo(fs afero.Afero, repoPath string) (string, error) {
	var out []byte
	var err error
	if out, err = execute(repoPath, gitops.GitCommand, "rev-parse", "HEAD"); err != nil {
		return "", fmt.Errorf("failed to retrieve commit id for repository in %q %q: %s", repoPath, string(out), err)
	}
	return string(out), nil

}

// GitRemoveComponent is not called by any tests
func (m *MockGenerator) GitRemoveComponent(outputPath string, remote string, componentName string, branch string, context string) error {
	return nil
}

// CloneRepo is not called by any tests
func (m *MockGenerator) CloneRepo(outputPath string, remote string, componentName string, branch string) error {
	return nil
}

// RemoveComponent is not called by any tests
func (m *MockGenerator) RemoveComponent(outputPath string, componentName string, context string) error {
	return nil
}

// CommitAndPush always succeeds
func (m *MockGenerator) CommitAndPush(outputPath string, repoPathOverride string, remote string, componentName string, branch string, commitMessage string) error {
	return nil
}

// GenerateAndPush is not called by any tests
func (m *MockGenerator) GenerateAndPush(outputPath string, remote string, options gitopsv1alpha1.GeneratorOptions, appFs afero.Afero, branch string, doPush bool, createdBy string) error {
	return nil
}

func execute(baseDir string, cmd gitops.CommandType, args ...string) ([]byte, error) {
	if cmd == gitops.GitCommand || cmd == gitops.RmCommand {
		if len(args) > 0 && args[0] == "rev-parse" {
			if strings.Contains(baseDir, "test-git-error") {
				return []byte(""), fmt.Errorf("unable to retrieve git commit id")
			} else {
				return []byte("ca82a6dff817ec66f44342007202690a93763949"), errorStack.Pop()
			}
		} else {
			return outputStack.Pop(), errorStack.Pop()
		}
	}

	return []byte(""), fmt.Errorf("unsupported command \"%s\" ", string(cmd))
}
