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
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/redhat-developer/gitops-generator/pkg/util"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/factory"

	gitopsv1alpha1 "github.com/redhat-developer/gitops-generator/api/v1alpha1"
	"github.com/spf13/afero"
)

const defaultRepoDescription = "Bootstrapped GitOps Repository based on Components"

type CommandType string

const (
	GitCommand        CommandType = "git"
	RmCommand         CommandType = "rm"
	unsupportedCmdMsg             = "Unsupported command \"%s\" "
)

type Generator interface {
	CloneGenerateAndPush(outputPath string, remote string, options gitopsv1alpha1.GeneratorOptions, appFs afero.Afero, branch string, context string, doPush bool) error
	CommitAndPush(outputPath string, repoPathOverride string, remote string, componentName string, branch string, commitMessage string) error
	GenerateAndPush(outputPath string, remote string, options gitopsv1alpha1.GeneratorOptions, appFs afero.Afero, branch string, doPush bool, createdBy string) error
	GenerateOverlaysAndPush(outputPath string, clone bool, remote string, options gitopsv1alpha1.GeneratorOptions, applicationName, environmentName, imageName, namespace string, appFs afero.Afero, branch string, context string, doPush bool, componentGeneratedResources map[string][]string) error
	GitRemoveComponent(outputPath string, remote string, componentName string, branch string, context string) error
	CloneRepo(outputPath string, remote string, componentName string, branch string) error
	RemoveComponent(outputPath string, componentName string, context string) error
	GetCommitIDFromRepo(fs afero.Afero, repoPath string) (string, error)
}

// NewGitopsGen returns a Generator implementation
func NewGitopsGen() Gen {
	return Gen{}
}

type Gen struct {
}

// expose as a global variable for the purpose of running mock tests
// only "git" and "rm" are supported
var execute = func(baseDir string, cmd CommandType, args ...string) ([]byte, error) {
	if cmd == GitCommand || cmd == RmCommand {
		c := exec.Command(string(cmd), args...)
		c.Dir = baseDir
		output, err := c.CombinedOutput()
		return output, err
	}

	return []byte(""), fmt.Errorf(unsupportedCmdMsg, string(cmd))
}

// CloneGenerateAndPush takes in the following args and generates the gitops resources for a given component
// 1. outputPath: Where to output the gitops resources to
// 2. remote: A string of the form https://$token@<domain>/<org>/<repo>, where <domain> is either github.com or gitlab.com and $token is optional. Corresponds to the component's gitops repository
// 3. options: Options for resource generation
// 4. The executor to use to execute the git commands (either gitops.executor or gitops.mockExecutor)
// 5. The filesystem object used to create (either ioutils.NewFilesystem() or ioutils.NewMemoryFilesystem())
// 6. The branch to push to
// 7. The path within the repository to generate the resources in
// 8. The gitops config containing the build bundle;
// Adapted from https://github.com/redhat-developer/kam/blob/master/pkg/pipelines/utils.go#L79
func (s Gen) CloneGenerateAndPush(outputPath string, remote string, options gitopsv1alpha1.GeneratorOptions, appFs afero.Afero, branch string, context string, doPush bool) error {
	componentName := options.Name

	invalidRemoteErr := util.ValidateRemote(remote)
	if invalidRemoteErr != nil {
		return invalidRemoteErr
	}

	if out, err := execute(outputPath, GitCommand, "clone", remote, componentName); err != nil {
		return fmt.Errorf("failed to clone git repository in %q %q: %s", outputPath, string(out), err)
	}

	repoPath := filepath.Join(outputPath, componentName)
	gitopsFolder := filepath.Join(repoPath, context)
	componentPath := filepath.Join(gitopsFolder, "components", componentName, "base")

	// Checkout the specified branch
	if _, err := execute(repoPath, GitCommand, "switch", branch); err != nil {
		if out, err := execute(repoPath, GitCommand, "checkout", "-b", branch); err != nil {
			return fmt.Errorf("failed to checkout branch %q in %q %q: %s", branch, repoPath, string(out), err)
		}
	}

	if out, err := execute(repoPath, RmCommand, "-rf", filepath.Join("components", componentName, "base")); err != nil {
		return fmt.Errorf("failed to delete %q folder in repository in %q %q: %s", filepath.Join("components", componentName, "base"), repoPath, string(out), err)
	}

	// Generate the gitops resources and update the parent kustomize yaml file
	if err := Generate(appFs, gitopsFolder, componentPath, options); err != nil {
		return fmt.Errorf("failed to generate the gitops resources in %q for component %q: %s", componentPath, componentName, err)
	}

	if doPush {
		return s.CommitAndPush(outputPath, "", remote, componentName, branch, fmt.Sprintf("Generate GitOps base resources for component %s", componentName))
	}
	return nil
}

