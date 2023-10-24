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

package webhooks

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Webhook describes the data structure for the release webhook
type ApplicationWebhook struct {
	client client.Client
	log    logr.Logger
}

//+kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-application,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=applications,verbs=create;update,versions=v1alpha1,name=mapplication.kb.io,admissionReviewVersions=v1

func (w *ApplicationWebhook) Register(mgr ctrl.Manager, log *logr.Logger) error {
	w.client = mgr.GetClient()

	return ctrl.NewWebhookManagedBy(mgr).
		For(&appstudiov1alpha1.Application{}).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

// +kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-application,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=applications,verbs=create;update,versions=v1alpha1,name=vapplication.kb.io,admissionReviewVersions=v1

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ApplicationWebhook) Default(ctx context.Context, obj runtime.Object) error {

	// TODO(user): fill in your defaulting logic.
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ApplicationWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	app := obj.(*appstudiov1alpha1.Application)

	applicationlog := r.log.WithValues("controllerKind", "Application").WithValues("name", app.Name).WithValues("namespace", app.Namespace)
	applicationlog.Info("validating the create request")
	// We use the DNS-1035 format for application names, so ensure it conforms to that specification
	if len(validation.IsDNS1035Label(app.Name)) != 0 {
		return fmt.Errorf("invalid application name: %q: an application resource name must start with a lower case alphabetical character, be under 63 characters, and can only consist of lower case alphanumeric characters or ‘-’,", app.Name)
	}
	if app.Spec.DisplayName == "" {
		return fmt.Errorf("display name must be provided when creating an Application")
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ApplicationWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	oldApp := oldObj.(*appstudiov1alpha1.Application)
	newApp := newObj.(*appstudiov1alpha1.Application)
	applicationlog := r.log.WithValues("controllerKind", "Application").WithValues("name", newApp.Name).WithValues("namespace", newApp.Namespace)
	applicationlog.Info("validating the update request")

	if !reflect.DeepEqual(newApp.Spec.AppModelRepository, oldApp.Spec.AppModelRepository) {
		return fmt.Errorf("app model repository cannot be updated to %+v", newApp.Spec.AppModelRepository)
	}

	if !reflect.DeepEqual(newApp.Spec.GitOpsRepository, oldApp.Spec.GitOpsRepository) {
		return fmt.Errorf("gitops repository cannot be updated to %+v", newApp.Spec.GitOpsRepository)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ApplicationWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
