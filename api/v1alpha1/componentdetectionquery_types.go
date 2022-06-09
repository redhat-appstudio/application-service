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

// ComponentDetectionQuerySpec defines the desired state of ComponentDetectionQuery
type ComponentDetectionQuerySpec struct {

	// Git Source for a Component
	GitSource GitSource `json:"git"`

	// Secret describes the name of a Kubernetes secret containing a Personal Access Token to access the git repostiory
	Secret string `json:"secret,omitempty"`
}

// ComponentDetectionDescription holds all the information about the component being detected
type ComponentDetectionDescription struct {

	// DevfileFound tells if a devfile is found in the component
	DevfileFound bool `json:"devfileFound,omitempty"`

	// Language specifies the language of the component detected
	Language string `json:"language,omitempty"`

	// ProjectType specifies the type of project for the component detected
	ProjectType string `json:"projectType,omitempty"`

	// ComponentStub is a stub of the component detected with all the info gathered from the devfile or service detection
	ComponentStub ComponentSpec `json:"componentStub,omitempty"`
}

// ComponentDetectionMap is a map containing all the components and their detected information
type ComponentDetectionMap map[string]ComponentDetectionDescription

// ComponentDetectionQueryStatus defines the observed state of ComponentDetectionQuery
type ComponentDetectionQueryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Condition about the Component CR
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ComponentDetected gives a list of components and the info from detection
	ComponentDetected ComponentDetectionMap `json:"componentDetected,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ComponentDetectionQuery is the Schema for the componentdetectionqueries API
// +kubebuilder:resource:path=componentdetectionqueries,shortName=hcdq;compdetection
type ComponentDetectionQuery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentDetectionQuerySpec   `json:"spec,omitempty"`
	Status ComponentDetectionQueryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentDetectionQueryList contains a list of ComponentDetectionQuery
type ComponentDetectionQueryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentDetectionQuery `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentDetectionQuery{}, &ComponentDetectionQueryList{})
}
