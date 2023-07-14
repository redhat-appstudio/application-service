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
	"github.com/redhat-developer/alizer/go/pkg/apis/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type K8sInfoClient struct {
	Ctx          context.Context
	Clientset    kubernetes.Interface
	Log          logr.Logger
	CreateK8sJob bool
}

// CDQ analyzer
// return values are for testing purpose
func CloneAndAnalyze(k K8sInfoClient, gitToken, namespace, name, context, devfilePath, dockerfilePath, URL, Revision, DevfileRegistryURL string, isDevfilePresent, isDockerfilePresent bool) (map[string][]byte, map[string]string, map[string]string, map[string][]int, error) {
	log := k.Log
	var clonePath, componentPath string
	alizerClient := AlizerClient{}
	devfilesMap := make(map[string][]byte)
	devfilesURLMap := make(map[string]string)
	dockerfileContextMap := make(map[string]string)
	componentPortsMap := make(map[string][]int)
	Fs := NewFilesystem()
	var err error

	var components []model.Component
	if context == "" {
		context = "./"
	}

	isMultiComponent := false
	if isDevfilePresent {
		updatedLink, err := UpdateGitLink(URL, Revision, path.Join(context, devfilePath))
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to update the devfile link for CDQ %v... %v", name, namespace))
			k.SendBackDetectionResult(devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, name, namespace, err)
			return nil, nil, nil, nil, err
		}
		shouldIgnoreDevfile, devfileBytes, err := ValidateDevfile(log, updatedLink)
		if err != nil {
			k.SendBackDetectionResult(devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, name, namespace, err)
			return nil, nil, nil, nil, err
		}
		if shouldIgnoreDevfile {
			isDevfilePresent = false
		} else {
			log.Info(fmt.Sprintf("Found a devfile, devfile to be analyzed to see if a Dockerfile is referenced for CDQ %v...%v", name, namespace))
			devfilesMap[context] = devfileBytes
			devfilesURLMap[context] = updatedLink
		}
	}
	// recheck if devfile presents, since the devfile may need to be ignored after validation
	if !isDevfilePresent && isDockerfilePresent {
		log.Info(fmt.Sprintf("Determined that this is a Dockerfile only component for cdq %v... %v", name, namespace))
		dockerfileContextMap[context] = dockerfilePath
	}

	clonePath, err = CreateTempPath(name, Fs)
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to create a temp path %s for cloning %v", clonePath, namespace))
		k.SendBackDetectionResult(devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, name, namespace, err)
		return nil, nil, nil, nil, err
	}

	err = CloneRepo(clonePath, URL, Revision, gitToken)
	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to clone repo %s to path %s, exiting reconcile loop %v", URL, clonePath, namespace))
		k.SendBackDetectionResult(devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, name, namespace, err)
		return nil, nil, nil, nil, err
	}
	log.Info(fmt.Sprintf("cloned from %s to path %s... %v", URL, clonePath, namespace))
	componentPath = clonePath
	if context != "" {
		componentPath = path.Join(clonePath, context)
	}

	if !isDockerfilePresent {
		log.Info(fmt.Sprintf("Unable to find devfile, Dockerfile or Containerfile under root directory, run Alizer to detect components... %v", namespace))

		if !isDevfilePresent {
			components, err = alizerClient.DetectComponents(componentPath)
			if err != nil {
				log.Error(err, fmt.Sprintf("Unable to detect components using Alizer for repo %v, under path %v... %v ", URL, componentPath, namespace))
				k.SendBackDetectionResult(devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, name, namespace, err)
				return nil, nil, nil, nil, err
			}
			log.Info(fmt.Sprintf("components detected %v... %v", components, namespace))
			// If no devfile and no Dockerfile or Containerfile present in the root
			// case 1: no components been detected by Alizer, might still has subfolders contains Dockerfile or Containerfile. Need to scan repo
			// case 2: one or more than 1 compinents been detected by Alizer, and the first one in the list is under sub-folder. Need to scan repo.
			if len(components) == 0 || (len(components) != 0 && path.Clean(components[0].Path) != path.Clean(componentPath)) {
				isMultiComponent = true
			}
		}
	}

	// Logic to read multiple components in from git
	if isMultiComponent {
		log.Info(fmt.Sprintf("Since this is a multi-component, attempt will be made to read only level 1 dir for devfiles... %v", namespace))
		devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, err = ScanRepo(log, alizerClient, componentPath, DevfileRegistryURL, URL, Revision, context)
		if err != nil {
			if _, ok := err.(*NoDevfileFound); !ok {
				log.Error(err, fmt.Sprintf("Unable to find devfile(s) in repo %s due to an error %s, exiting reconcile loop %v", URL, err.Error(), namespace))
				k.SendBackDetectionResult(devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, name, namespace, err)
				return nil, nil, nil, nil, err
			}
		}
	} else {
		log.Info(fmt.Sprintf("Since this is not a multi-component, attempt will be made to read devfile at the root dir... %v", namespace))
		err := AnalyzePath(log, alizerClient, componentPath, context, DevfileRegistryURL, devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, isDevfilePresent, isDockerfilePresent)
		if err != nil {
			log.Error(err, fmt.Sprintf("Unable to analyze path %s for a devfile, Dockerfile or Containerfile %v", componentPath, namespace))
			k.SendBackDetectionResult(devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, name, namespace, err)
			return nil, nil, nil, nil, err
		}
	}

	if isExist, _ := IsExisting(Fs, clonePath); isExist {
		if err := Fs.RemoveAll(clonePath); err != nil {
			log.Error(err, fmt.Sprintf("Unable to remove the clonepath %s %v", clonePath, namespace))
			k.SendBackDetectionResult(devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, name, namespace, err)
			return nil, nil, nil, nil, err
		}
	}

	k.SendBackDetectionResult(devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, name, namespace, nil)
	return devfilesMap, devfilesURLMap, dockerfileContextMap, componentPortsMap, nil
}

func (k K8sInfoClient) SendBackDetectionResult(devfilesMap map[string][]byte, devfilesURLMap map[string]string, dockerfileContextMap map[string]string, componentPortsMap map[string][]int, name, namespace string, completeError error) {
	log := k.Log
	if !k.CreateK8sJob {
		log.Info("Skip creating the job...")
		return
	}
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
