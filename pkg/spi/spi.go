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
	"io"
	"path/filepath"

	cdqanalysis "github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	"github.com/redhat-appstudio/application-service/pkg/devfile"
	"github.com/redhat-appstudio/service-provider-integration-scm-file-retriever/gitfile"
)

type SPI interface {
	GetFileContents(ctx context.Context, namespace string, repoUrl string, filepath string, ref string, callback func(ctx context.Context, url string)) (io.ReadCloser, error)
}

type SPIClient struct {
}

// GetFileContents is a wrapper call to scm file retriever's GetFileContents()
func (s SPIClient) GetFileContents(ctx context.Context, namespace string, repoUrl string, filepath string, ref string, callback func(ctx context.Context, url string)) (io.ReadCloser, error) {
	return gitfile.Default().GetFileContents(ctx, namespace, repoUrl, filepath, ref, callback)
}

func DownloadDevfileUsingSPI(s SPI, ctx context.Context, namespace string, repoURL string, ref string, path string) ([]byte, error) {
	validDevfileLocations := []string{cdqanalysis.Devfile, cdqanalysis.HiddenDevfile, cdqanalysis.HiddenDirDevfile, cdqanalysis.HiddenDirHiddenDevfile}

	for _, filename := range validDevfileLocations {
		devfileBytes, err := DownloadFileUsingSPI(s, ctx, namespace, repoURL, ref, filepath.Join("/", path, filename))
		if err == nil {
			return devfileBytes, nil
		} else {
			if _, ok := err.(*devfile.NoFileFound); !ok {
				return nil, err
			}
		}
	}

	return nil, &cdqanalysis.NoDevfileFound{Location: repoURL}
}

func DownloadFileUsingSPI(s SPI, ctx context.Context, namespace string, repoURL string, ref string, filepath string) ([]byte, error) {

	// Call out to SPI via scm-file-retriever to get the file from the given repository
	// Hardcode the callback function to nil - no way for us to handle this right now
	r, err := s.GetFileContents(ctx, namespace, repoURL, filepath, ref, func(ctx context.Context, url string) {})
	if err == nil {
		fileBytes, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		return fileBytes, nil
	}

	return nil, &devfile.NoFileFound{Location: repoURL}
}

func DownloadDevfileandDockerfileUsingSPI(s SPI, ctx context.Context, namespace string, repoURL string, ref string, path string) ([]byte, []byte, error) {

	devfileBytes, err := DownloadDevfileUsingSPI(s, ctx, namespace, repoURL, ref, path)
	if err != nil {
		if _, ok := err.(*cdqanalysis.NoDevfileFound); !ok {
			return nil, nil, err
		}
	}

	dockerfileBytes, err := DownloadFileUsingSPI(s, ctx, namespace, repoURL, ref, filepath.Join("/", path, "Dockerfile"))
	if err != nil {
		if _, ok := err.(*devfile.NoFileFound); !ok {
			return nil, nil, err
		}
	}

	return devfileBytes, dockerfileBytes, nil
}
