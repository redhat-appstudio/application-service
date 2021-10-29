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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HASApplicationSpec defines the desired state of HASApplication
type HASApplicationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of HASApplication. Edit hasapplication_types.go to remove/update
	//Foo string `json:"foo,omitempty"`

	// DisplayName refers to the name that an application will be deployed with in App Studio.
	DisplayName string `json:"displayName,omitempty"`

	// AppModelRepository refers to the git repository that will store the application model (a devfile)
	// Can be the same as GitOps repository.
	// A repository will be generated if this field is left blank.
	// +optional
	AppModelRepository HASApplicationGitRepository `json:"appModelRepository,omitempty"`

	// GitOpsRepository refers to the git repository that will store the gitops resources.
	// Can be the same as App Model Repository.
	// A repository will be generated if this field is left blank.
	// +optional
	GitOpsRepository HASApplicationGitRepository `json:"gitOpsRepository,omitempty"`

	// Description refers to a brief description of the application.
	Description string `json:"description,omitempty"`
}

// HASApplicationGitRepository defines a git repository for a given HASApplication resource (either appmodel or gitops)
type HASApplicationGitRepository struct {
	// URL refers to the repository URL that should be used.
	// +required
	URL string `json:"url"`

	// Branch corresponds to the branch in the repository that should be used
	// +optional
	Branch string `json:"branch,omitempty"`

	// Context corresponds to the context within the repository that should be used
	// +optional
	Context string `json:"context,omitempty"`
}

// HASApplicationStatus defines the observed state of HASApplication
type HASApplicationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []metav1.Condition `json:"conditions"`

	// Devfile corresponds to the devfile representation of the HASApplication resource
	Devfile string `json:"devfile,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HASApplication is the Schema for the hasapplications API
// +kubebuilder:resource:path=hasapplications,shortName=hasapp;ha
// +kubebuilder:subresource:status
type HASApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HASApplicationSpec   `json:"spec,omitempty"`
	Status HASApplicationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HASApplicationList contains a list of HASApplication
type HASApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HASApplication `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HASApplication{}, &HASApplicationList{})
}
