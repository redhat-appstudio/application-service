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

package webhooks

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// log is for logging in this package.
// Webhook describes the data structure for the release webhook
type ComponentWebhook struct {
	client client.Client
	log    logr.Logger
}

func (w *ComponentWebhook) Register(mgr ctrl.Manager, log *logr.Logger) error {
	w.client = mgr.GetClient()

	return ctrl.NewWebhookManagedBy(mgr).
		For(&appstudiov1alpha1.Component{}).
		WithValidator(w).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-component,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=components,verbs=create;update,versions=v1alpha1,name=mcomponent.kb.io,admissionReviewVersions=v1

// Github is the only supported vendor right now
const SupportedGitRepo = "github.com"

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-component,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=components,verbs=create;update,versions=v1alpha1,name=vcomponent.kb.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ComponentWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	comp := obj.(*appstudiov1alpha1.Component)
	componentlog := r.log.WithValues("controllerKind", "Component").WithValues("name", comp.Name).WithValues("namespace", comp.Namespace)
	componentlog.Info("validating the create request")

	// We use the DNS-1035 format for component names, so ensure it conforms to that specification
	if len(validation.IsDNS1035Label(comp.Name)) != 0 {
		return fmt.Errorf(appstudiov1alpha1.InvalidDNS1035Name, comp.Name)
	}
	sourceSpecified := false

	if comp.Spec.Source.GitSource != nil && comp.Spec.Source.GitSource.URL != "" {
		if gitsourceURL, err := url.ParseRequestURI(comp.Spec.Source.GitSource.URL); err != nil {
			return fmt.Errorf(err.Error() + appstudiov1alpha1.InvalidSchemeGitSourceURL)
		} else if SupportedGitRepo != strings.ToLower(gitsourceURL.Host) {
			return fmt.Errorf(appstudiov1alpha1.InvalidGithubVendorURL, gitsourceURL, SupportedGitRepo)
		}

		sourceSpecified = true
	} else if comp.Spec.ContainerImage != "" {
		sourceSpecified = true
	}

	if !sourceSpecified {
		return errors.New(appstudiov1alpha1.MissingGitOrImageSource)
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ComponentWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	oldComp := oldObj.(*appstudiov1alpha1.Component)
	newComp := newObj.(*appstudiov1alpha1.Component)

	componentlog := r.log.WithValues("controllerKind", "Component").WithValues("name", newComp.Name).WithValues("namespace", newComp.Namespace)
	componentlog.Info("validating the update request")

	if newComp.Spec.ComponentName != oldComp.Spec.ComponentName {
		return fmt.Errorf(appstudiov1alpha1.ComponentNameUpdateError, newComp.Spec.ComponentName)
	}

	if newComp.Spec.Application != oldComp.Spec.Application {
		return fmt.Errorf(appstudiov1alpha1.ApplicationNameUpdateError, newComp.Spec.Application)
	}

	if newComp.Spec.Source.GitSource != nil && oldComp.Spec.Source.GitSource != nil && (newComp.Spec.Source.GitSource.URL != oldComp.Spec.Source.GitSource.URL) {
		return fmt.Errorf(appstudiov1alpha1.GitSourceUpdateError, *(newComp.Spec.Source.GitSource))
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ComponentWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
