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

package webhooks

import (
	"context"
	"fmt"
	"testing"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestComponentCreateValidatingWebhook(t *testing.T) {

	tests := []struct {
		name    string
		newComp appstudiov1alpha1.Component
		err     string
	}{
		{
			name: "component metadata.name is invalid",
			err:  "invalid component name",
			newComp: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name: "1-test-component",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
					Application:   "application1",
				},
			},
		},
		{
			name: "component cannot be created due to bad URL",
			err:  "invalid URI for request" + appstudiov1alpha1.InvalidSchemeGitSourceURL,
			newComp: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-component",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
					Application:   "application1",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "badurl",
							},
						},
					},
				},
			},
		},
		{
			name: "component needs to have one source specified",
			err:  appstudiov1alpha1.MissingGitOrImageSource,
			newComp: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-component",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
					Application:   "application1",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{},
						},
					},
				},
			},
		},
		{
			name: "valid component with invalid git vendor src",
			err:  fmt.Errorf(appstudiov1alpha1.InvalidGithubVendorURL, "http://url", SupportedGitRepo).Error(),
			newComp: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-component",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
					Application:   "application1",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "http://url",
							},
						},
					},
				},
			},
		},
		{
			name: "valid component with invalid git scheme src",
			err:  "invalid URI for request" + appstudiov1alpha1.InvalidSchemeGitSourceURL,
			newComp: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-component",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
					Application:   "application1",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL: "git@github.com:devfile-samples/devfile-sample-java-springboot-basic.git",
							},
						},
					},
				},
			},
		},
		{
			name: "valid component with container image",
			newComp: appstudiov1alpha1.Component{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-component",
				},
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName:  "component1",
					Application:    "application1",
					ContainerImage: "image",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compWebhook := ComponentWebhook{
				log: zap.New(zap.UseFlagOptions(&zap.Options{
					Development: true,
					TimeEncoder: zapcore.ISO8601TimeEncoder,
				})),
			}
			err := compWebhook.ValidateCreate(context.Background(), &test.newComp)

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.err)
			}
		})
	}
}

func TestComponentUpdateValidatingWebhook(t *testing.T) {

	originalComponent := appstudiov1alpha1.Component{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-component",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName: "component",
			Application:   "application",
			Source: appstudiov1alpha1.ComponentSource{
				ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
					GitSource: &appstudiov1alpha1.GitSource{
						URL:     "http://link",
						Context: "context",
					},
				},
			},
		},
	}

	tests := []struct {
		name       string
		updateComp appstudiov1alpha1.Component
		err        string
	}{
		{
			name: "component name cannot be changed",
			err:  fmt.Errorf(appstudiov1alpha1.ComponentNameUpdateError, "component1").Error(),
			updateComp: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component1",
				},
			},
		},
		{
			name: "application name cannot be changed",
			err:  fmt.Errorf(appstudiov1alpha1.ApplicationNameUpdateError, "application1").Error(),
			updateComp: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component",
					Application:   "application1",
				},
			},
		},
		{
			name: "git src url cannot be changed",
			err: fmt.Errorf(appstudiov1alpha1.GitSourceUpdateError, appstudiov1alpha1.GitSource{
				URL:     "http://link1",
				Context: "context",
			}).Error(),
			updateComp: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component",
					Application:   "application",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								URL:     "http://link1",
								Context: "context",
							},
						},
					},
				},
			},
		},
		{
			name: "non-url git source can be changed",
			updateComp: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName: "component",
					Application:   "application",
					Source: appstudiov1alpha1.ComponentSource{
						ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
							GitSource: &appstudiov1alpha1.GitSource{
								Context:       "new-context",
								DevfileURL:    "https://new-devfile-url",
								DockerfileURL: "https://new-dockerfile-url",
							},
						},
					},
				},
			},
		},
		{
			name: "container image can be changed",
			updateComp: appstudiov1alpha1.Component{
				Spec: appstudiov1alpha1.ComponentSpec{
					ComponentName:  "component",
					Application:    "application",
					ContainerImage: "image1",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.err == "" {
				originalComponent = appstudiov1alpha1.Component{
					Spec: appstudiov1alpha1.ComponentSpec{
						ComponentName:  "component",
						Application:    "application",
						ContainerImage: "image",
						Source: appstudiov1alpha1.ComponentSource{
							ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
								GitSource: &appstudiov1alpha1.GitSource{
									Context: "context",
								},
							},
						},
					},
				}
			}
			var err error
			compWebhook := ComponentWebhook{
				log: zap.New(zap.UseFlagOptions(&zap.Options{
					Development: true,
					TimeEncoder: zapcore.ISO8601TimeEncoder,
				})),
			}
			err = compWebhook.ValidateUpdate(context.Background(), &originalComponent, &test.updateComp)

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.err)
			}
		})
	}
}

func TestComponentDeleteValidatingWebhook(t *testing.T) {

	tests := []struct {
		name    string
		newComp appstudiov1alpha1.Component
		err     string
	}{
		{
			name:    "ValidateDelete should return nil, it's unimplemented",
			err:     "",
			newComp: appstudiov1alpha1.Component{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compWebhook := ComponentWebhook{
				log: zap.New(zap.UseFlagOptions(&zap.Options{
					Development: true,
					TimeEncoder: zapcore.ISO8601TimeEncoder,
				})),
			}
			err := compWebhook.ValidateDelete(context.Background(), &test.newComp)

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.err)
			}
		})
	}
}
