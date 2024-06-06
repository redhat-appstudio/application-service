/*
Copyright 2021 Red Hat, Inc.

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
package prepare

import (
	"context"
	"reflect"
	"testing"

	appstudiov1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPrepareGitopsConfig(t *testing.T) {

	component := appstudiov1alpha1.Component{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myName",
			Namespace: "myNamespace",
		},
	}
	tests := []struct {
		name      string
		pacSecret corev1.Secret
		want      GitopsConfig
	}{
		{
			name: "should resolve the build bundle in case a configmap exists in the component's namespace",
			pacSecret: corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      PipelinesAsCodeSecretName,
					Namespace: buildServiceNamespaceName,
				},
				Data: map[string][]byte{
					"github.token": []byte("ghp_token"),
				},
			},
			want: GitopsConfig{
				PipelinesAsCodeCredentials: map[string][]byte{
					"github.token": []byte("ghp_token"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithRuntimeObjects(&tt.pacSecret).Build()
			if got := PrepareGitopsConfig(context.TODO(), client, component); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PrepareGitopsConfig() = %v, want %v", got, tt.want)
			}
		})
	}

}

func TestGetPipelinesAsCodeConfigurationSecretData(t *testing.T) {
	ctx := context.TODO()

	component := appstudiov1alpha1.Component{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myName",
			Namespace: "myNamespace",
		},
	}

	tests := []struct {
		name string
		data *corev1.Secret
		want map[string][]byte
	}{
		{
			name: "secret exists",
			data: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: buildServiceNamespaceName,
					Name:      PipelinesAsCodeSecretName,
				},
				Data: map[string][]byte{
					"github.token": []byte("ghp_token"),
				},
			},
			want: map[string][]byte{
				"github.token": []byte("ghp_token"),
			},
		},
		{
			name: "secret does not exist",
			want: map[string][]byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client crclient.WithWatch
			client = fake.NewClientBuilder().Build()
			if tt.data != nil {
				client = fake.NewClientBuilder().WithRuntimeObjects(tt.data).Build()
			}

			if got := getPipelinesAsCodeConfigurationSecretData(ctx, client, component); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}

}
