/*
Copyright 2023 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pkg

import "github.com/go-logr/logr"

type CDQUtilMockClient struct {
}

func NewCDQUtilMockClient() CDQUtilMockClient {
	return CDQUtilMockClient{}
}

func (cdqUtilMockClient CDQUtilMockClient) Clone(k K8sInfoClient, cdqInfo *CDQInfo, namespace, name, context string) error {

	if cdqInfo == nil {
		return nil
	}

	if cdqInfo.GitURL.Token == "valid-mock-token" {
		// This is a private repository case
		// Since we are not actually testing with a valid token,
		// mock the clone by calling a valid public repository instead.
		// This way, we can plug into the existing code where
		// CDQ can search for a devfile or a Dockerfile and
		// possibly use Alizer. By doing this check,
		// CDQ now assumes its a public repository path
		// by working around it and we can proceed
		// with the private repository test

		cdqInfo.GitURL.Token = ""
	}

	return clone(k, cdqInfo, namespace, name, context)
}

func (cdqUtilMockClient CDQUtilMockClient) ValidateDevfile(log logr.Logger, devfileLocation string, token string) (shouldIgnoreDevfile bool, devfileBytes []byte, err error) {

	if token == "valid-mock-token" {
		// This is a private repository case
		// Since we are not actually testing with a valid token,
		// mock the clone by calling a valid public repository instead.
		// This way, we can plug into the existing code where
		// CDQ can search for a devfile or a Dockerfile and
		// possibly use Alizer. By doing this check,
		// CDQ now assumes its a public repository path
		// by working around it and we can proceed
		// with the private repository test

		token = ""
	}

	return validateDevfile(log, devfileLocation, token)
}
