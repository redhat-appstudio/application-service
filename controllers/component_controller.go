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
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	devfileAPIV1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	devfilePkg "github.com/devfile/library/pkg/devfile"
	"github.com/devfile/library/pkg/devfile/parser"
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"
	"github.com/go-git/go-git/v5"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
)

const (
	devfile         = "devfile.yaml"
	clonePathPrefix = "/tmp/appstudio/has"
)

// ComponentReconciler reconciles a HASComponent object
type ComponentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
	logger := log.FromContext(ctx)
	// your logic here
	logger.Info("HELLO from the controller")

	// Fetch the HASComponent instance
	var hasComponent appstudiov1alpha1.HASComponent
	err := r.Get(ctx, req.NamespacedName, &hasComponent)
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
	if hasComponent.Status.Devfile == "" {
		source := hasComponent.Spec.Source
		context := hasComponent.Spec.Context
		var devfilePath string

		// append context to devfile if present
		// context is usually set when the git repo is a multi-component repo (example - contains both frontend & backend)
		if context == "" {
			devfilePath = devfile
		} else {
			devfilePath = path.Join(context, devfile)
		}

		logger.Info("calculated devfile path", "devfilePath", devfilePath)

		if source.GitSource.URL != "" {
			var devfileBytes []byte

			if source.GitSource.DevfileURL == "" {
				logger.Info("source.GitSource.URL", "source.GitSource.URL", source.GitSource.URL)
				rawURL, err := convertGitHubURL(source.GitSource.URL)
				if err != nil {
					return ctrl.Result{}, err
				}
				logger.Info("rawURL", "rawURL", rawURL)

				devfilePath = rawURL + "/" + devfilePath
				logger.Info("devfilePath", "devfilePath", devfilePath)
				resp, err := http.Get(devfilePath)
				if err != nil {
					return ctrl.Result{}, err
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					logger.Info("curl succesful")
					devfileBytes, err = ioutil.ReadAll(resp.Body)
					if err != nil {
						return ctrl.Result{}, err
					}
				} else {
					logger.Info("intializing cloning since unable to curl")
					clonePath := path.Join(clonePathPrefix, hasComponent.Spec.Application, hasComponent.Spec.ComponentName)

					// Check if the clone path is empty, if not delete it
					isDirExist, err := IsExist(clonePath)
					if err != nil {
						return ctrl.Result{}, err
					}
					if isDirExist {
						logger.Info("clone path exists, deleting", "path", clonePath)
						os.RemoveAll(clonePath)
					}

					// Clone the repo
					_, err = git.PlainClone(clonePath, false, &git.CloneOptions{
						URL: source.GitSource.URL,
					})
					if err != nil {
						return ctrl.Result{}, err
					}

					// Read the devfile
					devfileBytes, err = ioutil.ReadFile(path.Join(clonePath, devfilePath))
					if err != nil {
						return ctrl.Result{}, err
					}
				}
			} else {
				logger.Info("Getting devfile from the DevfileURL", "DevfileURL", source.GitSource.DevfileURL)
				resp, err := http.Get(source.GitSource.DevfileURL)
				if err != nil {
					return ctrl.Result{}, err
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					logger.Info("curl succesful")
					devfileBytes, err = ioutil.ReadAll(resp.Body)
					if err != nil {
						return ctrl.Result{}, err
					}
				} else {
					return ctrl.Result{}, fmt.Errorf("unable to GET from %s", source.GitSource.DevfileURL)
				}
			}

			logger.Info("successfully read the devfile", "string representation", string(devfileBytes[:]))

			devfileObj, _, err := devfilePkg.ParseDevfileAndValidate(parser.ParserArgs{
				Data: devfileBytes,
			})
			if err != nil {
				return ctrl.Result{}, err
			}

			components, err := devfileObj.Data.GetComponents(common.DevfileOptions{
				ComponentOptions: common.ComponentOptions{
					ComponentType: devfileAPIV1.ContainerComponentType,
				},
			})
			if err != nil {
				return ctrl.Result{}, err
			}
			logger.Info("components", "components", components)

			for _, component := range components {
				updateRequired := false
				if hasComponent.Spec.Route != "" {
					logger.Info("hasComponent.Spec.Route", "hasComponent.Spec.Route", hasComponent.Spec.Route)
					if len(component.Attributes) == 0 {
						logger.Info("init Attributes 1")
						component.Attributes = attributes.Attributes{}
					}
					logger.Info("len(component.Attributes) 111", "len(component.Attributes) 111", len(component.Attributes))
					component.Attributes = component.Attributes.PutString("appstudio/has.route", hasComponent.Spec.Route)
					updateRequired = true
				}
				if hasComponent.Spec.Replicas > 0 {
					logger.Info("hasComponent.Spec.Replicas", "hasComponent.Spec.Replicas", hasComponent.Spec.Replicas)
					if len(component.Attributes) == 0 {
						logger.Info("init Attributes 2")
						component.Attributes = attributes.Attributes{}
					}
					logger.Info("len(component.Attributes) 222", "len(component.Attributes) 222", len(component.Attributes))
					component.Attributes = component.Attributes.PutInteger("appstudio/has.replicas", hasComponent.Spec.Replicas)
					updateRequired = true
				}
				if hasComponent.Spec.TargetPort > 0 {
					logger.Info("hasComponent.Spec.TargetPort", "hasComponent.Spec.TargetPort", hasComponent.Spec.TargetPort)
					for i, endpoint := range component.Container.Endpoints {
						logger.Info("foudn endpoint", "endpoing", endpoint.Name)
						endpoint.TargetPort = hasComponent.Spec.TargetPort
						updateRequired = true
						component.Container.Endpoints[i] = endpoint
					}
				}

				if updateRequired {
					logger.Info("UPDATING COMPONENT", "component name", component.Container)
					// Update the component once it has been updated with the HAS Component data
					err := devfileObj.Data.UpdateComponent(component)
					if err != nil {
						return ctrl.Result{}, err
					}
				}
			}

			logger.Info("outside before getting NEW CONTENT")

			yamlData, err := yaml.Marshal(devfileObj.Data)
			if err != nil {
				return ctrl.Result{}, err
			}

			logger.Info("successfully UPDATED the devfile", "string representation", string(yamlData[:]))

			hasComponent.Status.Devfile = string(yamlData[:])
			err = r.Status().Update(ctx, &hasComponent)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.Component{}).
		Complete(r)
}

// IsExist returns whether the given file or directory exists
func IsExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// convertGitHubURL converts a git url to its raw format
// taken from Jingfu's odo code
func convertGitHubURL(URL string) (string, error) {
	url, err := url.Parse(URL)
	if err != nil {
		return "", err
	}

	if strings.Contains(url.Host, "github") && !strings.Contains(url.Host, "raw") {
		// Convert path part of the URL
		URLSlice := strings.Split(URL, "/")
		if len(URLSlice) > 2 && URLSlice[len(URLSlice)-2] == "tree" {
			// GitHub raw URL doesn't have "tree" structure in the URL, need to remove it
			URL = strings.Replace(URL, "/tree", "", 1)
		} else {
			// Add "main" branch for GitHub raw URL by default if branch is not specified
			URL = URL + "/main"
		}

		// Convert host part of the URL
		if url.Host == "github.com" {
			URL = strings.Replace(URL, "github.com", "raw.githubusercontent.com", 1)
		}
	}

	return URL, nil
}
