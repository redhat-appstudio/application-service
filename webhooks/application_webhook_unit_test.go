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

package webhooks

import (
	"context"
	"testing"

	appstudiov1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/stretchr/testify/assert"
)

func TestApplicationValidatingWebhook(t *testing.T) {

	originalApplication := appstudiov1alpha1.Application{
		Spec: appstudiov1alpha1.ApplicationSpec{
			DisplayName: "My App",
			AppModelRepository: appstudiov1alpha1.ApplicationGitRepository{
				URL: "http://appmodelrepo",
			},
			GitOpsRepository: appstudiov1alpha1.ApplicationGitRepository{
				URL: "http://gitopsrepo",
			},
		},
	}

	tests := []struct {
		name      string
		updateApp appstudiov1alpha1.Application
		err       string
	}{
		{
			name: "display name can be changed",
			updateApp: appstudiov1alpha1.Application{
				Spec: appstudiov1alpha1.ApplicationSpec{
					DisplayName: "My App 2",
					AppModelRepository: appstudiov1alpha1.ApplicationGitRepository{
						URL: "http://appmodelrepo",
					},
					GitOpsRepository: appstudiov1alpha1.ApplicationGitRepository{
						URL: "http://gitopsrepo",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var err error

			appWebhook := ApplicationWebhook{
				log: zap.New(zap.UseFlagOptions(&zap.Options{
					Development: true,
					TimeEncoder: zapcore.ISO8601TimeEncoder,
				})),
			}

			err = appWebhook.ValidateUpdate(context.Background(), &originalApplication, &test.updateApp)

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.err)
			}
		})
	}
}

func TestApplicationDeleteValidatingWebhook(t *testing.T) {

	tests := []struct {
		name string
		app  appstudiov1alpha1.Application
		err  string
	}{
		{
			name: "ValidateDelete should return nil, it's unimplimented",
			err:  "",
			app:  appstudiov1alpha1.Application{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			appWebhook := ApplicationWebhook{
				log: zap.New(zap.UseFlagOptions(&zap.Options{
					Development: true,
					TimeEncoder: zapcore.ISO8601TimeEncoder,
				})),
			}

			err := appWebhook.ValidateDelete(context.Background(), &test.app)

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.err)
			}
		})
	}
}
