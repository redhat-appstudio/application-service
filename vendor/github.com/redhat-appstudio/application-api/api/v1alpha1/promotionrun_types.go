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

// PromotionRunSpec defines the desired state of PromotionRun
type PromotionRunSpec struct {

	// Snapshot refers to the name of a Snapshot resource defined within the namespace, used to promote container images between Environments.
	Snapshot string `json:"snapshot"`

	// Application is the name of an Application resource defined within the namespaced, and which is the target of the promotion
	Application string `json:"application"`

	// ManualPromotion is for fields specific to manual promotion.
	// Only one field should be defined: either 'manualPromotion' or 'automatedPromotion', but not both.
	ManualPromotion ManualPromotionConfiguration `json:"manualPromotion,omitempty"`

	// AutomatedPromotion is for fields specific to automated promotion
	// Only one field should be defined: either 'manualPromotion' or 'automatedPromotion', but not both.
	AutomatedPromotion AutomatedPromotionConfiguration `json:"automatedPromotion,omitempty"`
}

// ManualPromotionConfiguration defines promotion parameters specific to manual promotion: the target environment to promote to.
type ManualPromotionConfiguration struct {
	// TargetEnvironment is the environment to promote to
	TargetEnvironment string `json:"targetEnvironment"`
}

// AutomatedPromotionConfiguration defines promotion parameters specific to automated promotion: the initial environment
// (in the promotion graph) to begin promoting on.
type AutomatedPromotionConfiguration struct {
	// InitialEnvironment: start iterating through the digraph, beginning with the value specified in 'initialEnvironment'
	InitialEnvironment string `json:"initialEnvironment"`
}

// PromotionRunStatus defines the observed state of PromotionRun
type PromotionRunStatus struct {

	// State indicates whether or not the overall promotion (either manual or automated is complete)
	State PromotionRunState `json:"state"`

	// CompletionResult indicates success/failure once the promotion has completed all work.
	// CompletionResult will only have a value if State field is 'Complete'.
	CompletionResult PromotionRunCompleteResult `json:"completionResult,omitempty"`

	// EnvironmentStatus represents the set of steps taken during the  current promotion
	EnvironmentStatus []PromotionRunEnvironmentStatus `json:"environmentStatus,omitempty"`

	// ActiveBindings is the list of active bindings currently being promoted to:
	// - For an automated promotion, there can be multiple active bindings at a time (one for each env at a particular tree depth)
	// - For a manual promotion, there will be only one.
	ActiveBindings []string `json:"activeBindings,omitempty"`

	// PromotionStartTime is set to the value when the PromotionRun Reconciler first started the promotion.
	PromotionStartTime metav1.Time `json:"promotionStartTime,omitempty"`

	Conditions []PromotionRunCondition `json:"conditions,omitempty"`
}

// PromotionRunState defines the 3 states of an Promotion resource.
type PromotionRunState string

const (
	PromotionRunState_Active   PromotionRunState = "Active"
	PromotionRunState_Waiting  PromotionRunState = "Waiting"
	PromotionRunState_Complete PromotionRunState = "Complete"
)

// PromotionRunCompleteResult defines the success/failure states if the PromotionRunState is 'Complete'.
type PromotionRunCompleteResult string

const (
	PromotionRunCompleteResult_Success PromotionRunCompleteResult = "Success"
	PromotionRunCompleteResult_Failure PromotionRunCompleteResult = "Failure"
)

// PromotionRunEnvironmentStatus represents the set of steps taken during the  current promotion:
// - manual promotions will only have a single step.
// - automated promotions may have one or more steps, depending on how many environments have been promoted to.
type PromotionRunEnvironmentStatus struct {

	// Step is the sequential number of the step in the array, starting with 1
	Step int `json:"step"`

	// EnvironmentName is the name of the environment that was promoted to in this step
	EnvironmentName string `json:"environmentName"`

	// Status is/was the result of promoting to that environment.
	Status PromotionRunEnvironmentStatusField `json:"status"`

	// DisplayStatus is human-readible description of the current state/status.
	DisplayStatus string `json:"displayStatus"`
}

// PromotionRunEnvironmentStatusField are the state values for promotion to individual enviroments, as
// used by the Status field of PromotionRunEnvironmentStatus
type PromotionRunEnvironmentStatusField string

const (
	PromotionRunEnvironmentStatus_Success    PromotionRunEnvironmentStatusField = "Success"
	PromotionRunEnvironmentStatus_InProgress PromotionRunEnvironmentStatusField = "In Progress"
	PromotionRunEnvironmentStatus_Failed     PromotionRunEnvironmentStatusField = "Failed"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PromotionRun is the Schema for the promotionruns API
// +kubebuilder:resource:path=promotionruns,shortName=apr;promotion
type PromotionRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PromotionRunSpec   `json:"spec,omitempty"`
	Status PromotionRunStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PromotionRunList contains a list of PromotionRun
type PromotionRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PromotionRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PromotionRun{}, &PromotionRunList{})
}

// PromotionRunCondition contains details about an PromotionRun condition, which is usually an error or warning
type PromotionRunCondition struct {
	// Type is a PromotionRun condition type
	Type PromotionRunConditionType `json:"type"`

	// Message contains human-readable message indicating details about the last condition.
	// +optional
	Message string `json:"message"`

	// LastProbeTime is the last time the condition was observed.
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`

	// LastTransitionTime is the last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// Status is the status of the condition.
	Status PromotionRunConditionStatus `json:"status"`

	// Reason is a unique, one-word, CamelCase reason for the condition's last transition.
	// +optional
	Reason PromotionRunReasonType `json:"reason"`
}

// PromotionRunConditionType represents type of GitOpsDeployment condition.
type PromotionRunConditionType string

const (
	PromotionRunConditionErrorOccurred PromotionRunConditionType = "ErrorOccurred"
)

// PromotionRunConditionStatus is a type which represents possible comparison results
type PromotionRunConditionStatus string

// PromotionRun Condition Status
const (
	// PromotionRunConditionStatusTrue indicates that a condition type is true
	PromotionRunConditionStatusTrue PromotionRunConditionStatus = "True"
	// PromotionRunConditionStatusFalse indicates that a condition type is false
	PromotionRunConditionStatusFalse PromotionRunConditionStatus = "False"
	// PromotionRunConditionStatusUnknown indicates that the condition status could not be reliably determined
	PromotionRunConditionStatusUnknown PromotionRunConditionStatus = "Unknown"
)

type PromotionRunReasonType string

const (
	PromotionRunReasonErrorOccurred PromotionRunReasonType = "ErrorOccurred"
)
