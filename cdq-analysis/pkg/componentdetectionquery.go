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

package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type K8sInfoClient struct{
	Ctx context.Context
	Clientset kubernetes.Interface
}

// CDQ analyzer
// return values are for testing purpose
func CloneAndAnalyze(k K8sInfoClient, gitToken, namespace, name, context, devfilePath, URL, Revision, DevfileRegistryURL string, isDevfilePresent, isDockerfilePresent bool) (map[string][]byte, map[string]string, map[string]string, error){
	opts := zap.Options{
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log.WithName("cdq-analysis").WithName("CloneAndAnalyze")
	var clonePath, componentPath string
	alizerClient := AlizerClient{}
	devfilesMap := make(map[string][]byte)
	devfilesURLMap := make(map[string]string)
	dockerfileContextMap := make(map[string]string)
	Fs := NewFilesystem()
	var err error
	if context == "" {
		context = "./"
	}

	isMultiComponent := false

	if !isDockerfilePresent {
		log.Info(fmt.Sprintf("Unable to find devfile or Dockerfile under root directory, run Alizer to detect components... %v", namespace))

		clonePath, err = CreateTempPath(name, Fs)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to create a temp path %s for cloning %v", clonePath, namespace))
			k.SendBackDetectionResult(log, devfilesMap, devfilesURLMap, dockerfileContextMap, name, namespace, err)
			return nil, nil, nil, err
		}

		err = CloneRepo(clonePath, URL, gitToken)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to clone repo %s to path %s, exiting reconcile loop %v", URL, clonePath, namespace))
			k.SendBackDetectionResult(log, devfilesMap, devfilesURLMap, dockerfileContextMap, name, namespace, err)
			return nil, nil, nil, err
		}
		log.Info(fmt.Sprintf("cloned from %s to path %s... %v", URL, clonePath, namespace))
		componentPath = clonePath
		if context != "" {
			componentPath = path.Join(clonePath, context)
		}

		if !isDevfilePresent {
			components, err := alizerClient.DetectComponents(componentPath)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to detect components using Alizer for repo %v, under path %v... %v ", URL, componentPath, namespace))
				// r.SetCompleteConditionAndUpdateCR(ctx, req, &componentDetectionQuery, copiedCDQ, err)
				k.SendBackDetectionResult(log, devfilesMap, devfilesURLMap, dockerfileContextMap, name, namespace, err)
				return nil, nil, nil, err
			}
			log.Info(fmt.Sprintf("components detected %v... %v", components, namespace))
			// If no devfile and no dockerfile present in the root
			// case 1: no components been detected by Alizer, might still has subfolders contains dockerfile. Need to scan repo
			// case 2: more than 1 components been detected by Alizer, is certain a multi-component project. Need to scan repo
			// case 3: one or more than 1 compinents been detected by Alizer, and the first one in the list is under sub-folder. Need to scan repo.
			if len(components) != 1 || (len(components) != 0 && path.Clean(components[0].Path) != path.Clean(componentPath)) {
				isMultiComponent = true
			}
		}
	}

	// Logic to read multiple components in from git
	if isMultiComponent {
		log.Info(fmt.Sprintf("Since this is a multi-component, attempt will be made to read only level 1 dir for devfiles... %v", namespace))
		devfilesMap, devfilesURLMap, dockerfileContextMap, err = ScanRepo(log, alizerClient, componentPath, DevfileRegistryURL, URL, Revision, context)
		if err != nil {
			if _, ok := err.(*NoDevfileFound); !ok {
				log.Error(err, fmt.Sprintf("Unable to find devfile(s) in repo %s due to an error %s, exiting reconcile loop %v", URL, err.Error(), namespace))
				k.SendBackDetectionResult(log, devfilesMap, devfilesURLMap, dockerfileContextMap, name, namespace, err)
				return nil, nil, nil, err
			}
		}
	} else {
		log.Info(fmt.Sprintf("Since this is not a multi-component, attempt will be made to read devfile at the root dir... %v", namespace))
		err := AnalyzePath(alizerClient, componentPath, context, DevfileRegistryURL, devfilesMap, devfilesURLMap, dockerfileContextMap, isDevfilePresent, isDockerfilePresent)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to analyze path %s for a dockerfile/devfile %v", componentPath, namespace))
			k.SendBackDetectionResult(log, devfilesMap, devfilesURLMap, dockerfileContextMap, name, namespace, err)
			return nil, nil, nil, err
		}
	}

	if isExist, _ := IsExisting(Fs, clonePath); isExist {
		if err := Fs.RemoveAll(clonePath); err != nil {
			log.Error(err, fmt.Sprintf("Unable to remove the clonepath %s %v", clonePath, namespace))
			k.SendBackDetectionResult(log, devfilesMap, devfilesURLMap, dockerfileContextMap, name, namespace, err)
			return nil, nil, nil, err
		}
	}

	k.SendBackDetectionResult(log, devfilesMap, devfilesURLMap, dockerfileContextMap, name, namespace, nil)
	return devfilesMap, devfilesURLMap, dockerfileContextMap, nil
}

