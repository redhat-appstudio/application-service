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

	"github.com/konflux-ci/application-api/api/v1alpha1"
	spiapi "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	K8sClient client.Client
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
func (s MockSPIClient) GetFileContents(ctx context.Context, name string, component v1alpha1.Component, repoURL string, filepath string, ref string) (io.ReadCloser, error) {
	if strings.Contains(repoURL, "test-error-response") {
		return nil, fmt.Errorf("file not found")
	} else if strings.Contains(repoURL, "test-parse-error") || (strings.Contains(repoURL, "test-error-dockerfile-response")) {
		mockReadCloser := mockReadCloser{}
		mockReadCloser.On("Read", mock.AnythingOfType("[]uint8")).Return(0, fmt.Errorf("error reading"))
		mockReadCloser.On("Close").Return(fmt.Errorf("error closing"))
		return &mockReadCloser, nil
	} else if strings.Contains(repoURL, "create-spi-fcr") {
		log := ctrl.LoggerFrom(ctx)
		spiFCRLookupKey := types.NamespacedName{Name: SPIFCR_prefix + name, Namespace: component.Namespace}
		spiFCR := &spiapi.SPIFileContentRequest{}
		spiFCR.Name = spiFCRLookupKey.Name
		spiFCR.Namespace = spiFCRLookupKey.Namespace
		spiFCR.Spec.RepoUrl = repoURL
		spiFCR.Spec.FilePath = filepath
		spiFCR.Spec.Ref = ref
		//add an owner reference
		ownerReference := metav1.OwnerReference{
			APIVersion: component.APIVersion,
			Kind:       component.Kind,
			Name:       component.Name,
			UID:        component.UID,
		}
		spiFCR.SetOwnerReferences(append(spiFCR.GetOwnerReferences(), ownerReference))
		err := s.K8sClient.Create(ctx, spiFCR)
		if err != nil {
			return nil, &SPIFileContentRequestError{fmt.Sprintf("Failed to create an SPIFileContentRequest CR: %s", err.Error())}
		}

		if strings.Contains(repoURL, "create-spi-fcr-return-devfile") {
			stringReader := strings.NewReader(mockDevfile)
			stringReadCloser := io.NopCloser(stringReader)
			return stringReadCloser, nil
		}

		return getFileContentFromSPIFCR(*spiFCR, log)
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
