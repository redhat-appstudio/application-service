//
// Copyright 2022 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package spi

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"github.com/redhat-appstudio/application-api/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"

	spiapi "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdqanalysis "github.com/redhat-appstudio/application-service/cdq-analysis/pkg"
	"github.com/redhat-appstudio/application-service/pkg/devfile"
)

type SPI interface {
	GetFileContents(ctx context.Context, name string, component v1alpha1.Component, repoUrl string, filepath string, ref string) (io.ReadCloser, error)
}

const (
	SPIFCR_waiting_for_delivered_phase = "File content request status has not been delivered"
	SPIFCR_prefix                      = "spi-fcr-"
)

type SPIClient struct {
	K8sClient client.Client
}

// SPIFileContentRequestError returns an internal error
type SPIFileContentRequestError struct {
	Message string
}

func (e *SPIFileContentRequestError) Error() string {
	return "SPIFileContentRequest failed " + e.Message
}

var ValidDevfileLocations = cdqanalysis.ValidDevfileLocations

// GetFileContents is a wrapper call to scm file retriever's GetFileContents()
func (s SPIClient) GetFileContents(ctx context.Context, name string, component v1alpha1.Component, repoUrl string, filepath string, ref string) (io.ReadCloser, error) {
	log := ctrl.LoggerFrom(ctx)
	spiFCRLookupKey := types.NamespacedName{Name: SPIFCR_prefix + name, Namespace: component.Namespace} //We'll use a unique component name and filepath to construct the name
	spiFCR := &spiapi.SPIFileContentRequest{}
	log.Info("Looking up SPIFileContentRequest CR", "name", spiFCRLookupKey.Name, "namespace", spiFCRLookupKey.Namespace)
	err := s.K8sClient.Get(ctx, spiFCRLookupKey, spiFCR)
	if err != nil {
		if errors.IsNotFound(err) {
			spiFCR.Name = spiFCRLookupKey.Name
			spiFCR.Namespace = spiFCRLookupKey.Namespace
			spiFCR.Spec.RepoUrl = repoUrl
			spiFCR.Spec.FilePath = filepath
			spiFCR.Spec.Ref = ref
			//add an owner reference
			ownerReference := metav1.OwnerReference{
				APIVersion: component.APIVersion,
				Kind:       component.Kind,
				Name:       component.Name,
				UID:        component.UID,
			}
			spiFCR.SetOwnerReferences(append(spiFCR.GetOwnerReferences(), ownerReference))

			log.Info("SPIFileContentRequest CR not found, creating a new request", "name", spiFCR.Name, "namespace", spiFCR.Namespace)
			err = s.K8sClient.Create(ctx, spiFCR)
			if err != nil {
				log.Error(err, "Unable to create an SPIFileContentRequest CR", "name", spiFCR.Name, "namespace", spiFCR.Namespace)
				return nil, &SPIFileContentRequestError{fmt.Sprintf("Failed to create an SPIFileContentRequest CR: %s", err.Error())}
			}
		}
	}

	return getFileContentFromSPIFCR(*spiFCR, log)
}

func getFileContentFromSPIFCR(fcr spiapi.SPIFileContentRequest, log logr.Logger) (io.ReadCloser, error) {

	if fcr.Status.Phase == spiapi.SPIFileContentRequestPhaseDelivered {
		fcrName := fcr.Name
		//get contents and decode them
		fileContent := fcr.Status.Content
		decodedFileContent, err := base64.StdEncoding.DecodeString(fileContent)
		if err != nil {
			return nil, &SPIFileContentRequestError{fmt.Sprintf("Failed to decode file content %s ", err)}
		}

		log.Info("SPIFileContentRequest: Decoded file contents successfully ", "name", fcrName, "namespace", fcr.Namespace)
		return io.NopCloser(bytes.NewBuffer(decodedFileContent)), nil
	} else {
		log.Info("SPI Phase delivered not reached")
		return nil, &SPIFileContentRequestError{SPIFCR_waiting_for_delivered_phase}
	}

}

func DownloadDevfileUsingSPI(s SPI, ctx context.Context, component v1alpha1.Component, repoURL string, ref string, path string) ([]byte, string, error) {
	for i, filename := range ValidDevfileLocations {
		devfileBytes, err := DownloadFileUsingSPI(s, ctx, component.Name+strconv.Itoa(i), component, repoURL, ref, filepath.Join("/", path, filename)) //pass in unique name so SPIFileContentRequest can be created for each valid devfile location type
		if err == nil {
			return devfileBytes, filename, nil
		} else {
			if _, ok := err.(*devfile.NoFileFound); !ok {
				return nil, filename, err
			}
		}
	}
	return nil, "", &cdqanalysis.NoDevfileFound{Location: repoURL}
}

func DownloadFileUsingSPI(s SPI, ctx context.Context, fcrName string, component v1alpha1.Component, repoURL string, ref string, filepath string) ([]byte, error) {
	// Call out to SPI via SPIFileContentRequests
	r, err := s.GetFileContents(ctx, fcrName, component, repoURL, filepath, ref)
	if err == nil {
		fileBytes, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		return fileBytes, nil
	}

	return nil, &devfile.NoFileFound{Location: repoURL, Err: err}
}

func DownloadDevfileandDockerfileUsingSPI(s SPI, ctx context.Context, name string, component v1alpha1.Component, repoURL string, ref string, path string) ([]byte, []byte, string, error) {

	devfileBytes, filename, err := DownloadDevfileUsingSPI(s, ctx, component, repoURL, ref, path)
	if err != nil {
		if _, ok := err.(*cdqanalysis.NoDevfileFound); !ok {
			return nil, nil, filename, err
		}
	}

	dockerfileBytes, err := DownloadFileUsingSPI(s, ctx, name, component, repoURL, ref, filepath.Join("/", path, "Dockerfile"))
	if err != nil {
		if _, ok := err.(*devfile.NoFileFound); !ok {
			return nil, nil, "", err
		}
	}

	return devfileBytes, dockerfileBytes, filename, nil
}
