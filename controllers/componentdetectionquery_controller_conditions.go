/*
Copyright 2022-2023 Red Hat, Inc.

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

package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
)

func (r *ComponentDetectionQueryReconciler) SetDetectingConditionAndUpdateCR(ctx context.Context, req ctrl.Request, componentDetectionQuery *appstudiov1alpha1.ComponentDetectionQuery) {
	log := r.Log.WithValues("kind", "ComponentDetectionQuery").WithValues("resource", req.NamespacedName.Name).WithValues("namespace", req.NamespacedName.Namespace)

	patch := client.MergeFrom(componentDetectionQuery.DeepCopy())

	meta.SetStatusCondition(&componentDetectionQuery.Status.Conditions, metav1.Condition{
		Type:    "Processing",
		Status:  metav1.ConditionTrue,
		Reason:  "Success",
		Message: "ComponentDetectionQuery is processing",
	})

	err := r.Client.Status().Patch(ctx, componentDetectionQuery, patch)
	if err != nil {
		log.Error(err, "Unable to update ComponentDetectionQuery")
	}
}

func (r *ComponentDetectionQueryReconciler) SetCompleteConditionAndUpdateCR(ctx context.Context, req ctrl.Request, componentDetectionQuery *appstudiov1alpha1.ComponentDetectionQuery, originalCDQ *appstudiov1alpha1.ComponentDetectionQuery, completeError error) {
	log := r.Log.WithValues("namespace", req.NamespacedName.Namespace)

	patch := client.MergeFrom(originalCDQ.DeepCopy())

	if completeError == nil {
		meta.SetStatusCondition(&componentDetectionQuery.Status.Conditions, metav1.Condition{
			Type:    "Completed",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "ComponentDetectionQuery has successfully finished",
		})
		logutil.LogAPIResourceChangeEvent(log, componentDetectionQuery.Name, "ComponentDetectionQuery", logutil.ResourceComplete, nil)

	} else {
		meta.SetStatusCondition(&componentDetectionQuery.Status.Conditions, metav1.Condition{
			Type:    "Completed",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("ComponentDetectionQuery failed: %v", completeError),
		})
		logutil.LogAPIResourceChangeEvent(log, componentDetectionQuery.Name, "ComponentDetectionQuery", logutil.ResourceComplete, completeError)
	}
	err := r.Client.Status().Patch(ctx, componentDetectionQuery, patch)
	if err != nil {
		// Error attempting to update the CDQ status. Since some CDQ status fields have specific validation rules (specifically the detected components), a bug could cause
		// an invalid field to be present in the status. _If_ the status update fails, still attempt to update only the status conditions
		log.Error(err, "Unable to update ComponentDetectionQuery. Will attempt to update only the status condition")

		copiedCDQ := originalCDQ.DeepCopy()
		meta.SetStatusCondition(&copiedCDQ.Status.Conditions, metav1.Condition{
			Type:    "Completed",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("ComponentDetectionQuery failed: %v", completeError),
		})
		err := r.Client.Status().Patch(ctx, componentDetectionQuery, patch)
		if err != nil {
			log.Error(err, "Unable to update ComponentDetectionQuery status conditions")
		}

	}
}
