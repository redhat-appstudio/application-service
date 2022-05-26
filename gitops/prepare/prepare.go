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
	"fmt"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
)

const (
	// namespace where the bundle configuration will be searched in case it is not found in the component's namespace
	BuildBundleDefaultNamespace = "build-templates"
	// name for a configMap that holds the URL to a build bundle
	BuildBundleConfigMapName = "build-pipelines-defaults"
	// data key within a configMap that holds the URL to a build bundle
	BuildBundleConfigMapKey = "default_build_bundle"
	// fallback bundle that will be used in case the bundle resolution fails
	FallbackBuildBundle = "quay.io/redhat-appstudio/build-templates-bundle:8201a567956ba6d2095d615ea2c0f6ab35f9ba5f"
	// default secret for app studio registry
	RegistrySecret = "redhat-appstudio-registry-pull-secret"
)

// Holds data that needs to be queried from the cluster in order for the gitops generation function to work
// This struct is left here so more data can be added as needed
type GitopsConfig struct {
	BuildBundle string

	AppStudioRegistrySecretPresent bool

	Log *logr.Logger
}

func PrepareGitopsConfig(ctx context.Context, cli client.Client, component appstudiov1alpha1.Component, log *logr.Logger) GitopsConfig {
	data := GitopsConfig{}

	data.BuildBundle = resolveBuildBundle(ctx, cli, component)
	data.AppStudioRegistrySecretPresent = resolveRegistrySecretPresence(ctx, cli, component, log)
	if log != nil {
		log.Info(fmt.Sprintf("GGM PrepareGitopsConfig for component %s:%s AppStudioRegistrySecretPresent %v", component.Namespace, component.Name, data.AppStudioRegistrySecretPresent))
	}
	data.Log = log

	return data
}

// Tries to load a custom build bundle path from a configmap.
// The following priority is used: component's namespace -> default namespace -> fallback value.
func resolveBuildBundle(ctx context.Context, cli client.Client, component appstudiov1alpha1.Component) string {
	namespaces := [2]string{component.Namespace, BuildBundleDefaultNamespace}

	for _, namespace := range namespaces {
		var configMap = corev1.ConfigMap{}

		// All errors during the loading of the configmaps should be treated as non-fatal
		// TODO: Add logging to help the debugging of eventual issues
		_ = cli.Get(ctx, types.NamespacedName{Name: BuildBundleConfigMapName, Namespace: namespace}, &configMap)

		if value, isPresent := configMap.Data[BuildBundleConfigMapKey]; isPresent && value != "" {
			return value
		}
	}

	return FallbackBuildBundle
}

// Determines whether the 'redhat-appstudio-registry-pull-secret' Secret exists, so that the Generate* functions
// can avoid declaring a secret volume workspace for the Secret when the Secret is not available.
func resolveRegistrySecretPresence(ctx context.Context, cli client.Client, component appstudiov1alpha1.Component, log *logr.Logger) bool {
	registrySecret := &corev1.Secret{}
	name := types.NamespacedName{Name: RegistrySecret, Namespace: component.Namespace}
	err := cli.Get(ctx, name, registrySecret)
	if log != nil {
		log.Info(fmt.Sprintf("GGM resolveRegistrySecretPresence err on secret %s: %#v", name, err))
		log.Info(fmt.Sprintf("GGM resolveRegistrySecretPresence secret %s: %#v", name, registrySecret))
	}
	return err == nil
}
