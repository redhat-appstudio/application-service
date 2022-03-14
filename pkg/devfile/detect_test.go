//
// Copyright 2022 Red Hat, Inc.
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

package devfile

import (
	"os"
	"reflect"
	"testing"

	"github.com/redhat-appstudio/application-service/pkg/util"
)

func TestAnalyzeAndDetectDevfile(t *testing.T) {

	tests := []struct {
		name                string
		clonePath           string
		repo                string
		token               string
		wantDevfile         bool
		wantDevfileEndpoint string
		wantErr             bool
	}{
		{
			name:                "Successfully detect a devfile from the registry",
			clonePath:           "/tmp/testclone",
			repo:                "https://github.com/maysunfaisal/devfile-sample-java-springboot-basic-1",
			wantDevfile:         true,
			wantDevfileEndpoint: "https://registry.stage.devfile.io/devfiles/java-springboot-basic",
		},
		{
			name:      "Cannot detect a devfile for a Go repository",
			clonePath: "/tmp/testclone",
			repo:      "https://github.com/devfile/devworkspace-operator",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := util.CloneRepo(tt.clonePath, tt.repo, tt.token)
			if err != nil {
				t.Errorf("got unexpected error %v", err)
			} else {
				path := tt.clonePath
				if tt.name == "Invalid Path" {
					path = ""
				}
				devfileBytes, detectedDevfileEndpoint, err := AnalyzeAndDetectDevfile(path)
				if !tt.wantErr && err != nil {
					t.Errorf("Unexpected err: %+v", err)
				} else if tt.wantErr && err == nil {
					t.Errorf("Expected error but got nil")
				} else if !reflect.DeepEqual(len(devfileBytes) > 0, tt.wantDevfile) {
					t.Errorf("Expected devfile: %+v, \nGot: %+v", tt.wantDevfile, len(devfileBytes) > 0)
				} else if !reflect.DeepEqual(detectedDevfileEndpoint, tt.wantDevfileEndpoint) {
					t.Errorf("Expected devfile endpoint: %+v, \nGot: %+v", tt.wantDevfileEndpoint, detectedDevfileEndpoint)
				}
			}
			os.RemoveAll(tt.clonePath)
		})
	}
}
