//
// Copyright 2023 Red Hat, Inc.
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
	"net/url"
	"strings"
	"testing"
)

// run this test locally using go test -fuzz={FuzzTestName} in the test directory
func FuzzConvertGitHubURL(f *testing.F) {

	//Add seed corpus
	f.Add("http:raw.githubusercontent.com", "0", "")
	f.Add("https://raw.githubusercontent.com/devfile-samples/devfile-sample-java-springboot-basic/testbranch/devfile.yaml", "testbranch", ".")
	f.Add("https://github.com/devfile/api/tree/2.1.x", "a!jro@", "#abc!")
	f.Add("http://github.com/devfile-samples/devfile-sample-java-springboot-basic", "master", "/context")
	f.Add("/devfile/api/sample", "%", "context")
	f.Add("git:github.com/devfile/api/tree/2.1.x", "main", "")

	f.Fuzz(func(t *testing.T, gitURL string, gitRevision string, context string) {
		convertedURL, err := ConvertGitHubURL(gitURL, gitRevision, context)

		//if there's an error, URL should not be converted
		if err != nil && convertedURL != "" {
			t.Errorf("Github URL should be empty %s ", convertedURL)
		}

		parsedGitURL, err := url.Parse(gitURL)
		if err == nil && (parsedGitURL.Scheme == "https" || parsedGitURL.Scheme == "http") {
			//if the input git URL is valid, then converted URL should contain the replacement text
			if parsedGitURL.Host == githubHost {
				if !strings.Contains(convertedURL, githubReplacementString) {
					t.Errorf("Github URL is not properly converted %s ", convertedURL)
				}
				//conversely, an invalid URL should not be converted
			} else {
				if strings.Contains(convertedURL, githubReplacementString) {
					t.Errorf("Github URL should not have been converted %s ", convertedURL)
				}
			}
		}
	})
}
