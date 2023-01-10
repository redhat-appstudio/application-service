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

	kcpv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/apis/v1alpha1"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// namespace where the bundle configuration will be searched in case it is not found in the component's namespace
	BuildBundleDefaultNamespace = "build-templates"
	// name for a configMap that holds the URL to a build bundle
	BuildBundleConfigMapName = "build-pipelines-defaults"
	// data key within a configMap that holds the URL to a build bundle
	BuildBundleConfigMapKey = "default_build_bundle"
	HACBSBundleConfigMapKey = "hacbs_build_bundle"

	// Fallback bundle that will be used in case the bundle resolution fails
	// List of AppStudio bundle tags: https://quay.io/repository/redhat-appstudio/build-templates-bundle?tab=tags
	AppStudioFallbackBuildBundle = "quay.io/redhat-appstudio/build-templates-bundle:9f5d549dd64aacf10e3baac90972dfd5df788324"
	// List of HACBS bundle tags: https://quay.io/repository/redhat-appstudio/hacbs-templates-bundle?tab=tags
	HACBSFallbackBuildBundle = "quay.io/redhat-appstudio/hacbs-templates-bundle:9f5d549dd64aacf10e3baac90972dfd5df788324"

	// default secret for app studio registry
	RegistrySecret = "redhat-appstudio-registry-pull-secret"
	// Pipelines as Code global configuration secret name
	PipelinesAsCodeSecretName = "pipelines-as-code-secret"
	// Pipelines as Code global configuration secret namespace
	buildServiceNamespaceName = "build-service"
	// ConfigMap name for detection hacbs workflow
	// Note: HACBS detection by configmap is temporary solution, will be changed to detection based
	// on APIBinding API in KCP environment.
	HACBSConfigMapName = "hacbs"
	// APIBinding name for detection hacbs workflow in KCP environment
	HACBSAPIBindingName = "integration-service"
)

// Holds data that needs to be queried from the cluster in order for the gitops generation function to work
// This struct is left here so more data can be added as needed
type GitopsConfig struct {
	BuildBundle string

	AppStudioRegistrySecretPresent bool

	// Contains data from Pipelies as Code configuration k8s secret
	PipelinesAsCodeCredentials map[string][]byte

	IsHACBS bool
}

func PrepareGitopsConfig(ctx context.Context, cli client.Client, component appstudiov1alpha1.Component) GitopsConfig {
	data := GitopsConfig{}

	data.AppStudioRegistrySecretPresent = resolveRegistrySecretPresence(ctx, cli, component)
	data.IsHACBS = IsHACBS(ctx, cli, component.Namespace)
	data.BuildBundle = ResolveBuildBundle(ctx, cli, component.Namespace, data.IsHACBS)
	data.PipelinesAsCodeCredentials = getPipelinesAsCodeConfigurationSecretData(ctx, cli, component)

	return data
}

// ResolveBuildBundle detects build bundle to use.
// The following priority is used:
// 1. Component's namespace, build-pipelines-defaults ConfigMap
// 2. build-templates namespace, build-pipelines-defaults ConfigMap
// 3. Fallback bundle
func ResolveBuildBundle(ctx context.Context, cli client.Client, namespace string, isHACBS bool) string {
	bundleConfigMapKey := BuildBundleConfigMapKey
	if isHACBS {
		bundleConfigMapKey = HACBSBundleConfigMapKey
	}

	// All errors during the loading of the ConfigMaps should be treated as non-fatal
	configMap := corev1.ConfigMap{}
	if err := cli.Get(ctx, types.NamespacedName{Name: BuildBundleConfigMapName, Namespace: namespace}, &configMap); err == nil {
		if value, isPresent := configMap.Data[bundleConfigMapKey]; isPresent && value != "" {
			return value
		}
	}
	// There is no build bundle configuration in the component namespace
	// Try global build bundle configuration
	if err := cli.Get(ctx, types.NamespacedName{Name: BuildBundleConfigMapName, Namespace: BuildBundleDefaultNamespace}, &configMap); err == nil {
		if value, isPresent := configMap.Data[bundleConfigMapKey]; isPresent && value != "" {
			return value
		}
	}
	// Use fallback bundle
	if isHACBS {
		return HACBSFallbackBuildBundle
	}
	return AppStudioFallbackBuildBundle
}

// Return true when integration-service APIBinding exists or hacbs configmap exists in the namespace
func IsHACBS(ctx context.Context, cli client.Client, namespace string) bool {
	var apiBinding = kcpv1alpha1.APIBinding{}
	err := cli.Get(ctx, types.NamespacedName{Name: HACBSAPIBindingName}, &apiBinding)
	if err == nil {
		return true
	}
	var configMap = corev1.ConfigMap{}
	err = cli.Get(ctx, types.NamespacedName{Name: HACBSConfigMapName, Namespace: namespace}, &configMap)
	return err == nil
}

// Determines whether the 'redhat-appstudio-registry-pull-secret' Secret exists, so that the Generate* functions
// can avoid declaring a secret volume workspace for the Secret when the Secret is not available.
func resolveRegistrySecretPresence(ctx context.Context, cli client.Client, component appstudiov1alpha1.Component) bool {
	registrySecret := &corev1.Secret{}
	err := cli.Get(ctx, types.NamespacedName{Name: RegistrySecret, Namespace: component.Namespace}, registrySecret)
	return err == nil
}

func getPipelinesAsCodeConfigurationSecretData(ctx context.Context, cli client.Client, component appstudiov1alpha1.Component) map[string][]byte {
	pacSecret := &corev1.Secret{}
	err := cli.Get(ctx, types.NamespacedName{Name: PipelinesAsCodeSecretName, Namespace: buildServiceNamespaceName}, pacSecret)
	if err != nil {
		return make(map[string][]byte)
	}
	return pacSecret.Data
}
