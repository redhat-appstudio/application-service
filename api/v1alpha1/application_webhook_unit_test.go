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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplicationValidatingWebhook(t *testing.T) {

	originalApplication := Application{
		Spec: ApplicationSpec{
			DisplayName: "My App",
			AppModelRepository: ApplicationGitRepository{
				URL: "http://appmodelrepo",
			},
			GitOpsRepository: ApplicationGitRepository{
				URL: "http://gitopsrepo",
			},
		},
	}

	tests := []struct {
		name      string
		updateApp Application
		err       string
	}{
		{
			name: "app model repo cannot be changed",
			err:  "app model repository cannot be updated",
			updateApp: Application{
				Spec: ApplicationSpec{
					DisplayName: "My App",
					AppModelRepository: ApplicationGitRepository{
						URL: "http://appmodelrepo1",
					},
					GitOpsRepository: ApplicationGitRepository{
						URL: "http://gitopsrepo",
					},
				},
			},
		},
		{
			name: "gitops repo cannot be changed",
			err:  "gitops repository cannot be updated",
			updateApp: Application{
				Spec: ApplicationSpec{
					DisplayName: "My App",
					AppModelRepository: ApplicationGitRepository{
						URL: "http://appmodelrepo",
					},
					GitOpsRepository: ApplicationGitRepository{
						URL: "http://gitopsrepo1",
					},
				},
			},
		},
		{
			name: "display name can be changed",
			updateApp: Application{
				Spec: ApplicationSpec{
					DisplayName: "My App 2",
					AppModelRepository: ApplicationGitRepository{
						URL: "http://appmodelrepo",
					},
					GitOpsRepository: ApplicationGitRepository{
						URL: "http://gitopsrepo",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.updateApp.ValidateUpdate(&originalApplication)
			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.err)
			}
		})
	}
}
