/*
Copyright 2021-2023 Red Hat, Inc.

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

	"github.com/devfile/library/v2/pkg/devfile/parser"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	cdqanalysis "github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	github "github.com/redhat-appstudio/application-service/pkg/github"
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
func (r *ComponentReconciler) Finalize(ctx context.Context, component *appstudiov1alpha1.Component, application *appstudiov1alpha1.Application, ghClient *github.GitHubClient, token string) error {
	// Get the Application CR devfile
	devfileObj, err := cdqanalysis.ParseDevfileWithParserArgs(&parser.ParserArgs{Data: []byte(application.Status.Devfile), Token: token})

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

	err = r.Status().Update(ctx, application)
	if err != nil {
		if errors.IsConflict(err) {
			return err
		}
		log := ctrl.LoggerFrom(ctx)
		log.Error(err, "Failed to update application in finalizer, will not retry to prevent finalizer loop")
	}

	gitOpsURL, gitOpsBranch, gitOpsContext, err := util.ProcessGitOpsStatus(component.Status.GitOps, ghClient.Token)
	if err != nil {
		return err
	}

	// Create a temp folder to create the gitops resources in
	tempDir, err := ioutils.CreateTempPath(component.Name, r.AppFS)
	if err != nil {
		return fmt.Errorf("unable to create temp directory for gitops resources due to error: %v", err)
	}

	//Gitops functions return sanitized error messages
	err = r.Generator.GitRemoveComponent(tempDir, gitOpsURL, component.Name, gitOpsBranch, gitOpsContext)
	if err != nil {
		ioutils.RemoveFolderAndLogError(r.Log, r.AppFS, tempDir)
		return err
	}

	return r.AppFS.RemoveAll(tempDir)
}
