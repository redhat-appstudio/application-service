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
	"fmt"
	"io"
	"strings"

	"github.com/stretchr/testify/mock"
)

type mockReadCloser struct {
	mock.Mock
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *mockReadCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockSPIClient struct {
}

var mockDevfile = `
schemaVersion: 2.2.0
metadata:
  displayName: Go Runtime
  icon: https://raw.githubusercontent.com/devfile-samples/devfile-stack-icons/main/golang.svg
  language: go
  name: go
  projectType: go
  tags:
    - Go
  version: 1.0.0
starterProjects:
  - name: go-starter
    git:
      checkoutFrom:
        revision: main
      remotes:
        origin: https://github.com/devfile-samples/devfile-stack-go.git
components:
  - container:
      endpoints:
        - name: http
          targetPort: 8080
      image: golang:latest
      memoryLimit: 1024Mi
      mountSources: true
      sourceMapping: /project
    name: runtime
  - name: image-build
    image:
      imageName: go-image:latest
      dockerfile:
        uri: docker/Dockerfile
        buildContext: .
        rootRequired: false
  - name: kubernetes-deploy
    kubernetes:
      inlined: |-
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          creationTimestamp: null
          labels:
            maysun: test
          name: deploy-sample
      endpoints:
      - name: http-8081
        targetPort: 8081
        path: /
commands:
  - exec:
      commandLine: GOCACHE=/project/.cache go build main.go
      component: runtime
      group:
        isDefault: true
        kind: build
      workingDir: /project
    id: build
  - exec:
      commandLine: ./main
      component: runtime
      group:
        isDefault: true
        kind: run
      workingDir: /project
    id: run
  - id: build-image
    apply:
      component: image-build
  - id: deployk8s
    apply:
      component: kubernetes-deploy
  - id: deploy
    composite:
      commands:
        - build-image
        - deployk8s
      group:
        kind: deploy
        isDefault: true
`

var mockDockerfile = `
FROM python:slim

WORKDIR /projects

RUN python3 -m venv venv
RUN . venv/bin/activate

# optimize image caching
COPY requirements.txt .
RUN pip install -r requirements.txt

COPY . .

EXPOSE 8081
CMD [ "waitress-serve", "--port=8081", "app:app"]
`

// GetFileContents mocks the GetFileContents function from SPI
// If "repoURL" parameter contains "test-error-response", then an error value will be returned,
// otherwise we return a mock devfile that can be read.
func (s MockSPIClient) GetFileContents(ctx context.Context, namespace string, repoURL string, filepath string, ref string, callback func(ctx context.Context, url string)) (io.ReadCloser, error) {
	if strings.Contains(repoURL, "test-error-response") {
		return nil, fmt.Errorf("file not found")
	} else if strings.Contains(repoURL, "test-parse-error") || (strings.Contains(repoURL, "test-error-dockerfile-response") && strings.Contains(filepath, "Dockerfile")) {
		mockReadCloser := mockReadCloser{}
		mockReadCloser.On("Read", mock.AnythingOfType("[]uint8")).Return(0, fmt.Errorf("error reading"))
		mockReadCloser.On("Close").Return(fmt.Errorf("error closing"))
		return &mockReadCloser, nil
	} else if strings.Contains(filepath, "Dockerfile") {
		stringReader := strings.NewReader(mockDockerfile)
		stringReadCloser := io.NopCloser(stringReader)
		return stringReadCloser, nil
	} else {
		stringReader := strings.NewReader(mockDevfile)
		stringReadCloser := io.NopCloser(stringReader)
		return stringReadCloser, nil
	}
}
