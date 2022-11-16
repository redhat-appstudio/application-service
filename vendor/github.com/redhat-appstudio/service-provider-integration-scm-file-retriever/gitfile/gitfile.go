// Copyright (c) 2022 Red Hat, Inc.
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

package gitfile

import (
	"bytes"
	"context"
	"io"

	"github.com/imroc/req"
)

type GitFile struct {
	fetcher TokenFetcher
}

// GetFileContents is a main entry function allowing to retrieve file content from the SCM provider.
// It expects three file location parameters, from which the repository URL and path to the file are mandatory,
// and optional Git reference for the branch/tags/commitIds.
// Function type parameter is a callback used when user authentication is needed in order to retrieve the file,
// that function will be called with the URL to OAuth service, where user need to be redirected.
func (g *GitFile) GetFileContents(ctx context.Context, namespace, repoUrl, filepath, ref string, callback func(ctx context.Context, url string)) (io.ReadCloser, error) {
	headerStruct, err := buildAuthHeader(ctx, namespace, repoUrl, g.fetcher, callback)
	if err != nil {
		return nil, err
	}
	authHeader := req.HeaderFromStruct(headerStruct)
	fileUrl, err := detect(repoUrl, filepath, ref, authHeader)
	if err != nil {
		return nil, err
	}

	response, _ := req.Get(fileUrl, ctx, authHeader)
	return io.NopCloser(bytes.NewBuffer(response.Bytes())), nil
}

// New creates a new *GitFile instance
func New(fetcher TokenFetcher) *GitFile {
	return &GitFile{fetcher: fetcher}
}

func Default() *GitFile {
	return &GitFile{fetcher: NewSpiTokenFetcher()}
}
