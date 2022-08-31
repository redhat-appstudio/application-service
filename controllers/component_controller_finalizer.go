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

package controllers

import (
	"context"
	"fmt"

	gitopsgen "github.com/redhat-developer/gitops-generator/pkg"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

const compFinalizerName = "component.appstudio.redhat.com/finalizer"

// AddFinalizer adds the finalizer to the Component CR
func (r *ComponentReconciler) AddFinalizer(ctx context.Context, component *appstudiov1alpha1.Component) error {
	controllerutil.AddFinalizer(component, compFinalizerName)
	return r.Update(ctx, component)
}

// Finalize deletes the corresponding devfile project or the devfile attribute entry from the Application CR and also deletes the corresponding GitOps repo's Component dir
// & updates the parent kustomize for the given Component CR.
func (r *ComponentReconciler) Finalize(ctx context.Context, component *appstudiov1alpha1.Component, application *appstudiov1alpha1.Application) error {
	// Get the Application CR devfile
	devfileObj, err := devfile.ParseDevfileModel(application.Status.Devfile)
	if err != nil {
		return err
	}

	if component.Spec.Source.GitSource != nil {
		err = devfileObj.DeleteProject(component.Spec.ComponentName)
		if err != nil {
			return err
		}
	} else if component.Spec.ContainerImage != "" {
		devSpec := devfileObj.GetDevfileWorkspaceSpec()
		if devSpec != nil {
			attributes := devSpec.Attributes
			delete(attributes, fmt.Sprintf("containerImage/%s", component.Spec.ComponentName))
			devSpec.Attributes = attributes
			devfileObj.SetDevfileWorkspaceSpec(*devSpec)
		}

	}

	yamldevfileObj, err := yaml.Marshal(devfileObj)
	if err != nil {
		return nil
	}

	application.Status.Devfile = string(yamldevfileObj)

	gitOpsURL, gitOpsBranch, gitOpsContext, err := util.ProcessGitOpsStatus(component.Status.GitOps, r.GitToken)
	if err != nil {
		return err
	}

	// Create a temp folder to create the gitops resources in
	tempDir, err := ioutils.CreateTempPath(component.Name, r.AppFS)
	if err != nil {
		return fmt.Errorf("unable to create temp directory for gitops resources due to error: %v", err)
	}

	err = gitopsgen.RemoveAndPush(tempDir, gitOpsURL, component.Name, r.Executor, r.AppFS, gitOpsBranch, gitOpsContext, true)
	if err != nil {
		gitOpsErr := util.SanitizeErrorMessage(err)
		return gitOpsErr
	}

	err = r.AppFS.RemoveAll(tempDir)
	if err != nil {
		return err
	}

	return r.Status().Update(ctx, application)
}
