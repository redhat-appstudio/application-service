/*
Copyright 2022 Red Hat, Inc.

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

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
)

func (r *ComponentDetectionQueryReconciler) SetDetectingConditionAndUpdateCR(ctx context.Context, req ctrl.Request, componentDetectionQuery *appstudiov1alpha1.ComponentDetectionQuery) {
	log := r.Log.WithValues("ComponentDetectionQuery", req.NamespacedName).WithValues("clusterName", req.ClusterName)

	meta.SetStatusCondition(&componentDetectionQuery.Status.Conditions, metav1.Condition{
		Type:    "Processing",
		Status:  metav1.ConditionTrue,
		Reason:  "Success",
		Message: "ComponentDetectionQuery is processing",
	})

	err := r.Client.Status().Update(ctx, componentDetectionQuery)
	if err != nil {
		log.Error(err, "Unable to update ComponentDetectionQuery")
	}
}

func (r *ComponentDetectionQueryReconciler) SetCompleteConditionAndUpdateCR(ctx context.Context, req ctrl.Request, componentDetectionQuery *appstudiov1alpha1.ComponentDetectionQuery, completeError error) {
	log := r.Log.WithValues("ComponentDetectionQuery", req.NamespacedName).WithValues("clusterName", req.ClusterName)

	if completeError == nil {
		meta.SetStatusCondition(&componentDetectionQuery.Status.Conditions, metav1.Condition{
			Type:    "Completed",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "ComponentDetectionQuery has successfully finished",
		})
	} else {
		meta.SetStatusCondition(&componentDetectionQuery.Status.Conditions, metav1.Condition{
			Type:    "Completed",
			Status:  metav1.ConditionFalse,
			Reason:  "Error",
			Message: fmt.Sprintf("ComponentDetectionQuery failed: %v", completeError),
		})
	}

	err := r.Client.Status().Update(ctx, componentDetectionQuery)
	if err != nil {
		log.Error(err, "Unable to update ComponentDetectionQuery")
	}
}