// CommitAndPush pushes any new changes to the GitOps repo.  The folder should already be cloned in the target output folder.
// 1. outputPath: Where the gitops resources are
// 2. repoPathOverride: The default path is the componentName. Use this to override the default folder.
// 3. remote: A string of the form https://$token@github.com/<org>/<repo>. Corresponds to the component's gitops repository
// 4. componentName: The component name corresponding to a single Component in an Application in AS. eg. component.Name
// 5. The executor to use to execute the git commands (either gitops.executor or gitops.mockExecutor)
// 6. The branch to push to
// 7. The path within the repository to generate the resources in
func (s Gen) CommitAndPush(outputPath string, repoPathOverride string, remote string, componentName string, branch string, commitMessage string) error {

	invalidRemoteErr := util.ValidateRemote(remote)
	if invalidRemoteErr != nil {
		return invalidRemoteErr
	}

	repoPath := filepath.Join(outputPath, componentName)
	if repoPathOverride != "" {
		repoPath = filepath.Join(outputPath, repoPathOverride)
	}

	if out, err := execute(repoPath, GitCommand, "add", "."); err != nil {
		return fmt.Errorf("failed to add files for component %q to repository in %q %q: %s", componentName, repoPath, string(out), err)
	}

	if out, err := execute(repoPath, GitCommand, "--no-pager", "diff", "--cached"); err != nil {
		return fmt.Errorf("failed to check git diff in repository %q %q: %s", repoPath, string(out), err)
	} else if string(out) != "" {
		// Commit the changes and push
		if out, err := execute(repoPath, GitCommand, "commit", "-m", commitMessage); err != nil {
			return fmt.Errorf("failed to commit files to repository in %q %q: %s", repoPath, string(out), err)
		}
		if out, err := execute(repoPath, GitCommand, "push", "origin", branch); err != nil {
			return fmt.Errorf("failed push remote to repository %q %q: %s", remote, string(out), err)
		}
	}

	return nil
}

