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
	"errors"
	"path/filepath"
	"testing"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops/testutils"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	"github.com/spf13/afero"
)

func TestGenerateAndPush(t *testing.T) {
	repo := "git@github.com:testing/testing.git"
	outputPath := "/fake/path"
	componentDir := "/fake/path/test-component"
	componentName := "test-component"
	component := appstudiov1alpha1.Component{
		Spec: appstudiov1alpha1.ComponentSpec{
			ContainerImage: "testimage:latest",
			Source: appstudiov1alpha1.ComponentSource{
				ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
					GitSource: &appstudiov1alpha1.GitSource{
						URL: repo,
					},
				},
			},
			TargetPort: 5000,
		},
	}
	component.Name = "test-component"
	fs := ioutils.NewMemoryFilesystem()
	readOnlyFs := ioutils.NewReadOnlyFs()

	tests := []struct {
		name          string
		fs            afero.Afero
		component     appstudiov1alpha1.Component
		errors        *testutils.ErrorStack
		outputs       [][]byte
		want          []testutils.Execution
		wantErrString string
	}{
		{
			name:      "No errors",
			fs:        fs,
			component: component,
			errors:    &testutils.ErrorStack{},
			want: []testutils.Execution{
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"clone", repo, component.Name},
				},
				{
					BaseDir: componentDir,
					Command: "git",
					Args:    []string{"switch", "main"},
				},
				{
					BaseDir: componentDir,
					Command: "rm",
					Args:    []string{"-rf", filepath.Join("components", componentName)},
				},
				{
					BaseDir: componentDir,
					Command: "git",
					Args:    []string{"add", "."},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"--no-pager", "diff", "--cached"},
				},
				{
					BaseDir: componentDir,
					Command: "git",
					Args:    []string{"commit", "-m", "Generate GitOps resources"},
				},
				{
					BaseDir: componentDir,
					Command: "git",
					Args:    []string{"push", "origin", "main"},
				},
			},
		},
		{
			name:      "Git clone failure",
			fs:        fs,
			component: component,
			errors: &testutils.ErrorStack{
				Errors: []error{
					nil,
					errors.New("test error"),
				},
			},
			outputs: [][]byte{
				[]byte("test output1"),
				[]byte("test output2"),
			},
			want: []testutils.Execution{
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"clone", repo, component.Name},
				},
			},
			wantErrString: "test error",
		},
		{
			name:      "Git switch failure, git checkout failure",
			fs:        fs,
			component: component,
			errors: &testutils.ErrorStack{
				Errors: []error{
					errors.New("Permission denied"),
					errors.New("Fatal error"),
					nil,
				},
			},
			outputs: [][]byte{
				[]byte("test output1"),
				[]byte("test output2"),
				[]byte("test output3"),
			},
			want: []testutils.Execution{
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"clone", repo, component.Name},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"switch", "main"},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"checkout", "-b", "main"},
				},
			},
			wantErrString: "failed to checkout branch \"main\" in \"/fake/path/test-component\" \"test output1\": Permission denied",
		},
		{
			name:      "Git switch failure, git checkout success",
			fs:        fs,
			component: component,
			errors: &testutils.ErrorStack{
				Errors: []error{
					nil,
					errors.New("test error"),
					nil,
				},
			},
			outputs: [][]byte{
				[]byte("test output1"),
				[]byte("test output2"),
				[]byte("test output3"),
			},
			want: []testutils.Execution{
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"clone", repo, component.Name},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"switch", "main"},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"checkout", "-b", "main"},
				},
			},
			wantErrString: "",
		},
		{
			name:      "rm -rf failure",
			fs:        fs,
			component: component,
			errors: &testutils.ErrorStack{
				Errors: []error{
					errors.New("Permission Denied"),
					nil,
					nil,
				},
			},
			outputs: [][]byte{
				[]byte("test output1"),
				[]byte("test output2"),
				[]byte("test output3"),
			},
			want: []testutils.Execution{
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"clone", repo, component.Name},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"switch", "main"},
				},
				{
					BaseDir: outputPath,
					Command: "rm",
					Args:    []string{"-rf", "components/test-component"},
				},
			},
			wantErrString: "failed to delete \"components/test-component\" folder in repository in \"/fake/path/test-component\" \"test output1\": Permission Denied",
		},
		{
			name:      "git add failure",
			fs:        fs,
			component: component,
			errors: &testutils.ErrorStack{
				Errors: []error{
					errors.New("Fatal error"),
					nil,
					nil,
					nil,
				},
			},
			outputs: [][]byte{
				[]byte("test output1"),
				[]byte("test output2"),
				[]byte("test output3"),
				[]byte("test output4"),
			},
			want: []testutils.Execution{
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"clone", repo, component.Name},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"switch", "main"},
				},
				{
					BaseDir: outputPath,
					Command: "rm",
					Args:    []string{"-rf", "components/test-component"},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"add", "."},
				},
			},
			wantErrString: "failed to add files for component \"test-component\" to repository in \"/fake/path/test-component\" \"test output1\": Fatal error",
		},
		{
			name:      "git diff failure",
			fs:        fs,
			component: component,
			errors: &testutils.ErrorStack{
				Errors: []error{
					errors.New("Permission Denied"),
					nil,
					nil,
					nil,
					nil,
				},
			},
			outputs: [][]byte{
				[]byte("test output1"),
				[]byte("test output2"),
				[]byte("test output3"),
				[]byte("test output4"),
				[]byte("test output5"),
			},
			want: []testutils.Execution{
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"clone", repo, component.Name},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"switch", "main"},
				},
				{
					BaseDir: outputPath,
					Command: "rm",
					Args:    []string{"-rf", "components/test-component"},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"--no-pager", "diff"},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"add", "."},
				},
			},
			wantErrString: "failed to check git diff in repository \"/fake/path/test-component\" \"test output1\": Permission Denied",
		},
		{
			name:      "git commit failure",
			fs:        fs,
			component: component,
			errors: &testutils.ErrorStack{
				Errors: []error{
					errors.New("Fatal error"),
					nil,
					nil,
					nil,
					nil,
					nil,
				},
			},
			outputs: [][]byte{
				[]byte("test output1"),
				[]byte("test output2"),
				[]byte("test output3"),
				[]byte("test output4"),
				[]byte("test output5"),
				[]byte("test output6"),
			},
			want: []testutils.Execution{
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"clone", repo, component.Name},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"switch", "main"},
				},
				{
					BaseDir: outputPath,
					Command: "rm",
					Args:    []string{"-rf", "components/test-component"},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"--no-pager", "diff"},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"add", "."},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"commit", "-m", "Generate GitOps resources"},
				},
			},
			wantErrString: "failed to commit files to repository in \"/fake/path/test-component\" \"test output1\": Fatal error",
		},
		{
			name:      "git push failure",
			fs:        fs,
			component: component,
			errors: &testutils.ErrorStack{
				Errors: []error{
					errors.New("Fatal error"),
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
				},
			},
			outputs: [][]byte{
				[]byte("test output1"),
				[]byte("test output2"),
				[]byte("test output3"),
				[]byte("test output4"),
				[]byte("test output5"),
				[]byte("test output6"),
			},
			want: []testutils.Execution{
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"clone", repo, component.Name},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"switch", "main"},
				},
				{
					BaseDir: outputPath,
					Command: "rm",
					Args:    []string{"-rf", "components/test-component"},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"--no-pager", "diff"},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"add", "."},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"commit", "-m", "Generate GitOps resources"},
				},
			},
			wantErrString: "failed push remote to repository \"git@github.com:testing/testing.git\" \"\": Fatal error",
		},
		{
			name:      "gitops generate failure",
			fs:        readOnlyFs,
			component: component,
			errors: &testutils.ErrorStack{
				Errors: []error{
					errors.New("Fatal error"),
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
				},
			},
			outputs: [][]byte{
				[]byte("test output1"),
				[]byte("test output2"),
				[]byte("test output3"),
				[]byte("test output4"),
				[]byte("test output5"),
				[]byte("test output6"),
			},
			want: []testutils.Execution{
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"clone", repo, component.Name},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"switch", "main"},
				},
				{
					BaseDir: outputPath,
					Command: "rm",
					Args:    []string{"-rf", "components/test-component"},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"--no-pager", "diff"},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"add", "."},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"commit", "-m", "Generate GitOps resources"},
				},
			},
			wantErrString: "failed to generate the gitops resources in \"/fake/path/test-component/components/test-component/base\" for component \"test-component\"",
		},
		{
			name: "gitops generate failure - image component",
			fs:   readOnlyFs,
			component: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ContainerImage: "quay.io/test/test",
					TargetPort:     5000,
				},
			},
			errors: &testutils.ErrorStack{
				Errors: []error{
					errors.New("Fatal error"),
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
				},
			},
			outputs: [][]byte{
				[]byte("test output1"),
				[]byte("test output2"),
				[]byte("test output3"),
				[]byte("test output4"),
				[]byte("test output5"),
				[]byte("test output6"),
			},
			want: []testutils.Execution{
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"clone", repo, component.Name},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"switch", "main"},
				},
				{
					BaseDir: outputPath,
					Command: "rm",
					Args:    []string{"-rf", "components/test-component"},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"--no-pager", "diff"},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"add", "."},
				},
				{
					BaseDir: outputPath,
					Command: "git",
					Args:    []string{"commit", "-m", "Generate GitOps resources"},
				},
			},
			wantErrString: "failed to generate the gitops resources in \"/fake/path/components/base\" for component \"\": failed to MkDirAll",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := testutils.NewMockExecutor(tt.outputs...)
			e.Errors = tt.errors
			err := GenerateAndPush(outputPath, repo, tt.component, e, tt.fs, "main", "/")

			if tt.wantErrString != "" {
				testutils.AssertErrorMatch(t, tt.wantErrString, err)
			} else {
				testutils.AssertNoError(t, err)
			}

		})
	}
}

func TestExecute(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		outputPath string
		args       string
		wantErr    bool
	}{
		{
			name:    "Simple command to execute",
			command: "git",
			args:    "help",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new executor
			e := NewCmdExecutor()
			_, err := e.Execute(tt.outputPath, tt.command, tt.args)
			if !tt.wantErr && (err != nil) {
				t.Errorf("TestExecute() unexpected error value: %v", err)
			}
		})
	}
}
