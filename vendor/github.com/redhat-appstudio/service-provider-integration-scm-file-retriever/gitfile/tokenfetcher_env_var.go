// Copyright (c) 2021 - 2022 Red Hat, Inc.
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
	"context"
	"errors"
	"os"
)

var (
	noEnvVarFoundError = errors.New("no TOKEN variable found in env")
)

// EnvVarTokenFetcher token fetcher implementation that looks for token in the specific ENV variable.
type EnvVarTokenFetcher struct{}

func (s *EnvVarTokenFetcher) BuildHeader(context.Context, string, string, func(ctx context.Context, url string)) (*HeaderStruct, error) {
	envToken := os.Getenv("TOKEN")
	if len(envToken) == 0 {
		return nil, noEnvVarFoundError
	}
	return &HeaderStruct{
		"Bearer " + envToken,
	}, nil
}
