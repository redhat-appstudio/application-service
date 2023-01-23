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

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Pipelines as Code global configuration secret name
	PipelinesAsCodeSecretName = "pipelines-as-code-secret"
	// Pipelines as Code global configuration secret namespace
	buildServiceNamespaceName = "build-service"
)

// Holds data that needs to be queried from the cluster in order for the gitops generation function to work
// This struct is left here so more data can be added as needed
type GitopsConfig struct {
	// Contains data from Pipelies as Code configuration k8s secret
	PipelinesAsCodeCredentials map[string][]byte
}

func PrepareGitopsConfig(ctx context.Context, cli client.Client, component appstudiov1alpha1.Component) GitopsConfig {
	data := GitopsConfig{}

	data.PipelinesAsCodeCredentials = getPipelinesAsCodeConfigurationSecretData(ctx, cli, component)

	return data
}

func getPipelinesAsCodeConfigurationSecretData(ctx context.Context, cli client.Client, component appstudiov1alpha1.Component) map[string][]byte {
	pacSecret := &corev1.Secret{}
	err := cli.Get(ctx, types.NamespacedName{Name: PipelinesAsCodeSecretName, Namespace: buildServiceNamespaceName}, pacSecret)
	if err != nil {
		return make(map[string][]byte)
	}
	return pacSecret.Data
}
