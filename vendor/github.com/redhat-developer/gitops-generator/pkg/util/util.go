/* Copyright 2022 Red Hat, Inc.

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

package util

import (
	"errors"
	"net/url"
)

var invalidRemoteMsg = errors.New("remote URL is invalid or missing the https scheme and/or supported github.com or gitlab.com hosts")

// ValidateRemote minimally validates the remote gitops URL to ensure it contains the "https" scheme and supported "github.com" and "gitlab.com" hosts
func ValidateRemote(remote string) error {
	remoteURL, parseErr := url.Parse(remote)
	if parseErr != nil {
		return invalidRemoteMsg
	}

	if remoteURL.Scheme == "https" && (remoteURL.Host == "github.com" || remoteURL.Host == "gitlab.com") {
		return nil
	}

	return invalidRemoteMsg
}
