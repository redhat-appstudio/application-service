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

package yaml

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	res "github.com/redhat-appstudio/application-service/gitops/resources"
	"github.com/spf13/afero"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

func TestWriteResources(t *testing.T) {
	fs := afero.NewOsFs()
	homeEnv := "HOME"
	originalHome := os.Getenv(homeEnv)
	defer os.Setenv(homeEnv, originalHome)
	path, cleanup := makeTempDir(t)
	defer cleanup()
	os.Setenv(homeEnv, path)
	sampleYAML := appsv1.Deployment{}
	r := res.Resources{
		"test/myfile.yaml": sampleYAML,
	}

	tests := []struct {
		name   string
		path   string
		errMsg string
	}{
		{"Path with ~", "~/manifest", ""},
		{"Path without ~", filepath.ToSlash(filepath.Join(path, "manifest", "gitops")), ""},
		{"Path without permission", "/", "failed to MkDirAll for /test/myfile.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := WriteResources(fs, tt.path, r)
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

func assertErrorMatch(t *testing.T, msg string, testErr error) {
	t.Helper()
	if !errorMatch(t, msg, testErr) {
		t.Fatalf("failed to match error: '%s' did not match %v", testErr, msg)
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
