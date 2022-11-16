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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SPIAccessCheckSpec defines the desired state of SPIAccessCheck
type SPIAccessCheckSpec struct {
	RepoUrl     string      `json:"repoUrl"`
	Permissions Permissions `json:"permissions,omitempty"`
}

// SPIAccessCheckStatus defines the observed state of SPIAccessCheck
type SPIAccessCheckStatus struct {
	Accessible      bool                        `json:"accessible"`
	Accessibility   SPIAccessCheckAccessibility `json:"accessibility"`
	Type            SPIRepoType                 `json:"repoType"`
	ServiceProvider ServiceProviderType         `json:"serviceProvider"`
	ErrorReason     SPIAccessCheckErrorReason   `json:"errorReason,omitempty"`
	ErrorMessage    string                      `json:"errorMessage,omitempty"`
}

type SPIRepoType string

const (
	SPIRepoTypeGit               SPIRepoType = "git"
	SPIRepoTypeContainerRegistry SPIRepoType = "containerRegistry"
)

type SPIAccessCheckErrorReason string

const (
	SPIAccessCheckErrorUnknownError           SPIAccessCheckErrorReason = "UnknownError"
	SPIAccessCheckErrorUnknownServiceProvider SPIAccessCheckErrorReason = "UnknownServiceProviderType"
	SPIAccessCheckErrorRepoNotFound           SPIAccessCheckErrorReason = "RepositoryNotFound"
	SPIAccessCheckErrorBadURL                 SPIAccessCheckErrorReason = "BadURL"
	SPIAccessCheckErrorNotImplemented         SPIAccessCheckErrorReason = "NotImplemented"
	SPIAccessCheckErrorTokenLookupFailed      SPIAccessCheckErrorReason = "TokenLookupFailure"
)

type SPIAccessCheckAccessibility string

const (
	SPIAccessCheckAccessibilityPublic  SPIAccessCheckAccessibility = "public"
	SPIAccessCheckAccessibilityPrivate SPIAccessCheckAccessibility = "private"
	SPIAccessCheckAccessibilityUnknown SPIAccessCheckAccessibility = "unknown"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SPIAccessCheck is the Schema for the spiaccesschecks API
type SPIAccessCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SPIAccessCheckSpec   `json:"spec,omitempty"`
	Status SPIAccessCheckStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SPIAccessCheckList contains a list of SPIAccessCheck
type SPIAccessCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SPIAccessCheck `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SPIAccessCheck{}, &SPIAccessCheckList{})
}

func (c *SPIAccessCheck) RepoUrl() string {
	return c.Spec.RepoUrl
}

func (c *SPIAccessCheck) ObjNamespace() string {
	return c.Namespace
}

func (c *SPIAccessCheck) Permissions() *Permissions {
	return &c.Spec.Permissions
}
