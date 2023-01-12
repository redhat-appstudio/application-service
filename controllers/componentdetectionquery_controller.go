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

	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/maps"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"
	logicalcluster "github.com/kcp-dev/logicalcluster/v2"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	devfile "github.com/redhat-appstudio/application-service/pkg/devfile"
	logutil "github.com/redhat-appstudio/application-service/pkg/log"
	"github.com/redhat-appstudio/application-service/pkg/spi"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ComponentDetectionQueryReconciler reconciles a ComponentDetectionQuery object
type ComponentDetectionQueryReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	SPIClient          spi.SPI
	AlizerClient       devfile.Alizer
	Log                logr.Logger
	DevfileRegistryURL string
	AppFS              afero.Afero
}

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
	log := r.Log.WithValues("ComponentDetectionQuery", req.NamespacedName).WithValues("clusterName", req.ClusterName)
	// if we're running on kcp, we need to include workspace in context
	if req.ClusterName != "" {
		ctx = logicalcluster.WithCluster(ctx, logicalcluster.New(req.ClusterName))
	}

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

		source := componentDetectionQuery.Spec.GitSource
		var devfileBytes, dockerfileBytes []byte
		var devfilePath string
		devfilesMap := make(map[string][]byte)
		devfilesURLMap := make(map[string]string)
		dockerfileContextMap := make(map[string]string)

		context := source.Context
		if context == "" {
			context = "./"
		}

		if source.DevfileURL == "" {
			isDockerfilePresent := false
			isDevfilePresent := false
			log.Info(fmt.Sprintf("Attempting to read a devfile from the URL %s... %v", source.URL, req.NamespacedName))
			// check if the project is multi-component or single-component
			if gitToken == "" {
				gitURL, err := util.ConvertGitHubURL(source.URL, source.Revision, context)
				log.Info(fmt.Sprintf("Look for devfile or dockerfile at the URL %s... %v", gitURL, req.NamespacedName))
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to convert Github URL to raw format, exiting reconcile loop %v", req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
					return ctrl.Result{}, nil
				}
				devfileBytes, devfilePath, dockerfileBytes = devfile.DownloadDevfileAndDockerfile(gitURL)
			} else {
				// Use SPI to retrieve the devfile from the private repository
				// TODO - maysunfaisal also search for Dockerfile
				devfileBytes, err = spi.DownloadDevfileUsingSPI(r.SPIClient, ctx, componentDetectionQuery.Namespace, source.URL, "main", context)
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to curl for any known devfile locations from %s... %v", source.URL, req.NamespacedName))
				}
			}

			isDevfilePresent = len(devfileBytes) != 0
			isDockerfilePresent = len(dockerfileBytes) != 0

			if isDevfilePresent {
				log.Info(fmt.Sprintf("Found a devfile, devfile to be analyzed to see if a Dockerfile is referenced %v", req.NamespacedName))
				devfilesMap[context] = devfileBytes
			} else if isDockerfilePresent {
				log.Info(fmt.Sprintf("Determined that this is a Dockerfile only component  %v", req.NamespacedName))
				dockerfileContextMap[context] = "./Dockerfile"
			}

			// perfume cdq job that requires repo cloning and azlier analysis
			config, err := rest.InClusterConfig()
			if err != nil {
				log.Error(err, fmt.Sprintf("Error creating InClusterConfig... %v", req.NamespacedName))
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
			clientset, err := kubernetes.NewForConfig(config)
			if err != nil {
				log.Error(err, fmt.Sprintf("Error creating clientset with config... %v", req.NamespacedName))
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
			jobs := clientset.BatchV1().Jobs(req.Namespace)
			jobName := req.Name + "-job"
			var backOffLimit int32 = 0
			jobSpec := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      req.Name + "-job",
					Namespace: req.Namespace,
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							ServiceAccountName: "application-service-controller-manager",
							Containers: []corev1.Container{
								{
									Name:    jobName,
									Image:   "quay.io/redhat-appstudio/cdq-analysis:latest",
									Command: []string{"/app/main", gitToken, req.Namespace, req.Name, context, devfilePath, source.URL, source.Revision, r.DevfileRegistryURL, fmt.Sprintf("%v", isDevfilePresent), fmt.Sprintf("%v", isDockerfilePresent)},
								},
							},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
					BackoffLimit: &backOffLimit,
				},
			}

			_, err = jobs.Create(ctx, jobSpec, metav1.CreateOptions{})
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
			var devfileMapReturned map[string][]byte
			var devfilesURLMapReturned map[string]string
			var dockerfileContextMapReturned map[string]string
			var retErr error
			json.Unmarshal(cm.BinaryData["devfilesMap"], &devfileMapReturned)
			json.Unmarshal(cm.BinaryData["dockerfileContextMap"], &dockerfileContextMapReturned)
			json.Unmarshal(cm.BinaryData["devfilesURLMap"], &devfilesURLMapReturned)
			json.Unmarshal(cm.BinaryData["error"], &retErr)
			cleanupK8sResources(log, clientset, ctx, fmt.Sprintf("%s-job", req.Name), req.Name, req.Namespace)
			if retErr != nil {
				log.Error(retErr, fmt.Sprintf("Unable to analyze the repo via kubernetes job... %v", req.NamespacedName))
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, retErr)
				return ctrl.Result{}, nil
			}
			maps.Copy(devfilesMap, devfileMapReturned)
			maps.Copy(dockerfileContextMap, dockerfileContextMapReturned)
			maps.Copy(devfilesURLMap, devfilesURLMapReturned)
		} else {
			log.Info(fmt.Sprintf("devfile was explicitly specified at %s %v", source.DevfileURL, req.NamespacedName))
			devfileBytes, err = util.CurlEndpoint(source.DevfileURL)
			if err != nil {
				// if a direct devfileURL is provided and errors out, we dont do an alizer detection
				log.Error(err, fmt.Sprintf("Unable to GET %s, exiting reconcile loop %v", source.DevfileURL, req.NamespacedName))
				err := fmt.Errorf("unable to GET from %s", source.DevfileURL)
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
			devfilesMap[context] = devfileBytes
		}

		for context, link := range dockerfileContextMap {
			updatedLink, err := devfile.UpdateGitLink(source.URL, source.Revision, link)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to update the dockerfile link... %v", req.NamespacedName))
				r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				return ctrl.Result{}, nil
			}
			dockerfileContextMap[context] = updatedLink
		}

		for context := range devfilesMap {
			if _, ok := devfilesURLMap[context]; !ok {
				updatedLink, err := devfile.UpdateGitLink(source.URL, source.Revision, path.Join(context, devfilePath))
				if err != nil {
					log.Error(err, fmt.Sprintf("Unable to update the devfile link... %v", req.NamespacedName))
					r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
					return ctrl.Result{}, nil
				}
				devfilesURLMap[context] = updatedLink
			}
		}

		err = r.updateComponentStub(req, &componentDetectionQuery, devfilesMap, devfilesURLMap, dockerfileContextMap)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to update the component stub... %v", req.NamespacedName))
			r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
			return ctrl.Result{}, nil
		}

		r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, nil)
	} else {
		// CDQ resource has been requeued after it originally ran
		// Delete the resource as it's no longer needed and can be cleaned up
		log.Info(fmt.Sprintf("Deleting finished ComponentDetectionQuery resource... %v", req.NamespacedName))
		if err = r.Delete(ctx, &componentDetectionQuery); err != nil {
			// Delete failed. Log the error but don't bother modifying the resource's status
			logutil.LogAPIResourceChangeEvent(log, componentDetectionQuery.Name, "ComponentDetectionQuery", logutil.ResourceDelete, err)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentDetectionQueryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := zap.Options{
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log.WithName("controllers").WithName("ComponentDetectionQuery").WithValues("appstudio-component", "HAS")
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.ComponentDetectionQuery{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).WithEventFilter(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			log := log.WithValues("Namespace", e.Object.GetNamespace()).WithValues("clusterName", logicalcluster.From(e.Object).String())
			logutil.LogAPIResourceChangeEvent(log, e.Object.GetName(), "ComponentDetectionQuery", logutil.ResourceCreate, nil)
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := log.WithValues("Namespace", e.ObjectNew.GetNamespace()).WithValues("clusterName", logicalcluster.From(e.ObjectNew).String())
			logutil.LogAPIResourceChangeEvent(log, e.ObjectNew.GetName(), "ComponentDetectionQuery", logutil.ResourceUpdate, nil)
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log := log.WithValues("Namespace", e.Object.GetNamespace()).WithValues("clusterName", logicalcluster.From(e.Object).String())
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

	err := jobsClient.Delete(context.Background(), jobName, metav1.DeleteOptions{PropagationPolicy: &pp})

	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to delete job %s... %s", jobName, namespace))
	} else {
		log.Info(fmt.Sprintf("Successfully deleted job %s... %s", jobName, namespace))
	}

	log.Info(fmt.Sprintf("Deleting config map %s... %s", configMapName, namespace))
	configMapClient := clientset.CoreV1().ConfigMaps(namespace)
	err = configMapClient.Delete(context.Background(), configMapName, metav1.DeleteOptions{PropagationPolicy: &pp})
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to delete config map %s... %s", configMapName, namespace))
	} else {
		log.Info(fmt.Sprintf("Successfully deleted config map %s... %s", configMapName, namespace))
	}
}
