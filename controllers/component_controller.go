/*
Copyright 2021-2022 Red Hat, Inc.

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
	"net/url"
	"os"
	"path"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/devfile/api/v2/pkg/attributes"
	data "github.com/devfile/library/pkg/devfile/parser/data"
	"github.com/go-logr/logr"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	util "github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/spf13/afero"
)

// ComponentReconciler reconciles a Component object
type ComponentReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Log       logr.Logger
	GitToken  string
	GitHubOrg string
	Executor  gitops.Executor
	AppFS     afero.Afero
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=components,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=components/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=components/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Component object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("Component", req.NamespacedName)
	log.Info(fmt.Sprintf("Starting reconcile loop for %v", req.NamespacedName))

	// Fetch the Component instance
	var component appstudiov1alpha1.Component
	err := r.Get(ctx, req.NamespacedName, &component)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// If the devfile hasn't been populated, the CR was just created
	if component.Status.Devfile == "" {
		source := component.Spec.Source
		context := component.Spec.Context

		if source.GitSource != nil && source.GitSource.URL != "" {
			var devfileBytes []byte

			if source.GitSource.DevfileURL == "" {
				rawURL, err := util.ConvertGitHubURL(source.GitSource.URL)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to convert Github URL to raw format, exiting reconcile loop %v", req.NamespacedName))
					r.SetCreateConditionAndUpdateCR(ctx, &component, err)
					return ctrl.Result{}, err
				}

				// append context to the path if present
				// context is usually set when the git repo is a multi-component repo (example - contains both frontend & backend)
				var devfileDir string
				if context == "" {
					devfileDir = rawURL
				} else {
					devfileDir = path.Join(rawURL, context)
				}

				devfileBytes, err = util.DownloadDevfile(devfileDir)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to read the devfile from dir %s %v", devfileDir, req.NamespacedName))
					r.SetCreateConditionAndUpdateCR(ctx, &component, err)
					return ctrl.Result{}, err
				}
			} else {
				devfileBytes, err = util.CurlEndpoint(source.GitSource.DevfileURL)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to GET %s, exiting reconcile loop %v", source.GitSource.DevfileURL, req.NamespacedName))
					err := fmt.Errorf("unable to GET from %s", source.GitSource.DevfileURL)
					r.SetCreateConditionAndUpdateCR(ctx, &component, err)
					return ctrl.Result{}, err
				}
			}

			// Parse the Component Devfile
			hasCompDevfileData, err := devfile.ParseDevfileModel(string(devfileBytes))
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component, exiting reconcile loop %v", req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, &component, err)
				return ctrl.Result{}, err
			}

			err = r.updateComponentDevfileModel(hasCompDevfileData, component)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to update the Component Devfile model %v", req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, &component, err)
				return ctrl.Result{}, nil
			}

			// Get the Application CR
			hasApplication := appstudiov1alpha1.Application{}
			err = r.Get(ctx, types.NamespacedName{Name: component.Spec.Application, Namespace: component.Namespace}, &hasApplication)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to get the Application %s, exiting reconcile loop %v", component.Spec.Application, req.NamespacedName))
				r.SetCreateConditionAndUpdateCR(ctx, &component, err)
				return ctrl.Result{}, nil
			}
			if hasApplication.Status.Devfile != "" {
				// Get the devfile of the hasApp CR
				hasAppDevfileData, err := devfile.ParseDevfileModel(hasApplication.Status.Devfile)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to parse the devfile from Application, exiting reconcile loop %v", req.NamespacedName))
					r.SetCreateConditionAndUpdateCR(ctx, &component, err)
					return ctrl.Result{}, err
				}

				err = r.updateApplicationDevfileModel(hasAppDevfileData, component)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to update the HAS Application Devfile model %v", req.NamespacedName))
					r.SetCreateConditionAndUpdateCR(ctx, &component, err)
					return ctrl.Result{}, nil
				}

				yamlHASCompData, err := yaml.Marshal(hasCompDevfileData)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to marshall the Component devfile, exiting reconcile loop %v", req.NamespacedName))
					r.SetCreateConditionAndUpdateCR(ctx, &component, err)
					return ctrl.Result{}, err
				}

				component.Status.Devfile = string(yamlHASCompData)

				// Update the HASApp CR with the new devfile
				yamlHASAppData, err := yaml.Marshal(hasAppDevfileData)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to marshall the Application devfile, exiting reconcile loop %v", req.NamespacedName))
					r.SetCreateConditionAndUpdateCR(ctx, &component, err)
					return ctrl.Result{}, err
				}
				hasApplication.Status.Devfile = string(yamlHASAppData)
				err = r.Status().Update(ctx, &hasApplication)
				if err != nil {
					log.Error(err, "Unable to update Application")
					// if we're unable to update the Application CR, then  we need to err out
					// since we need to save a reference of the Component in Application
					r.SetCreateConditionAndUpdateCR(ctx, &component, err)
					return ctrl.Result{}, err
				}

				if component.Spec.Build.ContainerImage != "" {
					// Set the container image in the status
					component.Status.ContainerImage = component.Spec.Build.ContainerImage
				}

				log.Info(fmt.Sprintf("Updating the labels for Component %v", req.NamespacedName))
				componentLabels := make(map[string]string)
				componentLabels[applicationKey] = component.Spec.Application
				componentLabels[componentKey] = component.Spec.ComponentName
				component.SetLabels(componentLabels)
				err = setGitopsAnnotations(&component, hasAppDevfileData)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to retrieve gitops repository information for resource %v", req.NamespacedName))
					r.SetCreateConditionAndUpdateCR(ctx, &component, err)
					return ctrl.Result{}, err
				}
				err = r.Client.Update(ctx, &component)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to update Component with the required labels %v", req.NamespacedName))
					r.SetCreateConditionAndUpdateCR(ctx, &component, err)
					return ctrl.Result{}, err
				}

				// Generate and push the gitops resources if spec.containerImage is set
				if component.Spec.Build.ContainerImage != "" {
					err = r.generateGitops(&component)
					if err != nil {
						errMsg := fmt.Sprintf("Unable to generate gitops resources for component %v", req.NamespacedName)
						log.Error(err, errMsg)
						r.SetCreateConditionAndUpdateCR(ctx, &component, fmt.Errorf(errMsg))
						return ctrl.Result{}, err
					}
				}

				r.SetCreateConditionAndUpdateCR(ctx, &component, nil)

			} else {
				log.Error(err, fmt.Sprintf("Application devfile model is empty. Before creating a Component, an instance of Application should be created, exiting reconcile loop %v", req.NamespacedName))
				err := fmt.Errorf("application devfile model is empty. Before creating a Component, an instance of Application should be created")
				r.SetCreateConditionAndUpdateCR(ctx, &component, err)
				return ctrl.Result{}, err
			}

		} else if source.ImageSource != nil && source.ImageSource.ContainerImage != "" {
			log.Info(fmt.Sprintf("container image is not supported at the moment, please use github links for adding a component to an application for %v", req.NamespacedName))
		}
	} else {
		// If the model already exists, see if fields have been updated
		log.Info(fmt.Sprintf("Checking if the Component has been updated %v", req.NamespacedName))

		// Parse the Component Devfile
		hasCompDevfileData, err := devfile.ParseDevfileModel(component.Status.Devfile)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component, exiting reconcile loop %v", req.NamespacedName))
			r.SetUpdateConditionAndUpdateCR(ctx, &component, err)
			return ctrl.Result{}, err
		}

		err = r.updateComponentDevfileModel(hasCompDevfileData, component)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to update the Component Devfile model %v", req.NamespacedName))
			r.SetUpdateConditionAndUpdateCR(ctx, &component, err)
			return ctrl.Result{}, err
		}

		// Read the devfile again to compare it with any updates
		oldCompDevfileData, err := devfile.ParseDevfileModel(component.Status.Devfile)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to parse the devfile from Component, exiting reconcile loop %v", req.NamespacedName))
			r.SetUpdateConditionAndUpdateCR(ctx, &component, err)
			return ctrl.Result{}, err
		}

		isUpdated := !reflect.DeepEqual(oldCompDevfileData, hasCompDevfileData) || component.Spec.Build.ContainerImage != component.Status.ContainerImage
		if isUpdated {
			log.Info(fmt.Sprintf("The Component was updated %v", req.NamespacedName))
			yamlHASCompData, err := yaml.Marshal(hasCompDevfileData)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to marshall the Component devfile, exiting reconcile loop %v", req.NamespacedName))
				r.SetUpdateConditionAndUpdateCR(ctx, &component, err)
				return ctrl.Result{}, err
			}

			// Generate and push the gitops resources if spec.containerImage is set
			if component.Spec.Build.ContainerImage != "" {
				component.Status.ContainerImage = component.Spec.Build.ContainerImage
				err = r.generateGitops(&component)
				if err != nil {
					errMsg := fmt.Sprintf("Unable to generate gitops resources for component %v", req.NamespacedName)
					log.Error(err, errMsg)
					r.SetUpdateConditionAndUpdateCR(ctx, &component, fmt.Errorf("%v: %v", errMsg, err))
					return ctrl.Result{}, err
				}
			}

			component.Status.Devfile = string(yamlHASCompData)
			r.SetUpdateConditionAndUpdateCR(ctx, &component, nil)
		} else {
			log.Info(fmt.Sprintf("The Component devfile data was not updated %v", req.NamespacedName))
		}
	}

	if component.Spec.Build.ContainerImage == "" {
		component.Spec.Build.ContainerImage = "quay.io/redhat-appstudio/user-workload:" + component.Namespace + "-" + component.Name
	}

	if component.Spec.Build.ContainerImage != "" {

		// TODO: would move this to the user's gitops repository under /.tekton
		webhookURL, err := r.setupWebhookTriggeredImageBuilds(ctx, log, component)
		if err != nil {
			log.Error(err, "Unable to setup builds")
		}
		component.Status.Webhook = webhookURL
		err = r.Client.Status().Update(ctx, &component)
		if err != nil {
			log.Error(err, "Unable to update Component with webhook URL")
		}
	}

	log.Info(fmt.Sprintf("Finished reconcile loop for %v", req.NamespacedName))
	return ctrl.Result{}, err
}

// generateGitops retrieves the necessary information about a Component's gitops repository (URL, branch, context)
// and attempts to use the GitOps package to generate gitops resources based on that component
func (r *ComponentReconciler) generateGitops(component *appstudiov1alpha1.Component) error {
	log := r.Log.WithValues("Component", component.Name)

	componentAnnotations := component.GetAnnotations()
	if componentAnnotations == nil {
		return fmt.Errorf("unable to create gitops resource, component gitops annotations are not set")
	}

	// Get the information about the gitops repository from the Component resource
	var gitOpsURL, gitOpsBranch, gitOpsContext string
	gitOpsURL = componentAnnotations["gitOpsRepository.url"]
	if gitOpsURL == "" {
		err := fmt.Errorf("unable to create gitops resource, gitOpsRepository.url annotation not set on component")
		log.Error(err, "")
		return err
	}
	if componentAnnotations["gitOpsRepository.branch"] != "" {
		gitOpsBranch = componentAnnotations["gitOpsRepository.branch"]
	} else {
		gitOpsBranch = "main"
	}
	if componentAnnotations["gitOpsRepository.context"] != "" {
		gitOpsContext = componentAnnotations["gitOpsRepository.context"]
	} else {
		gitOpsContext = "/"
	}

	// Construct the remote URL for the gitops repository
	parsedURL, err := url.Parse(gitOpsURL)
	if err != nil {
		log.Error(err, "unable to parse gitops URL due to error")
		return err
	}
	parsedURL.User = url.User(r.GitToken)
	remoteURL := parsedURL.String()

	// Create a temp folder to create the gitops resources in
	tempDir, err := r.AppFS.TempDir(os.TempDir(), component.Name)
	if err != nil {
		log.Error(err, "unable to create temp directory for gitops resources due to error")
		return fmt.Errorf("unable to create temp directory for gitops resources due to error: %v", err)
	}

	// Generate and push the gitops resources
	err = gitops.GenerateAndPush(tempDir, remoteURL, *component, r.Executor, r.AppFS, gitOpsBranch, gitOpsContext)
	if err != nil {
		log.Error(err, "unable to generate gitops resources due to error")
		return err
	}

	// Remove the temp folder that was created
	return r.AppFS.RemoveAll(tempDir)
}

// setGitopsAnnotations adds the necessary gitops annotations (url, branch, context) to the component CR object
func setGitopsAnnotations(component *appstudiov1alpha1.Component, devfileData data.DevfileData) error {
	var err error
	devfileAttributes := devfileData.GetMetadata().Attributes
	componentAnnotations := component.GetAnnotations()
	if componentAnnotations == nil {
		componentAnnotations = make(map[string]string)
	}
	// Get the GitOps repository URL
	gitOpsURL := devfileAttributes.GetString("gitOpsRepository.url", &err)
	if err != nil {
		return fmt.Errorf("unable to retrieve GitOps repository from Application CR devfile: %v", err)
	}
	componentAnnotations["gitOpsRepository.url"] = gitOpsURL

	// Get the GitOps repository branch
	gitOpsBranch := devfileAttributes.GetString("gitOpsRepository.branch", &err)
	if err != nil {
		if _, ok := err.(*attributes.KeyNotFoundError); !ok {
			return err
		}
	}
	if gitOpsBranch != "" {
		componentAnnotations["gitOpsRepository.branch"] = gitOpsBranch
	}

	// Get the GitOps repository context
	gitOpsContext := devfileAttributes.GetString("gitOpsRepository.context", &err)
	if err != nil {
		if _, ok := err.(*attributes.KeyNotFoundError); !ok {
			return err
		}
	}
	if gitOpsContext != "" {
		componentAnnotations["gitOpsRepository.context"] = gitOpsContext
	}

	component.SetAnnotations(componentAnnotations)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.Component{}).
		Complete(r)
}
