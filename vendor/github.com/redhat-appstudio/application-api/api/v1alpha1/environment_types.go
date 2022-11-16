/*
Copyright 2022.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvironmentSpec defines the desired state of Environment
type EnvironmentSpec struct {

	// Type is whether the Environment is a POC or non-POC environment
	Type EnvironmentType `json:"type"`

	// DisplayName is the user-visible, user-definable name for the environment (but not used for functional requirements)
	DisplayName string `json:"displayName"`

	// DeploymentStrategy is the promotion strategy for the Environment
	// See Environment API doc for details.
	DeploymentStrategy DeploymentStrategyType `json:"deploymentStrategy"`

	// ParentEnvironment references another Environment defined in the namespace: when automated promotion is enabled,
	// promotions to the parent environment will cause this environment to be promoted to.
	// See Environment API doc for details.
	ParentEnvironment string `json:"parentEnvironment,omitempty"`

	// Tags are a user-visisble, user-definable set of tags that can be applied to the environment
	Tags []string `json:"tags,omitempty"`

	// Configuration contains environment-specific details for Applications/Components that are deployed to
	// the Environment.
	Configuration EnvironmentConfiguration `json:"configuration,omitempty"`

	// UnstableConfigurationFields are experimental/prototype: the API has not been finalized here, and is subject to breaking changes.
	// See comment on UnstableEnvironmentConfiguration for details.
	UnstableConfigurationFields *UnstableEnvironmentConfiguration `json:"unstableConfigurationFields,omitempty"`
}

// EnvironmentType currently indicates whether an environment is POC/Non-POC, see API doc for details.
type EnvironmentType string

const (
	EnvironmentType_POC    EnvironmentType = "POC"
	EnvironmentType_NonPOC EnvironmentType = "Non-POC"
)

// DeploymentStrategyType defines the available promotion/deployment strategies for an Environment
// See Environment API doc for details.
type DeploymentStrategyType string

const (
	// DeploymentStrategy_Manual: Promotions to an Environment with this strategy will occur due to explicit user intent
	DeploymentStrategy_Manual DeploymentStrategyType = "Manual"

	// DeploymentStrategy_AppStudioAutomated: Promotions to an Environment with this strategy will occur if a previous ("parent")
	// environment in the environment graph was successfully promoted to.
	// See Environment API doc for details.
	DeploymentStrategy_AppStudioAutomated DeploymentStrategyType = "AppStudioAutomated"
)

// UnstableEnvironmentConfiguration contains fields that are related to configuration of the target environment:
// - credentials for connecting to the cluster (if connecting to a non-KCP cluster)
// - KCP workspace configuration credentials (TBD)
//
// Note: as of this writing (Jul 2022), I expect the contents of this struct to undergo major changes, and the API should not be considered
// complete, or even a reflection of final desired state.
type UnstableEnvironmentConfiguration struct {
	KubernetesClusterCredentials `json:"kubernetesCredentials,omitempty"`
}

// KubernetesClusterCredentials allows you to specify cluster credentials for stanadard K8s cluster (e.g. non-KCP workspace).
//
// See this temporary URL for details on what values to provide for the APIURL and Secret:
// https://github.com/redhat-appstudio/managed-gitops/tree/main/examples/m6-demo#gitopsdeploymentmanagedenvironment-resource
type KubernetesClusterCredentials struct {

	// TargetNamespace is the default destination target on the cluster for deployments. This Namespace will be used
	// for any GitOps repository K8s resources where the `.metadata.Namespace` field is not specified.
	TargetNamespace string `json:"targetNamespace"`

	// APIURL is a reference to a cluster API url defined within the kube config file of the cluster credentials secret.
	APIURL string `json:"apiURL"`

	// ClusterCredentialsSecret is a reference to the name of k8s Secret, defined within the same namespace as the Environment resource,
	// that contains a kubeconfig.
	// The Secret must be of type 'managed-gitops.redhat.com/managed-environment'
	//
	// See this temporary URL for details:
	// https://github.com/redhat-appstudio/managed-gitops/tree/main/examples/m6-demo#gitopsdeploymentmanagedenvironment-resource
	ClusterCredentialsSecret string `json:"clusterCredentialsSecret"`
}

// EnvironmentConfiguration contains Environment-specific configurations details, to be used when generating
// Component/Application GitOps repository resources.
type EnvironmentConfiguration struct {
	// Env is an array of standard environment vairables
	Env []EnvVarPair `json:"env"`
}

// EnvironmentStatus defines the observed state of Environment
type EnvironmentStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Environment is the Schema for the environments API
// +kubebuilder:resource:path=environments,shortName=env
type Environment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvironmentSpec   `json:"spec,omitempty"`
	Status EnvironmentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EnvironmentList contains a list of Environment
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Environment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Environment{}, &EnvironmentList{})
}
