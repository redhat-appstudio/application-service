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

// From https://github.com/redhat-developer/kam/tree/master/pkg/pipelines/yaml
package yaml

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/redhat-appstudio/application-service/gitops/ioutils"
	"github.com/redhat-appstudio/application-service/gitops/resources"
	"github.com/spf13/afero"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

type Resources map[string]interface{}

func TestWriteResources(t *testing.T) {
	fs := ioutils.NewFilesystem()
	readOnlyFs := ioutils.NewReadOnlyFs()
	homeEnv := "HOME"
	originalHome := os.Getenv(homeEnv)
	defer os.Setenv(homeEnv, originalHome)
	path, cleanup := makeTempDir(t)
	defer cleanup()
	os.Setenv(homeEnv, path)
	sampleYAML := appsv1.Deployment{}
	r := Resources{
		"test/myfile.yaml": sampleYAML,
	}

	tests := []struct {
		name   string
		fs     afero.Afero
		path   string
		errMsg string
	}{
		{
			name:   "Path with ~",
			fs:     fs,
			path:   "~/manifest",
			errMsg: "",
		},
		{
			name:   "Path without ~",
			fs:     fs,
			path:   filepath.ToSlash(filepath.Join(path, "manifest", "gitops")),
			errMsg: "",
		},
		{
			name:   "Path without permission",
			fs:     fs,
			path:   "/",
			errMsg: "failed to MkDirAll for /test/myfile.yaml",
		},
		{
			name:   "Invalid path",
			fs:     readOnlyFs,
			path:   "~~~",
			errMsg: "failed to resolve path to file: cannot expand user-specific home dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := WriteResources(tt.fs, tt.path, r)
			if !errorMatch(t, tt.errMsg, err) {
				t.Fatalf("error mismatch: got %v, want %v", err, tt.errMsg)
			}
			if tt.path[0] == '~' {
				tt.path = filepath.ToSlash(filepath.Join(path, strings.Split(tt.path, "~")[1]))
			}
			if err == nil {
				assertResourceExists(t, filepath.Join(tt.path, "test", "myfile.yaml"), sampleYAML)
			}
		})
	}
}

func TestMarshalItemToFile(t *testing.T) {
	fs := ioutils.NewFilesystem()
	readOnlyFs := ioutils.NewReadOnlyFs()

	// Create a regexpfs for test cases where we need to mock file creation failures
	// If a given file name doesn't match the given regex, file creation will fail, so it makes it easy to mock file creation failures
	regexpFs := afero.Afero{Fs: afero.NewRegexpFs(afero.NewMemMapFs(), regexp.MustCompile("hello"))}

	tests := []struct {
		name   string
		fs     afero.Afero
		path   string
		item   interface{}
		errMsg string
	}{
		{
			name:   "Simple resource",
			fs:     fs,
			path:   filepath.Join(os.TempDir(), "test"),
			item:   resources.Kustomization{},
			errMsg: "",
		},
		{
			name:   "Read only filesystem error",
			fs:     readOnlyFs,
			path:   "/test/file",
			item:   resources.Kustomization{},
			errMsg: "failed to MkDirAll for /test/file: operation not permitted",
		},
		{
			name:   "Unable to create file error",
			fs:     regexpFs,
			path:   "/testtwo/file-two",
			item:   resources.Kustomization{},
			errMsg: "failed to Create file /testtwo/file-two: no such file or directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MarshalItemToFile(tt.fs, tt.path, tt.item)
			if !errorMatch(t, tt.errMsg, err) {
				t.Fatalf("error mismatch: got %v, want %v", err, tt.errMsg)
			}
			if err == nil {
				assertResourceExists(t, tt.path, tt.item)
			}
		})
	}
}

func TestMarshallOutput(t *testing.T) {
	fs := ioutils.NewMemoryFilesystem()
	readOnlyFs := afero.NewReadOnlyFs(afero.NewMemMapFs())

	f, _ := fs.Create("/test/file")
	readonlyF, _ := readOnlyFs.Open("/")
	tests := []struct {
		name   string
		f      afero.File
		item   interface{}
		errMsg string
	}{
		{
			name:   "Simple resource",
			f:      f,
			item:   resources.Kustomization{},
			errMsg: "",
		},
		{
			name:   "Invalid resource",
			f:      f,
			item:   make(chan int),
			errMsg: "failed to marshal data: error marshaling into JSON: json: unsupported type: chan int",
		},
		{
			name:   "Unable to write to resource",
			f:      readonlyF,
			item:   6.0,
			errMsg: "failed to write data: write /: file handle is read only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MarshalOutput(tt.f, tt.item)
			if !errorMatch(t, tt.errMsg, err) {
				t.Fatalf("TestMarshallOutput(): error mismatch: got %v, want %v", err, tt.errMsg)
			}
		})
	}
}

func makeTempDir(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := ioutil.TempDir(os.TempDir(), "manifest")
	assertNoError(t, err)
	return dir, func() {
		err := os.RemoveAll(dir)
		assertNoError(t, err)
	}
}

func assertResourceExists(t *testing.T, path string, resource interface{}) {
	t.Helper()
	want, err := yaml.Marshal(resource)
	assertNoError(t, err)
	got, err := ioutil.ReadFile(path)
	assertNoError(t, err)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("files not written to correct location: %s", diff)
	}
}

// ErrorMatch returns true if an error matches the required string.
//
// e.g. ErrorMatch(t, "failed to open", err) would return true if the
// err passed in had a string that matched.
//
// The message can be a regular expression, and if this fails to compile, then
// the test will fail.
func errorMatch(t *testing.T, msg string, testErr error) bool {
	t.Helper()
	if msg == "" && testErr == nil {
		return true
	}
	if msg != "" && testErr == nil {
		return false
	}
	match, err := regexp.MatchString(msg, testErr.Error())
	if err != nil {
		t.Fatal(err)
	}
	return match
}

// AssertNoError fails if there's an error
func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
