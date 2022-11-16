/*
Copyright 2021.

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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SPIAccessTokenBindingSpec defines the desired state of SPIAccessTokenBinding
type SPIAccessTokenBindingSpec struct {
	RepoUrl     string      `json:"repoUrl"`
	Permissions Permissions `json:"permissions,omitempty"`
	Secret      SecretSpec  `json:"secret"`
}

// SPIAccessTokenBindingStatus defines the observed state of SPIAccessTokenBinding
type SPIAccessTokenBindingStatus struct {
	Phase                 SPIAccessTokenBindingPhase       `json:"phase"`
	ErrorReason           SPIAccessTokenBindingErrorReason `json:"errorReason,omitempty"`
	ErrorMessage          string                           `json:"errorMessage,omitempty"`
	LinkedAccessTokenName string                           `json:"linkedAccessTokenName"`
	OAuthUrl              string                           `json:"oAuthUrl,omitempty"`
	SyncedObjectRef       TargetObjectRef                  `json:"syncedObjectRef"`
}

type SPIAccessTokenBindingPhase string

const (
	SPIAccessTokenBindingPhaseAwaitingTokenData SPIAccessTokenBindingPhase = "AwaitingTokenData"
	SPIAccessTokenBindingPhaseInjected          SPIAccessTokenBindingPhase = "Injected"
	SPIAccessTokenBindingPhaseError             SPIAccessTokenBindingPhase = "Error"
)

type SPIAccessTokenBindingErrorReason string

const (
	SPIAccessTokenBindingErrorReasonUnknownServiceProviderType SPIAccessTokenBindingErrorReason = "UnknownServiceProviderType"
	SPIAccessTokenBindingErrorReasonTokenLookup                SPIAccessTokenBindingErrorReason = "TokenLookup"
	SPIAccessTokenBindingErrorReasonLinkedToken                SPIAccessTokenBindingErrorReason = "LinkedToken"
	SPIAccessTokenBindingErrorReasonTokenRetrieval             SPIAccessTokenBindingErrorReason = "TokenRetrieval"
	SPIAccessTokenBindingErrorReasonTokenSync                  SPIAccessTokenBindingErrorReason = "TokenSync"
	SPIAccessTokenBindingErrorReasonTokenAnalysis              SPIAccessTokenBindingErrorReason = "TokenAnalysis"
	SPIAccessTokenBindingErrorReasonUnsupportedPermissions     SPIAccessTokenBindingErrorReason = "UnsupportedPermissions"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SPIAccessTokenBinding is the Schema for the spiaccesstokenbindings API
type SPIAccessTokenBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SPIAccessTokenBindingSpec   `json:"spec,omitempty"`
	Status SPIAccessTokenBindingStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SPIAccessTokenBindingList contains a list of SPIAccessTokenBinding
type SPIAccessTokenBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SPIAccessTokenBinding `json:"items"`
}

type SecretSpec struct {
	// Name is the name of the secret to be created. If it is not defined a random name based on the name of the binding
	// is used.
	// +optional
	Name string `json:"name,omitempty"`
	// Labels contains the labels that the created secret should be labeled with.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations is the keys and values that the create secret should be annotated with.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Type is the type of the secret to be created. If left empty, the default type used in the cluster is assumed (typically Opaque).
	// The type of the secret defines the automatic mapping of the token record fields to keys in the secret data
	// according to the documentation https://kubernetes.io/docs/concepts/configuration/secret/#secret-types.
	// Only kubernetes.io/service-account-token, kubernetes.io/dockercfg, kubernetes.io/dockerconfigjson and kubernetes.io/basic-auth
	// are supported. All other secret types need to have their mapping specified manually using the Fields.
	Type corev1.SecretType `json:"type,omitempty"`
	// Fields specifies the mapping from the token record fields to the keys in the secret data.
	Fields TokenFieldMapping `json:"fields,omitempty"`
}

type TokenFieldMapping struct {
	// Token specifies the data key in which the token should be stored.
	Token string `json:"token,omitempty"`
	// Name specifies the data key in which the name of the token record should be stored.
	Name string `json:"name,omitempty"`
	// ServiceProviderUrl specifies the data key in which the url of the service provider should be stored.
	ServiceProviderUrl string `json:"serviceProviderUrl,omitempty"`
	// ServiceProviderUserName specifies the data key in which the url of the user name used in the service provider should be stored.
	ServiceProviderUserName string `json:"serviceProviderUserName,omitempty"`
	// ServiceProviderUserId specifies the data key in which the url of the user id used in the service provider should be stored.
	ServiceProviderUserId string `json:"serviceProviderUserId,omitempty"`
	// UserId specifies the data key in which the user id as known to the SPI should be stored (note that this is usually different from
	// ServiceProviderUserId, because the former is usually a kubernetes user, while the latter is some arbitrary ID used by the service provider
	// which might or might not correspond to the Kubernetes user id).
	UserId string `json:"userId,omitempty"`
	// ExpiredAfter specifies the data key in which the expiry date of the token should be stored.
	ExpiredAfter string `json:"expiredAfter,omitempty"`
	// Scopes specifies the data key in which the comma-separated list of token scopes should be stored.
	Scopes string `json:"scopes,omitempty"`
}

type TargetObjectRef struct {
	// Name is the name of the object with the injected data. This always lives in the same namespace as the AccessTokenSecret object.
	Name string `json:"name"`
	// Kind is the kind of the object with the injected data.
	Kind string `json:"kind"`
	// ApiVersion is the api version of the object with the injected data.
	ApiVersion string `json:"apiVersion"`
}

func init() {
	SchemeBuilder.Register(&SPIAccessTokenBinding{}, &SPIAccessTokenBindingList{})
}

func (in *SPIAccessTokenBinding) RepoUrl() string {
	return in.Spec.RepoUrl
}

func (in *SPIAccessTokenBinding) ObjNamespace() string {
	return in.Namespace
}

func (in *SPIAccessTokenBinding) Permissions() *Permissions {
	return &in.Spec.Permissions
}
