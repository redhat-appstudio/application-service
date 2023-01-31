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
	"strings"

	"github.com/redhat-appstudio/application-service/pkg/util"
)

// UpdateGitLink updates the relative uri
// to a full URL link with the context & revision
func UpdateGitLink(repo, revision, context string) (string, error) {
	var rawGitURL string
	var err error
	if !strings.HasPrefix(context, "http") {
		rawGitURL, err = util.ConvertGitHubURL(repo, revision, context)
		if err != nil {
			return "", err
		}

	} else {
		return context, nil
	}

	return rawGitURL, nil
}
