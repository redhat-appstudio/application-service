/*
Copyright 2023 Red Hat, Inc.

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
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func TestSetCompleteConditionAndUpdateCR(t *testing.T) {
	// Set up a fake Kubernetes client and Component reconciler
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appstudiov1alpha1.AddToScheme(scheme))
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &ComponentDetectionQueryReconciler{
		Log:    ctrl.Log.WithName("controllers").WithName("Component"),
		Client: fakeClient,
	}

	originalCDQ := appstudiov1alpha1.ComponentDetectionQuery{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "ComponentDetectionQuery",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cdq",
			Namespace: "test-namespace",
		},
		Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
			GitSource: appstudiov1alpha1.GitSource{
				URL: "https://github.com/fakeorg/fakerepo",
			},
		},
	}
	r.Client.Create(context.Background(), &originalCDQ)

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "test-namespace",
			Name:      "test-cdq",
		},
	}

	tests := []struct {
		name          string
		updateCDQ     appstudiov1alpha1.ComponentDetectionQuery
		err           error
		wantCondition metav1.Condition
	}{
		{
			name:      "Simple CDQ, no error",
			updateCDQ: originalCDQ,
			wantCondition: metav1.Condition{
				Type:    "Completed",
				Status:  metav1.ConditionTrue,
				Reason:  "OK",
				Message: "ComponentDetectionQuery has successfully finished",
			},
		},
		{
			name:      "Simple CDQ, with error",
			updateCDQ: originalCDQ,
			err:       fmt.Errorf("some error"),
			wantCondition: metav1.Condition{
				Type:    "Completed",
				Status:  metav1.ConditionFalse,
				Reason:  "Error",
				Message: fmt.Sprintf("ComponentDetectionQuery failed: %v", fmt.Errorf("some error")),
			},
		},
		{
			name: "CDQ with invalid status fields",
			updateCDQ: appstudiov1alpha1.ComponentDetectionQuery{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ComponentDetectionQuery",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cdq",
					Namespace: "test-namespace",
				},
				Spec: appstudiov1alpha1.ComponentDetectionQuerySpec{
					GitSource: appstudiov1alpha1.GitSource{
						URL: "https://github.com/fakeorg/fakerepo",
					},
				},
				Status: appstudiov1alpha1.ComponentDetectionQueryStatus{
					ComponentDetected: appstudiov1alpha1.ComponentDetectionMap{
						"-ohu7": appstudiov1alpha1.ComponentDetectionDescription{
							ComponentStub: appstudiov1alpha1.ComponentSpec{
								ComponentName: "-ohu7",
							},
						},
					},
				},
			},
			err: fmt.Errorf("some error"),
			wantCondition: metav1.Condition{
				Type:    "Completed",
				Status:  metav1.ConditionFalse,
				Reason:  "Error",
				Message: fmt.Sprintf("ComponentDetectionQuery failed: %v", fmt.Errorf("some error")),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r.SetCompleteConditionAndUpdateCR(context.Background(), request, &tt.updateCDQ, tt.err)

			// Now get the resource and verify its status condition
			getCDQ := appstudiov1alpha1.ComponentDetectionQuery{}
			err := r.Client.Get(context.Background(), types.NamespacedName{Namespace: "test-namespace", Name: "test-cdq"}, &getCDQ)
			if err != nil {
				t.Errorf("TestSetCompleteConditionAndUpdateCR(): Unexpected error: %v", err)
			}

			if len(getCDQ.Status.Conditions) != 1 {
				t.Errorf("TestSetCompleteConditionAndUpdateCR(): Unexpected error, length of %v was %v, not 1", getCDQ.Status.Conditions, len(getCDQ.Status.Conditions))
			}
			tt.wantCondition.LastTransitionTime = getCDQ.Status.Conditions[0].LastTransitionTime
			tt.wantCondition.ObservedGeneration = getCDQ.Status.Conditions[0].ObservedGeneration
			if !reflect.DeepEqual(getCDQ.Status.Conditions[0], tt.wantCondition) {
				t.Errorf("TestSetCompleteConditionAndUpdateCR(): expected %v, got %v", tt.wantCondition, getCDQ.Status.Conditions[0])
			}
		})
	}
}
