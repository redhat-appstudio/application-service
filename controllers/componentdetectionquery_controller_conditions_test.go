/*
Copyright 2021-2022 Red Hat, Inc.

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

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

func TestUpdateComponentName(t *testing.T) {
	ctx := context.Background()
	fakeClientNoError := NewFakeClient(t)
	fakeClientNoError.MockGet = func(ctx context.Context, key types.NamespacedName, obj client.Object) error {
		return nil
	}
	fakeClientHCExist := NewFakeClient(t)
	fakeClientHCExist.MockGet = func(ctx context.Context, key types.NamespacedName, obj client.Object) error {
		hc := appstudiov1alpha1.Component{
			Spec: appstudiov1alpha1.ComponentSpec{
				ComponentName: "devfile-sample-go-basic",
			},
			Status: appstudiov1alpha1.ComponentStatus{},
		}
		data, _ := json.Marshal(hc)

		json.Unmarshal(data, obj)
		return nil
	}
	fakeClientWithError := NewFakeClient(t)
	fakeClientWithError.MockGet = func(ctx context.Context, key types.NamespacedName, obj client.Object) error {
		return fmt.Errorf("some error")
	}

	tests := []struct {
		name                 string
		client               client.Client
		cdq                  *appstudiov1alpha1.ComponentDetectionQuery
		expectedName         []string
		expectedRandomString bool
	}{
		{
			name: "valid repo name",
			cdq: &appstudiov1alpha1.ComponentDetectionQuery{
				Status: appstudiov1alpha1.ComponentDetectionQueryStatus{
					ComponentDetected: appstudiov1alpha1.ComponentDetectionMap{
						"component1": appstudiov1alpha1.ComponentDetectionDescription{
							ComponentStub: appstudiov1alpha1.ComponentSpec{
								ComponentName: "component1",
								Source: appstudiov1alpha1.ComponentSource{
									ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
										GitSource: &appstudiov1alpha1.GitSource{
											URL: "https://github.com/devfile-samples/devfile-sample-go-basic",
										},
									},
								},
							},
						},
					},
				},
			},
			client:       fakeClientNoError,
			expectedName: []string{"devfile-sample-go-basic"},
		},
		{
			name: "long repo name with special chars",
			cdq: &appstudiov1alpha1.ComponentDetectionQuery{
				Status: appstudiov1alpha1.ComponentDetectionQueryStatus{
					ComponentDetected: appstudiov1alpha1.ComponentDetectionMap{
						"component1": appstudiov1alpha1.ComponentDetectionDescription{
							ComponentStub: appstudiov1alpha1.ComponentSpec{
								ComponentName: "component1",
								Source: appstudiov1alpha1.ComponentSource{
									ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
										GitSource: &appstudiov1alpha1.GitSource{
											URL: "https://github.com/devfile-samples/123-testdevfilego--ImportRepository--withaverylongreporitoryname-test-validation-and-generation",
										},
									},
								},
							},
						},
					},
				},
			},
			client:       fakeClientNoError,
			expectedName: []string{"123-testdevfilego--importrepository--withaverylongreporito"},
		},
		{
			name: "numeric repo name",
			cdq: &appstudiov1alpha1.ComponentDetectionQuery{
				Status: appstudiov1alpha1.ComponentDetectionQueryStatus{
					ComponentDetected: appstudiov1alpha1.ComponentDetectionMap{
						"component1": appstudiov1alpha1.ComponentDetectionDescription{
							ComponentStub: appstudiov1alpha1.ComponentSpec{
								ComponentName: "component1",
								Source: appstudiov1alpha1.ComponentSource{
									ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
										GitSource: &appstudiov1alpha1.GitSource{
											URL: "https://github.com/devfile-samples/123454678.git",
										},
									},
								},
							},
						},
					},
				},
			},
			client:       fakeClientNoError,
			expectedName: []string{"comp-123454678"},
		},
		{
			name: "error when look for hc",
			cdq: &appstudiov1alpha1.ComponentDetectionQuery{
				Status: appstudiov1alpha1.ComponentDetectionQueryStatus{
					ComponentDetected: appstudiov1alpha1.ComponentDetectionMap{
						"component1": appstudiov1alpha1.ComponentDetectionDescription{
							ComponentStub: appstudiov1alpha1.ComponentSpec{
								ComponentName: "component1",
								Source: appstudiov1alpha1.ComponentSource{
									ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
										GitSource: &appstudiov1alpha1.GitSource{
											URL: "https://github.com/devfile-samples/devfile-sample-go-basic",
										},
									},
								},
							},
						},
					},
				},
			},
			client:               fakeClientWithError,
			expectedName:         []string{"devfile-sample-go-basic"},
			expectedRandomString: true,
		},
		{
			name: "hc exist with conflict name",
			cdq: &appstudiov1alpha1.ComponentDetectionQuery{
				Status: appstudiov1alpha1.ComponentDetectionQueryStatus{
					ComponentDetected: appstudiov1alpha1.ComponentDetectionMap{
						"component1": appstudiov1alpha1.ComponentDetectionDescription{
							ComponentStub: appstudiov1alpha1.ComponentSpec{
								ComponentName: "component1",
								Source: appstudiov1alpha1.ComponentSource{
									ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
										GitSource: &appstudiov1alpha1.GitSource{
											URL: "https://github.com/devfile-samples/devfile-sample-go-basic",
										},
									},
								},
							},
						},
					},
				},
			},
			client:               fakeClientHCExist,
			expectedName:         []string{"devfile-sample-go-basic"},
			expectedRandomString: true,
		},
		{
			name: "valid repo name with multi-components",
			cdq: &appstudiov1alpha1.ComponentDetectionQuery{
				Status: appstudiov1alpha1.ComponentDetectionQueryStatus{
					ComponentDetected: appstudiov1alpha1.ComponentDetectionMap{
						"component1": appstudiov1alpha1.ComponentDetectionDescription{
							ComponentStub: appstudiov1alpha1.ComponentSpec{
								ComponentName: "component1",
								Source: appstudiov1alpha1.ComponentSource{
									ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
										GitSource: &appstudiov1alpha1.GitSource{
											URL:     "https://github.com/devfile-samples/devfile-multi-component",
											Context: "nodejs",
										},
									},
								},
							},
						},
						"component2": appstudiov1alpha1.ComponentDetectionDescription{
							ComponentStub: appstudiov1alpha1.ComponentSpec{
								ComponentName: "component2",
								Source: appstudiov1alpha1.ComponentSource{
									ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
										GitSource: &appstudiov1alpha1.GitSource{
											URL:     "https://github.com/devfile-samples/devfile-multi-component",
											Context: "java-springboot",
										},
									},
								},
							},
						},
					},
				},
			},
			client:       fakeClientNoError,
			expectedName: []string{"nodejs-devfile-multi-component", "java-springboot-devfile-multi-component"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateComponentName(ctx, tt.cdq, tt.client)
			if tt.expectedRandomString {
				assert.Contains(t, (tt.cdq.Status.ComponentDetected["component1"].ComponentStub.ComponentName), tt.expectedName[0], "the component name should contain repo name")
				assert.NotEqual(t, tt.expectedName[0], (tt.cdq.Status.ComponentDetected["component1"].ComponentStub.ComponentName), "the component name should not equal to repo name")
			} else {
				assert.Equal(t, tt.expectedName[0], (tt.cdq.Status.ComponentDetected["component1"].ComponentStub.ComponentName), "the component name does not match expected name")
				if len(tt.expectedName) > 1 {
					assert.Equal(t, tt.expectedName[1], (tt.cdq.Status.ComponentDetected["component2"].ComponentStub.ComponentName), "the component name does not match expected name")
				}
			}
		})
	}

}
