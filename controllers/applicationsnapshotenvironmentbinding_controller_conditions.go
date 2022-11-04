//
// Copyright 2022 Red Hat, Inc.
//
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

package controllers

import (
	"context"
	"fmt"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *SnapshotEnvironmentBindingReconciler) SetConditionAndUpdateCR(ctx context.Context, req ctrl.Request, appSnapshotEnvBinding *appstudiov1alpha1.SnapshotEnvironmentBinding, createError error) {
	log := r.Log.WithValues("SnapshotEnvironmentBinding", req.NamespacedName).WithValues("clusterName", req.ClusterName)

	if createError == nil {
		meta.SetStatusCondition(&appSnapshotEnvBinding.Status.GitOpsRepoConditions, metav1.Condition{
			Type:    "GitOpsResourcesGenerated",
			Status:  metav1.ConditionTrue,
			Reason:  "OK",
			Message: "GitOps repository sync successful",
		})
	} else {
		meta.SetStatusCondition(&appSnapshotEnvBinding.Status.GitOpsRepoConditions, metav1.Condition{
			Type:    "GitOpsResourcesGenerated",
			Status:  metav1.ConditionFalse,
			Reason:  "GenerateError",
			Message: fmt.Sprintf("GitOps repository sync failed: %v", createError),
		})
		logutil.LogAPIResourceChangeEvent(log, appSnapshotEnvBinding.Name, "SnapshotEnvironmentBinding", logutil.ResourceCreate, createError)
	}

	err := r.Client.Status().Update(ctx, appSnapshotEnvBinding)
	if err != nil {
		log.Error(err, "Unable to update application snapshot environment binding")
	}
}