// GenerateAndPush generates a new gitops folder with one component, and optionally pushes to Git. Note: this does not
// clone an existing gitops repo.
// 1. outputPath: Where the gitops resources are
// 2. remote: A string of the form https://$token@github.com/<org>/<repo>. Corresponds to the component's gitops repository
// 3. options: Options for resource generation
// 4. The executor to use to execute the git commands (either gitops.executor or gitops.mockExecutor)
// 5. The filesystem object used to create (either ioutils.NewFilesystem() or ioutils.NewMemoryFilesystem())
// 6. The branch to push to
// 7. Optionally push to the GitOps repository or not.  Default is not to push.
// 8. createdBy: Use a unique name to identify that clients are generating the GitOps repository. Default is "application-service" and should be overwritten.
func (s Gen) GenerateAndPush(outputPath string, remote string, options gitopsv1alpha1.GeneratorOptions, appFs afero.Afero, branch string, doPush bool, createdBy string) error {
	CreatedBy = createdBy

	componentName := options.Name
	repoPath := filepath.Join(outputPath, options.Application)

	// Generate the gitops resources and update the parent kustomize yaml file
	gitopsFolder := repoPath

	gitHostAccessToken := options.Secret
	componentPath := filepath.Join(gitopsFolder, "components", componentName, "base")
	if err := Generate(appFs, gitopsFolder, componentPath, options); err != nil {
		return fmt.Errorf("failed to generate the gitops resources in %q for component %q: %s", componentPath, componentName, err)
	}

	// Commit the changes and push
	if doPush {
		gitOpsRepoURL := ""
		if options.GitSource != nil {
			gitOpsRepoURL = options.GitSource.URL
		}
		if gitOpsRepoURL == "" {
			return fmt.Errorf("the GitOps repo URL is not set")
		}
		u, err := url.Parse(gitOpsRepoURL)
		if err != nil {
			return fmt.Errorf("failed to parse GitOps repo URL %q: %w", gitOpsRepoURL, err)
		}
		parts := strings.Split(u.Path, "/")
		org := parts[1]
		repoName := strings.TrimSuffix(strings.Join(parts[2:], "/"), ".git")
		u.User = url.UserPassword("", gitHostAccessToken)

		client, err := factory.FromRepoURL(u.String())
		if err != nil {
			return fmt.Errorf("failed to create a client to access %q: %w", gitOpsRepoURL, err)
		}
		ctx := context.Background()
		// If we're creating the repository in a personal user's account, it's a
		// different API call that's made, clearing the org triggers go-scm to use
		// the "create repo in personal account" endpoint.
		currentUser, _, err := client.Users.Find(ctx)
		if err != nil {
			return fmt.Errorf("failed to get the user with their auth token: %w", err)
		}
		if currentUser.Login == org {
			org = ""
		}

		ri := &scm.RepositoryInput{
			Private:     true,
			Description: defaultRepoDescription,
			Namespace:   org,
			Name:        repoName,
		}
		_, _, err = client.Repositories.Create(context.Background(), ri)
		if err != nil {
			repo := fmt.Sprintf("%s/%s", org, repoName)
			if org == "" {
				repo = fmt.Sprintf("%s/%s", currentUser.Login, repoName)
			}
			if _, resp, err := client.Repositories.Find(context.Background(), repo); err == nil && resp.Status == 200 {
				return fmt.Errorf("failed to create repository, repo already exists")
			}
			return fmt.Errorf("failed to create repository %q in namespace %q: %w", repoName, org, err)
		}

		if out, err := execute(repoPath, GitCommand, "init", "."); err != nil {
			return fmt.Errorf("failed to initialize git repository in %q %q: %s", repoPath, string(out), err)
		}
		if out, err := execute(repoPath, GitCommand, "add", "."); err != nil {
			return fmt.Errorf("failed to add components to repository in %q %q: %s", repoPath, string(out), err)
		}
		if out, err := execute(repoPath, GitCommand, "commit", "-m", "Generate GitOps resources"); err != nil {
			return fmt.Errorf("failed to commit files to repository in %q %q: %s", repoPath, string(out), err)
		}
		if out, err := execute(repoPath, GitCommand, "branch", "-m", branch); err != nil {
			return fmt.Errorf("failed to switch to branch %q in repository in %q %q: %s", branch, repoPath, string(out), err)
		}
		if out, err := execute(repoPath, GitCommand, "remote", "add", "origin", remote); err != nil {
			return fmt.Errorf("failed to add files for component %q, to remote 'origin' %q to repository in %q %q: %s", componentName, remote, repoPath, string(out), err)
		}
		if out, err := execute(repoPath, GitCommand, "push", "-u", "origin", branch); err != nil {
			return fmt.Errorf("failed push remote to repository %q %q: %s", remote, string(out), err)
		}
	}

	return nil
}

// GenerateOverlaysAndPush generates the overlays kustomize from App Env Snapshot Binding Spec
// 1. outputPath: Where to output the gitops resources to
// 2. clone: Optionally clone the repository first
// 3. remote: A string of the form https://$token@github.com/<org>/<repo>. Corresponds to the component's gitops repository
// 4. options: Options for resource generation
// 5. applicationName: The name of the application
// 6. environmentName: The name of the environment
// 7. imageName: The image name of the source
// 8  namespace: The namespace of the component. This is used in as the namespace of the deployment yaml.
// 9. The executor to use to execute the git commands (either gitops.executor or gitops.mockExecutor)
// 10. The filesystem object used to create (either ioutils.NewFilesystem() or ioutils.NewMemoryFilesystem())
// 11. The branch to push to
// 12. The path within the repository to generate the resources in
// 13. Push the changes to the repository or not.
// 14. The gitops config containing the build bundle;
func (s Gen) GenerateOverlaysAndPush(outputPath string, clone bool, remote string, options gitopsv1alpha1.GeneratorOptions, applicationName, environmentName, imageName, namespace string, appFs afero.Afero, branch string, context string, doPush bool, componentGeneratedResources map[string][]string) error {

	if clone || doPush {
		invalidRemoteErr := util.ValidateRemote(remote)
		if invalidRemoteErr != nil {
			return invalidRemoteErr
		}
	}

	componentName := options.Name
	repoPath := filepath.Join(outputPath, applicationName)

	if clone {
		if out, err := execute(outputPath, GitCommand, "clone", remote, applicationName); err != nil {
			return fmt.Errorf("failed to clone git repository in %q %q: %s", outputPath, string(out), err)
		}

		// Checkout the specified branch
		if _, err := execute(repoPath, GitCommand, "switch", branch); err != nil {
			if out, err := execute(repoPath, GitCommand, "checkout", "-b", branch); err != nil {
				return fmt.Errorf("failed to checkout branch %q in %q %q: %s", branch, repoPath, string(out), err)
			}
		}
	}

	// Generate the gitops resources and update the parent kustomize yaml file
	gitopsFolder := filepath.Join(repoPath, context)
	componentEnvOverlaysPath := filepath.Join(gitopsFolder, "components", componentName, "overlays", environmentName)
	if err := GenerateOverlays(appFs, gitopsFolder, componentEnvOverlaysPath, options, imageName, namespace, componentGeneratedResources); err != nil {
		return fmt.Errorf("failed to generate the gitops resources in overlays dir %q for component %q: %s", componentEnvOverlaysPath, componentName, err)
	}

	if doPush {
		return s.CommitAndPush(outputPath, applicationName, remote, componentName, branch, fmt.Sprintf("Generate %s environment overlays for component %s", environmentName, componentName))
	}
	return nil
}

