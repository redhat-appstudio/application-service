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
	"fmt"
	"regexp"
)

// GetIngressHostName gets the ingress host name from the component name, namepsace and ingress domain
func GetIngressHostName(componentName, namespace, ingressDomain string) (string, error) {

	regexString := `[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*`
	ingressHostRegex := regexp.MustCompile(regexString)

	host := fmt.Sprintf("%s-%s.%s", componentName, namespace, ingressDomain)

	if !ingressHostRegex.MatchString(host) {
		return "", fmt.Errorf("hostname %s should match regex %s", host, regexString)
	}

	return host, nil
}
