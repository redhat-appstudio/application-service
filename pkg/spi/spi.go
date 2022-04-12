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
	validDevfileLocations := []string{devfile.Devfile, devfile.HiddenDevfile, devfile.HiddenDirDevfile, devfile.HiddenDirHiddenDevfile}

	for _, filename := range validDevfileLocations {
		// Call out to SPI via scm-file-retriever to get the devfile from the given repository
		// Hardcode the callback function to nil - no way for us to handle this right now
		r, err := s.GetFileContents(ctx, namespace, repoURL, filepath.Join("/", path, filename), ref, func(ctx context.Context, url string) {})
		if err == nil {
			devfileBytes, err := io.ReadAll(r)
			if err != nil {
				return nil, err
			}
			return devfileBytes, nil
		}

	}

	return nil, &devfile.NoDevfileFound{Location: repoURL}
}