// GitRemoveComponent clones the repo, removes the component, and pushes the changes back to the repository. It takes in the following args and updates the gitops resources by removing the given component
// 1. outputPath: Where to output the gitops resources to
// 2. remote: A string of the form https://$token@<domain>/<org>/<repo>, where <domain> is either github.com or gitlab.com and $token is optional. Corresponds to the component's gitops repository
// 3. componentName: The component name corresponding to a single Component in an Application. eg. component.Name
// 4. The executor to use to execute the git commands (either gitops.executor or gitops.mockExecutor)
// 5. The branch to push to
// 6. The path within the repository to generate the resources in
func (s Gen) GitRemoveComponent(outputPath string, remote string, componentName string, branch string, context string) error {
	if cloneError := s.CloneRepo(outputPath, remote, componentName, branch); cloneError != nil {
		return cloneError
	}
	if removeComponentError := s.RemoveComponent(outputPath, componentName, context); removeComponentError != nil {
		return removeComponentError
	}

	return s.CommitAndPush(outputPath, "", remote, componentName, branch, fmt.Sprintf("Removed component %s", componentName))
}

// CloneRepo clones the repo, and switches to the branch
// 1. outputPath: Where to output the gitops resources to
// 2. remote: A string of the form https://$token@<domain>/<org>/<repo>, where <domain> is either github.com or gitlab.com and $token is optional. Corresponds to the component's gitops repository
// 3. componentName: The component name corresponding to a single Component in an Application. eg. component.Name
// 4. The executor to use to execute the git commands (either gitops.executor or gitops.mockExecutor)
// 5. The branch to push to switch to
func (s Gen) CloneRepo(outputPath string, remote string, componentName string, branch string) error {
	invalidRemoteErr := util.ValidateRemote(remote)
	if invalidRemoteErr != nil {
		return invalidRemoteErr
	}

	repoPath := filepath.Join(outputPath, componentName)

	if out, err := execute(outputPath, GitCommand, "clone", remote, componentName); err != nil {
		return fmt.Errorf("failed to clone git repository in %q %q: %s", outputPath, string(out), err)
	}
	// Checkout the specified branch
	if _, err := execute(repoPath, GitCommand, "switch", branch); err != nil {
		if out, err := execute(repoPath, GitCommand, "checkout", "-b", branch); err != nil {
			return fmt.Errorf("failed to checkout branch %q in %q %q: %s", branch, repoPath, string(out), err)
		}
	}
	return nil
}

// RemoveComponent removes the component from the local folder.  This expects the git repo to be already cloned
// 1. outputPath: Where the gitops repo contents have been cloned
// 2. componentName: The component name corresponding to a single Component in an Application. eg. component.Name
// 3. The executor to use to execute the git commands (either gitops.executor or gitops.mockExecutor)
// 4. The path within the repository to generate the resources in
func (s Gen) RemoveComponent(outputPath string, componentName string, context string) error {
	repoPath := filepath.Join(outputPath, componentName)
	gitopsFolder := filepath.Join(repoPath, context)
	componentPath := filepath.Join(gitopsFolder, "components", componentName)
	if out, err := execute(repoPath, RmCommand, "-rf", componentPath); err != nil {
		return fmt.Errorf("failed to delete %q folder in repository in %q %q: %s", componentPath, repoPath, string(out), err)
	}
	return nil
}

// GetCommitIDFromRepo returns the commit ID for the given repository
func (s Gen) GetCommitIDFromRepo(fs afero.Afero, repoPath string) (string, error) {
	var out []byte
	var err error
	if out, err = execute(repoPath, GitCommand, "rev-parse", "HEAD"); err != nil {
		return "", fmt.Errorf("failed to retrieve commit id for repository in %q %q: %s", repoPath, string(out), err)
	}
	return string(out), nil
}
