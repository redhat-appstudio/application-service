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

package spi

import (
	"context"
	"testing"
)

// TestDownloadDevfileFromSPI uses the Mock SPI client to test the DownloadDevfileFromSPI function
// Since SPI does not support running outside of Kube, we cannot unit test the non-mock SPI client at this moment
func TestDownloadDevfileFromSPI(t *testing.T) {
	var mock MockSPIClient

	tests := []struct {
		name    string
		repoUrl string
		path    string
		want    string
		wantErr bool
	}{
		{
			name:    "Successfully retrieve devfile, no context/path set",
			repoUrl: "https://github.com/testrepo/test-private-repo",
			want:    mockDevfile,
		},
		{
			name:    "Successfully retrieve devfile, context/path set",
			repoUrl: "https://github.com/testrepo/test-private-repo",
			path:    "/test",
			want:    mockDevfile,
		},
		{
			name:    "Unable to retrieve devfile",
			repoUrl: "https://github.com/testrepo/test-error-response",
			wantErr: true,
		},
		{
			name:    "Error reading devfile",
			repoUrl: "https://github.com/testrepo/test-parse-error",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert the hasApp resource to a devfile
			devfileBytes, err := DownloadDevfileUsingSPI(mock, context.Background(), "test-namespace", tt.repoUrl, "main", tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error return value: %v", err)
			}

			devfileBytesString := string(devfileBytes)
			if devfileBytesString != tt.want {
				t.Errorf("error: expected %v, got %v", tt.want, devfileBytesString)
			}
		})
	}
}

func TestDownloadDevfileandDockerfileUsingSPI(t *testing.T) {
	var mock MockSPIClient

	tests := []struct {
		name           string
		repoUrl        string
		path           string
		wantDevfile    string
		wantDockerfile string
		wantErr        bool
	}{
		{
			name:           "Successfully retrieve devfile, no context/path set",
			repoUrl:        "https://github.com/testrepo/test-private-repo",
			wantDevfile:    mockDevfile,
			wantDockerfile: mockDockerfile,
		},
		{
			name:           "Successfully retrieve devfile, context/path set",
			repoUrl:        "https://github.com/testrepo/test-private-repo",
			path:           "/test",
			wantDevfile:    mockDevfile,
			wantDockerfile: mockDockerfile,
		},
		{
			name:    "Error reading devfile",
			repoUrl: "https://github.com/testrepo/test-parse-error",
			wantErr: true,
		},
		{
			name:    "Error reading Dockerfile",
			repoUrl: "https://github.com/testrepo/test-error-dockerfile-response",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devfileBytes, dockerfileBytes, err := DownloadDevfileandDockerfileUsingSPI(mock, context.Background(), "test-namespace", tt.repoUrl, "main", tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error return value: %v", err)
				return
			}

			devfileBytesString := string(devfileBytes)
			if devfileBytesString != tt.wantDevfile {
				t.Errorf("devfile error: expected %v, got %v", tt.wantDevfile, devfileBytesString)
			}

			dockerfileBytesString := string(dockerfileBytes)
			if dockerfileBytesString != tt.wantDockerfile {
				t.Errorf("Dockerfile error: expected %v, got %v", tt.wantDockerfile, dockerfileBytesString)
			}
		})
	}
}