// func SendBackDetectionResult(log logr.Logger, devfilesMap map[string][]byte, devfilesURLMap map[string]string, dockerfileContextMap map[string]string, name, namespace string, completeError error) {
// 	ctx := context.Background()
// 	config, err := rest.InClusterConfig()
// 	if err != nil {
// 		log.Error(err, fmt.Sprintf("Error creating InClusterConfig"))
// 	}

// 	clientset, err := kubernetes.NewForConfig(config)
// 	if err != nil {
// 		log.Error(err, fmt.Sprintf("Error creating clientset with config %v", config))
// 	}
// 	SendBackDetectionResultHelper(clientset, ctx, log, devfilesMap, devfilesURLMap, dockerfileContextMap, name, namespace, completeError)
// }

func (k K8sInfoClient)SendBackDetectionResult(log logr.Logger, devfilesMap map[string][]byte, devfilesURLMap map[string]string, dockerfileContextMap map[string]string, name, namespace string, completeError error) {
	log.Info(fmt.Sprintf("Sending back result, devfilesMap %v,devfilesURLMap %v, dockerfileContextMap %v , error %v ... %v", devfilesMap, devfilesURLMap, dockerfileContextMap, completeError, namespace))

	configMapBinaryData := make(map[string][]byte)
	if devfilesMap != nil {
		devfilesMapbytes, _ := json.Marshal(devfilesMap)
		configMapBinaryData["devfilesMap"] = devfilesMapbytes
	}
	if devfilesURLMap != nil {
		devfilesURLMapbytes, _ := json.Marshal(devfilesURLMap)
		configMapBinaryData["devfilesURLMap"] = devfilesURLMapbytes
	}

	if dockerfileContextMap != nil {
		dockerfileContextMapbytes, _ := json.Marshal(dockerfileContextMap)
		configMapBinaryData["dockerfileContextMap"] = dockerfileContextMapbytes
	}
	if completeError != nil {
		errorMap := make(map[string]string)
		switch completeError.(type) {
			case *NoDevfileFound:
				errorMap["NoDevfileFound"] = fmt.Sprintf("%v", completeError)
			case *NoDockerfileFound:
				errorMap["NoDockerfileFound"] = fmt.Sprintf("%v", completeError)
			default:
				errorMap["InternalError"] = fmt.Sprintf("%v", completeError)
		}
		errorMapbytes, _ := json.Marshal(errorMap)
		configMapBinaryData["errorMap"] = errorMapbytes
	}

	configMap := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		BinaryData: configMapBinaryData,
	}
	_, err := k.Clientset.CoreV1().ConfigMaps(namespace).Create(k.Ctx, &configMap, metav1.CreateOptions{})
	if err != nil {
		log.Error(err, fmt.Sprintf("Error creating configmap"))
	}
	return
}
