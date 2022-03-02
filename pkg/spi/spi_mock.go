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
schemaVersion: 2.0.0
metadata:
  description: Stack with the latest Go version
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
`

// GetFileContents mocks the GetFileContents function from SPI
// If "repoURL" parameter contains "test-error-response", then an error value will be returned,
// otherwise we return a mock devfile that can be read.
func (s MockSPIClient) GetFileContents(ctx context.Context, namespace string, repoURL string, filepath string, ref string, callback func(ctx context.Context, url string)) (io.ReadCloser, error) {
	if strings.Contains(repoURL, "test-error-response") {
		return nil, fmt.Errorf("file not found")
	} else if strings.Contains(repoURL, "test-parse-error") {
		//stringReader := strings.NewReader(make(chan error))
		mockReadCloser := mockReadCloser{}
		mockReadCloser.On("Read", mock.AnythingOfType("[]uint8")).Return(0, fmt.Errorf("error reading"))
		mockReadCloser.On("Close").Return(fmt.Errorf("error closing"))
		return &mockReadCloser, nil
	} else {
		stringReader := strings.NewReader(mockDevfile)
		stringReadCloser := io.NopCloser(stringReader)
		return stringReadCloser, nil
	}
}
