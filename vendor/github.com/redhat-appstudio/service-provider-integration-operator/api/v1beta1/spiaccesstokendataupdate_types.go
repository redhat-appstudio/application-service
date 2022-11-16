//
// Copyright (c) 2021 Red Hat, Inc.
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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SPIAccessTokenDataUpdateSpec defines the desired state of SPIAccessTokenDataUpdate
type SPIAccessTokenDataUpdateSpec struct {
	// TokenName is the name of the SPIAccessToken object in the same namespace as the update object
	//+kubebuilder:validation:Required
	TokenName string `json:"tokenName"`
}

//+kubebuilder:object:root=true

// SPIAccessTokenDataUpdate is a special CRD that advertises to the controller in the Kubernetes cluster that there
// has been an update of the data in the token storage. Because token storage is out-of-cluster, updates to it are
// not registered by the controllers. This CRD serves as a "trigger" for reconciliation of the SPIAccessToken after
// the data has been updated in the token storage.
// The caller that updates the data in the token storage is responsible for creating an object pointing to the
// SPIAccessToken that should have been affected.
type SPIAccessTokenDataUpdate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SPIAccessTokenDataUpdateSpec `json:"spec"`
}

//+kubebuilder:object:root=true

// SPIAccessTokenDataUpdateList contains a list of SPIAccessTokenDataUpdate
type SPIAccessTokenDataUpdateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SPIAccessTokenDataUpdate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SPIAccessTokenDataUpdate{}, &SPIAccessTokenDataUpdateList{})
}
