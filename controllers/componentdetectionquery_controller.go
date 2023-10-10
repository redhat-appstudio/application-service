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
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	cdqanalysis "github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	"github.com/redhat-appstudio/application-service/pkg/github"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
	"github.com/redhat-appstudio/application-service/pkg/metrics"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/spf13/afero"
	"golang.org/x/exp/maps"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ComponentDetectionQueryReconciler reconciles a ComponentDetectionQuery object
type ComponentDetectionQueryReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	Log                logr.Logger
	GitHubTokenClient  github.GitHubToken
	DevfileRegistryURL string
	AppFS              afero.Afero
	RunKubernetesJob   bool
	Config             *rest.Config
	CdqAnalysisImage   string
}

const cdqName = "ComponentDetectionQuery"

// CDQReconcileTimeout is the default timeout, 5 mins, for the context of cdq reconcile
const CDQReconcileTimeout = 5 * time.Minute

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=componentdetectionqueries,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=componentdetectionqueries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=componentdetectionqueries/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ComponentDetectionQuery object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ComponentDetectionQueryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// set 5 mins timeout for cdq reconcile
	ctx, cancel := context.WithTimeout(ctx, CDQReconcileTimeout)
	defer cancel()

	// Fetch the ComponentDetectionQuery instance
	var componentDetectionQuery appstudiov1alpha1.ComponentDetectionQuery
	err := r.Get(ctx, req.NamespacedName, &componentDetectionQuery)
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

	// If there are no conditions attached to the CDQ, the resource was just created
	if len(componentDetectionQuery.Status.Conditions) == 0 {
		// Start the ComponentDetectionQuery, and update its status condition accordingly
		log.Info(fmt.Sprintf("Checking to see if a component can be detected %v", req.NamespacedName))
		r.SetDetectingConditionAndUpdateCR(ctx, req, &componentDetectionQuery)

		// Create a copy of the CDQ, to use as the base when setting the CDQ status via mergepatch later
		copiedCDQ := componentDetectionQuery.DeepCopy()

		var gitToken string
		if componentDetectionQuery.Spec.Secret != "" {
			gitSecret := corev1.Secret{}
			namespacedName := types.NamespacedName{
				Name:      componentDetectionQuery.Spec.Secret,
				Namespace: componentDetectionQuery.Namespace,
			}
			err = r.Client.Get(ctx, namespacedName, &gitSecret)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to retrieve Git secret %v, exiting reconcile loop %v", componentDetectionQuery.Spec.Secret, req.NamespacedName))
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
			gitToken = string(gitSecret.Data["password"])
		}

		// Create a Go-GitHub client for checking the default branch
		ghClient, err := r.GitHubTokenClient.GetNewGitHubClient(gitToken)
		if err != nil {
			log.Error(err, "Unable to create Go-GitHub client due to error")
			return reconcile.Result{}, err
		}

		// Add the Go-GitHub client name to the context
		ctx = context.WithValue(ctx, github.GHClientKey, ghClient.TokenName)

		source := componentDetectionQuery.Spec.GitSource
		var devfilePath string
		devfilesMap := make(map[string][]byte)
		devfilesURLMap := make(map[string]string)
		dockerfileContextMap := make(map[string]string)
		componentPortsMap := make(map[string][]int)
		context := source.Context

		if context == "" {
			context = "./"
		}
		// remove leading and trailing spaces of the repo URL
		source.URL = strings.TrimSpace(source.URL)
		sourceURL := source.URL
		// If the repository URL ends in a forward slash, remove it to avoid issues with default branch lookup
		if string(sourceURL[len(sourceURL)-1]) == "/" {
			sourceURL = sourceURL[0 : len(sourceURL)-1]
		}
		err = util.ValidateEndpoint(sourceURL) // does this work without token?
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to validate the source URL %v... %v", source.URL, req.NamespacedName))
			r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
			return ctrl.Result{}, nil
		}

		cdqInfo := &cdqanalysis.CDQInfoClient{
			DevfileRegistryURL: r.DevfileRegistryURL,
			GitURL:             cdqanalysis.GitURL{RepoURL: source.URL, Revision: source.Revision, Token: gitToken},
		}

		if source.DevfileURL == "" {
			log.Info(fmt.Sprintf("Attempting to read a devfile from the URL %s... %v", source.URL, req.NamespacedName))
			metrics.ImportGitRepoTotalReqs.Inc()

			var devfilesMapReturned map[string][]byte
			var devfilesURLMapReturned, dockerfileContextMapReturned map[string]string
			var componentPortsMapReturned map[string][]int
			revision := source.Revision

			// with annotation runCDQAnalysisLocal = true, would allow the CDQ controller to run the cdq-analysis go modoule
			// it is being used for CDQ controller tests to test both k8s job and go module
			if r.RunKubernetesJob && !(componentDetectionQuery.Annotations["runCDQAnalysisLocal"] == "true") {
				// perfume cdq job that requires repo cloning and azlier analysis
				clientset, err := kubernetes.NewForConfig(r.Config)
				if err != nil {
					log.Error(err, fmt.Sprintf("Error creating clientset with config... %v", req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
					return ctrl.Result{}, nil
				}
				jobName := req.Name + "-job"
				var backOffLimit int32 = 0
				jobSpec := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      jobName,
						Namespace: req.Namespace,
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								ServiceAccountName: "application-service-controller-manager",
								Containers: []corev1.Container{
									{
										Name:            jobName,
										Image:           r.CdqAnalysisImage,
										ImagePullPolicy: corev1.PullAlways,
										Env: []corev1.EnvVar{
											{
												Name:  "NAME",
												Value: req.Name,
											},
											{
												Name:  "NAMESPACE",
												Value: req.Namespace,
											},
											{
												Name:  "GITHUB_TOKEN",
												Value: gitToken,
											},
											{
												Name:  "CONTEXT_PATH",
												Value: context,
											},
											{
												Name:  "REVISION",
												Value: revision,
											},
											{
												Name:  "URL",
												Value: source.URL,
											},
											{
												Name:  "DEVFILE_REGISTRY_URL",
												Value: r.DevfileRegistryURL,
											},
											{
												Name:  "CREATE_K8S_Job",
												Value: "true",
											},
										},
									},
								},
								RestartPolicy: corev1.RestartPolicyNever,
							},
						},
						BackoffLimit: &backOffLimit,
					},
				}
				err = r.Client.Create(ctx, jobSpec, &client.CreateOptions{})
				if err != nil {
					log.Error(err, fmt.Sprintf("Error creating cdq analysis job %s... %v", jobName, req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
					return ctrl.Result{}, nil
				} else {
					//print job details
					log.Info(fmt.Sprintf("Successfully created cdq analysis job %v, waiting for config map to be created... %v", jobName, req.NamespacedName))
				}

				cm, err := waitForConfigMap(clientset, ctx, req.Name, req.Namespace)
				if err != nil {
					log.Error(err, fmt.Sprintf("Error waiting for configmap creation ...%v", req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
					cleanupK8sResources(log, clientset, ctx, fmt.Sprintf("%s-job", req.Name), req.Name, req.Namespace)
					return ctrl.Result{}, nil
				}
				var errMapReturned map[string]string
				var unmarshalErr error
				err = json.Unmarshal(cm.BinaryData["devfilesMap"], &devfilesMapReturned)
				if err != nil {
					unmarshalErr = multierror.Append(unmarshalErr, fmt.Errorf("unmarshal devfilesMap: %v", err))
				}
				err = json.Unmarshal(cm.BinaryData["dockerfileContextMap"], &dockerfileContextMapReturned)
				if err != nil {
					unmarshalErr = multierror.Append(unmarshalErr, fmt.Errorf("unmarshal dockerfileContextMap: %v", err))
				}
				err = json.Unmarshal(cm.BinaryData["devfilesURLMap"], &devfilesURLMapReturned)
				if err != nil {
					unmarshalErr = multierror.Append(unmarshalErr, fmt.Errorf("unmarshal devfilesURLMap: %v", err))
				}
				err = json.Unmarshal(cm.BinaryData["componentPortsMap"], &componentPortsMapReturned)
				if err != nil {
					unmarshalErr = multierror.Append(unmarshalErr, fmt.Errorf("unmarshal componentPortsMap: %v", err))
				}
				err = json.Unmarshal(cm.BinaryData["revision"], &revision)
				if err != nil {
					unmarshalErr = multierror.Append(unmarshalErr, fmt.Errorf("unmarshal revision: %v", err))
				}
				err = json.Unmarshal(cm.BinaryData["errorMap"], &errMapReturned)
				if err != nil {
					unmarshalErr = multierror.Append(unmarshalErr, fmt.Errorf("unmarshal errorMap: %v", err))
				}
				cleanupK8sResources(log, clientset, ctx, fmt.Sprintf("%s-job", req.Name), req.Name, req.Namespace)

				if unmarshalErr != nil {
					log.Error(unmarshalErr, fmt.Sprintf("Failed to unmarshal the returned result from CDQ configmap... %v", req.NamespacedName))
				}

				if errMapReturned != nil && !reflect.DeepEqual(errMapReturned, map[string]string{}) {
					var retErr error
					// only 1 index in the error map
					for key, value := range errMapReturned {
						if key == "NoDevfileFound" {
							metrics.ImportGitRepoSucceeded.Inc()
							retErr = &cdqanalysis.NoDevfileFound{Err: fmt.Errorf(value)}
						} else if key == "NoDockerfileFound" {
							metrics.ImportGitRepoSucceeded.Inc()
							retErr = &cdqanalysis.NoDockerfileFound{Err: fmt.Errorf(value)}
						} else if key == "RepoNotFound" {
							metrics.ImportGitRepoSucceeded.Inc()
							retErr = &cdqanalysis.RepoNotFound{Err: fmt.Errorf(value)}
						} else if key == "InvalidDevfile" {
							metrics.ImportGitRepoSucceeded.Inc()
							retErr = &cdqanalysis.InvalidDevfile{Err: fmt.Errorf(value)}
						} else if key == "InvalidURL" {
							metrics.ImportGitRepoSucceeded.Inc()
							retErr = &cdqanalysis.InvalidURL{Err: fmt.Errorf(value)}
						} else if key == "AuthenticationFailed" {
							metrics.ImportGitRepoSucceeded.Inc()
							retErr = &cdqanalysis.AuthenticationFailed{Err: fmt.Errorf(value)}
						} else {
							// Increment the git import failure metric
							metrics.ImportGitRepoFailed.Inc()
							retErr = &cdqanalysis.InternalError{Err: fmt.Errorf(value)}
						}
					}
					log.Error(retErr, fmt.Sprintf("Unable to analyze the repo via kubernetes job... %v", req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, retErr)
					return ctrl.Result{}, nil
				}

			} else {
				k8sInfoClient := cdqanalysis.K8sInfoClient{
					Log:          log,
					CreateK8sJob: false,
				}

				devfilesMapReturned, devfilesURLMapReturned, dockerfileContextMapReturned, componentPortsMapReturned, revision, err = cdqanalysis.CloneAndAnalyze(k8sInfoClient, req.Namespace, req.Name, context, cdqInfo)
				if err != nil {
					switch err.(type) {
					case *cdqanalysis.NoDevfileFound:
						metrics.ImportGitRepoSucceeded.Inc()
						log.Error(err, fmt.Sprintf("NoDevfileFound error running cdq analysis... %v", req.NamespacedName))
					case *cdqanalysis.NoDockerfileFound:
						metrics.ImportGitRepoSucceeded.Inc()
						log.Error(err, fmt.Sprintf("NoDockerfileFound error running cdq analysis... %v", req.NamespacedName))
					case *cdqanalysis.RepoNotFound:
						metrics.ImportGitRepoSucceeded.Inc()
						log.Error(err, fmt.Sprintf("RepoNotFound error running cdq analysis... %v", req.NamespacedName))
					case *cdqanalysis.InvalidDevfile:
						metrics.ImportGitRepoSucceeded.Inc()
						log.Error(err, fmt.Sprintf("InvalidDevfile error running cdq analysis... %v", req.NamespacedName))
					case *cdqanalysis.InvalidURL:
						metrics.ImportGitRepoSucceeded.Inc()
						log.Error(err, fmt.Sprintf("InvalidURL error running cdq analysis... %v", req.NamespacedName))
					case *cdqanalysis.AuthenticationFailed:
						metrics.ImportGitRepoSucceeded.Inc()
						log.Error(err, fmt.Sprintf("AuthenticationFailed error running cdq analysis... %v", req.NamespacedName))
					default:
						// Increment the git import failure metric only on non user failure
						metrics.ImportGitRepoFailed.Inc()
						log.Error(err, fmt.Sprintf("Internal error running cdq analysis... %v", req.NamespacedName))
					}

					r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
					return ctrl.Result{}, nil
				}
			}
			metrics.ImportGitRepoSucceeded.Inc()
			maps.Copy(devfilesMap, devfilesMapReturned)
			maps.Copy(dockerfileContextMap, dockerfileContextMapReturned)
			maps.Copy(devfilesURLMap, devfilesURLMapReturned)
			maps.Copy(componentPortsMap, componentPortsMapReturned)
			devfilePath, _ = cdqanalysis.GetDevfileAndDockerFilePaths(*cdqInfo)
			componentDetectionQuery.Spec.GitSource.Revision = revision

		} else {
			log.Info(fmt.Sprintf("devfile was explicitly specified at %s %v", source.DevfileURL, req.NamespacedName))

			// For scenarios where a devfile is passed in, we still need to use the GH API for branch detection as we do not clone.
			if source.Revision == "" {
				log.Info(fmt.Sprintf("Look for default branch of repo %s... %v", source.URL, req.NamespacedName))
				metricsLabel := prometheus.Labels{"controller": cdqName, "tokenName": ghClient.TokenName, "operation": "GetDefaultBranchFromURL"}
				metrics.ControllerGitRequest.With(metricsLabel).Inc()
				source.Revision, err = ghClient.GetDefaultBranchFromURL(sourceURL, ctx)
				metrics.HandleRateLimitMetrics(err, metricsLabel)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to get default branch of Github Repo %v, try to fall back to main branch... %v", source.URL, req.NamespacedName))
					metricsLabel := prometheus.Labels{"controller": cdqName, "tokenName": ghClient.TokenName, "operation": "GetBranchFromURL"}
					metrics.ControllerGitRequest.With(metricsLabel).Inc()
					_, err := ghClient.GetBranchFromURL(sourceURL, ctx, "main")
					if err != nil {
						metrics.HandleRateLimitMetrics(err, metricsLabel)
						log.Error(err, fmt.Sprintf("Unable to get main branch of Github Repo %v ... %v", source.URL, req.NamespacedName))
						retErr := fmt.Errorf("unable to get default branch of Github Repo %v, try to fall back to main branch, failed to get main branch... %v", source.URL, req.NamespacedName)
						r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, retErr)
						return ctrl.Result{}, nil
					} else {
						source.Revision = "main"
					}
				}
			}

			// set in the CDQ spec
			componentDetectionQuery.Spec.GitSource.Revision = source.Revision

			shouldIgnoreDevfile, devfileBytes, err := cdqanalysis.ValidateDevfile(log, source.DevfileURL, gitToken)
			if err != nil {
				// if a direct devfileURL is provided and errors out, we dont do an alizer detection
				log.Error(err, fmt.Sprintf("Unable to GET %s, exiting reconcile loop %v", source.DevfileURL, req.NamespacedName))
				err := fmt.Errorf("unable to GET from %s", source.DevfileURL)
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
			if shouldIgnoreDevfile {
				// if a direct devfileURL is provided and errors out, we dont do an alizer detection
				log.Error(err, fmt.Sprintf("the provided devfileURL %s does not contain a valid outerloop definition, exiting reconcile loop %v", source.DevfileURL, req.NamespacedName))
				err := fmt.Errorf("the provided devfileURL %s does not contain a valid outerloop definition", source.DevfileURL)
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
			devfilesMap[context] = devfileBytes
			devfilesURLMap[context] = source.DevfileURL
		}

		for context := range devfilesMap {
			if _, ok := devfilesURLMap[context]; !ok {
				updatedLink, err := cdqanalysis.UpdateGitLink(source.URL, source.Revision, path.Join(context, devfilePath))
				if err != nil {
					log.Error(err, fmt.Sprintf(
						"Unable to update the devfile link %v", req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
					return ctrl.Result{}, nil
				}
				devfilesURLMap[context] = updatedLink
			}
		}
		// only update the componentStub when a component has been detected
		if len(devfilesMap) != 0 || len(devfilesURLMap) != 0 || len(dockerfileContextMap) != 0 {
			err = r.updateComponentStub(req, ctx, &componentDetectionQuery, devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, gitToken)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to update the component stub %v", req.NamespacedName))
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
		}

		r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, nil)
	} else {
		// CDQ resource has been requeued after it originally ran
		// Delete the resource as it's no longer needed and can be cleaned up
		log.Info(fmt.Sprintf("Deleting finished ComponentDetectionQuery resource %v", req.NamespacedName))
		if err = r.Delete(ctx, &componentDetectionQuery); err != nil {
			// Delete failed. Log the error but don't bother modifying the resource's status
			logutil.LogAPIResourceChangeEvent(log, componentDetectionQuery.Name, "ComponentDetectionQuery", logutil.ResourceDelete, err)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentDetectionQueryReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	log := ctrl.LoggerFrom(ctx).WithName("controllers").WithName("ComponentDetectionQuery")

	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.ComponentDetectionQuery{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).WithEventFilter(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			log := log.WithValues("namespace", e.Object.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "ComponentDetectionQuery", logutil.ResourceCreate, nil)
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := log.WithValues("namespace", e.ObjectNew.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "ComponentDetectionQuery", logutil.ResourceUpdate, nil)
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log := log.WithValues("namespace", e.Object.GetNamespace())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "ComponentDetectionQuery", logutil.ResourceDelete, nil)
			return false
		},
	}).
		Complete(r)
}

func waitForConfigMap(clientset *kubernetes.Clientset, ctx context.Context, name, namespace string) (*corev1.ConfigMap, error) {
	// 5 mins timeout
	timeout := int64(300)
	opts := metav1.ListOptions{
		TypeMeta:       metav1.TypeMeta{},
		FieldSelector:  fmt.Sprintf("metadata.name=%s", name),
		TimeoutSeconds: &timeout,
	}
	watcher, err := clientset.CoreV1().ConfigMaps(namespace).Watch(ctx, opts)
	if err != nil {
		return nil, err
	}
	defer watcher.Stop()

	for {
		select {
		case event := <-watcher.ResultChan():
			configMap := event.Object.(*corev1.ConfigMap)
			return configMap, nil

		case <-ctx.Done():
			return nil, nil
		}
	}
}

func cleanupK8sResources(log logr.Logger, clientset *kubernetes.Clientset, ctx context.Context, jobName, configMapName, namespace string) {
	log.Info(fmt.Sprintf("Attempting to cleanup k8s resources for cdq analysis... %s", namespace))
	log.Info(fmt.Sprintf("Deleting job %s... %s", jobName, namespace))

	jobsClient := clientset.BatchV1().Jobs(namespace)

	pp := metav1.DeletePropagationBackground

	err := jobsClient.Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &pp})

	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to delete job %s... %s", jobName, namespace))
	} else {
		log.Info(fmt.Sprintf("Successfully deleted job %s... %s", jobName, namespace))
	}

	log.Info(fmt.Sprintf("Deleting config map %s... %s", configMapName, namespace))
	configMapClient := clientset.CoreV1().ConfigMaps(namespace)
	err = configMapClient.Delete(ctx, configMapName, metav1.DeleteOptions{PropagationPolicy: &pp})
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to delete config map %s... %s", configMapName, namespace))
	} else {
		log.Info(fmt.Sprintf("Successfully deleted config map %s... %s", configMapName, namespace))
	}
}
