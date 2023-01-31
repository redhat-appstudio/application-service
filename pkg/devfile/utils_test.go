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

package devfile

import (
	"testing"
)

func TestUpdateGitLink(t *testing.T) {

	tests := []struct {
		name     string
		repo     string
		context  string
		wantLink string
		wantErr  bool
	}{
		{
			name:     "context has no http",
			repo:     "https://github.com/maysunfaisal/multi-components-dockerfile/",
			context:  "devfile-sample-java-springboot-basic/docker/Dockerfile",
			wantLink: "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-java-springboot-basic/docker/Dockerfile",
		},
		{
			name:     "context has http",
			repo:     "https://github.com/maysunfaisal/multi-components-dockerfile/",
			context:  "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-java-springboot-basic/docker/Dockerfile",
			wantLink: "https://raw.githubusercontent.com/maysunfaisal/multi-components-dockerfile/main/devfile-sample-java-springboot-basic/docker/Dockerfile",
		},
		{
			name:    "err case",
			repo:    "\000x",
			context: "test/dir",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			gotLink, err := UpdateGitLink(tt.repo, "", tt.context)
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected err: %+v", err)
			} else if tt.wantErr && err == nil {
				t.Errorf("Expected error but got nil")
			} else if gotLink != tt.wantLink {
				t.Errorf("Expected: %+v, Got: %+v", tt.wantLink, gotLink)
			}

		})
	}
}
