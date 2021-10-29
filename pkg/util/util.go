//
// Copyright 2021 Red Hat, Inc.
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

package util

import (
	"strings"

	gofakeit "github.com/brianvoe/gofakeit/v6"
)

const AppStudioDataOrg = "https://github.com/redhat-appstudio-appdata/"

func SanitizeDisplayName(displayName string) string {
	sanitizedName := strings.ToLower(strings.Replace(displayName, " ", "-", -1))
	if len(sanitizedName) > 50 {
		sanitizedName = sanitizedName[0:50]
	}
	return sanitizedName
}

func GenerateNewRepository(displayName string) string {
	sanitizedName := SanitizeDisplayName(displayName)

	repoName := sanitizedName + "-" + gofakeit.Verb() + "-" + gofakeit.Noun()
	repository := AppStudioDataOrg + repoName
	return repository
}
