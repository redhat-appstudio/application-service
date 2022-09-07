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

	logicalcluster "github.com/kcp-dev/logicalcluster/v2"
	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// MapToBindingByBoundObjectName maps the bound object (Environment) to the associated Bindings.
// The correct Bindings are listed using the given label whose value should equal to the object's name.
// Adapted from https://github.com/codeready-toolchain/host-operator/blob/master/controllers/spacebindingcleanup/mapper.go#L17
func MapToBindingByBoundObjectName(cl client.Client, objectType, label string) func(object client.Object) []reconcile.Request {
	return func(obj client.Object) []reconcile.Request {
		// Retrieve the cluster name (if applicable)
		clusterName := logicalcluster.From(obj).String()

		mapperLog := ctrl.Log.WithName("MapToBindingByBoundObjectName")
		log := mapperLog.WithValues("object-name", obj.GetName(), "object-kind", obj.GetObjectKind()).WithValues("clusterName", clusterName)
		ctx := logicalcluster.WithCluster(context.TODO(), logicalcluster.New(clusterName))

		bindingList := &appstudioshared.ApplicationSnapshotEnvironmentBindingList{}
		err := cl.List(ctx, bindingList,
			client.InNamespace(obj.GetNamespace()),
			client.MatchingLabels{label: obj.GetName()})
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to list ApplicationSnapshotEnvironmentBinding for a %s object %s", objectType, obj.GetName()))
			return []reconcile.Request{}
		}
		if len(bindingList.Items) == 0 {
			log.Info(fmt.Sprintf("no ApplicationSnapshotEnvironmentBinding found for a %s object %s", objectType, obj.GetName()))
			return []reconcile.Request{}
		}

		log.Info(fmt.Sprintf("Found %d ApplicationSnapshotEnvironmentBindings for a %s object %s", len(bindingList.Items), objectType, obj.GetName()))

		req := make([]reconcile.Request, len(bindingList.Items))
		for i, item := range bindingList.Items {
			req[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: item.Namespace,
					Name:      item.Name,
				},
				ClusterName: clusterName,
			}
			log.Info(fmt.Sprintf("The corresponding ApplicationSnapshotEnvironmentBinding %s will be reconciled", item.Name))
		}
		return req
	}
}
