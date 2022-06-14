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

package devfile

import (
	"fmt"
	"io/ioutil"
	"path"
	"reflect"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"
	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/redhat-developer/alizer/go/pkg/apis/language"
	"github.com/redhat-developer/alizer/go/pkg/apis/recognizer"
)

type Alizer interface {
	SelectDevFileFromTypes(path string, devFileTypes []recognizer.DevFileType) (recognizer.DevFileType, error)
	DetectComponents(path string) ([]recognizer.Component, error)
}

type AlizerClient struct {
}

// search attempts to read and return devfiles and dockerfiles from the local path upto the specified depth
// If no devfile(s) or dockerfile(s) are found, then the Alizer tool is used to detect and match a devfile/dockerfile from the devfile registry
// search returns 3 maps and an error:
// Map 1 returns a context to the devfile bytes if present.
// Map 2 returns a context to the matched devfileURL from the devfile registry if no devfile is present in the context.
// Map 3 returns a context to the dockerfile uri or a matched dockerfileURL from the devfile registry if no dockerfile is present in the context
func search(log logr.Logger, a Alizer, localpath string, devfileRegistryURL string) (map[string][]byte, map[string]string, map[string]string, error) {

	devfileMapFromRepo := make(map[string][]byte)
	devfilesURLMapFromRepo := make(map[string]string)
	dockerfileContextMapFromRepo := make(map[string]string)

	files, err := ioutil.ReadDir(localpath)
	if err != nil {
		return nil, nil, nil, err
	}

	for _, f := range files {
		if f.IsDir() {
			isDevfilePresent := false
			isDockerfilePresent := false
			curPath := path.Join(localpath, f.Name())
			context := f.Name()
			files, err := ioutil.ReadDir(curPath)
			if err != nil {
				return nil, nil, nil, err
			}
			for _, f := range files {
				if f.Name() == DevfileName || f.Name() == HiddenDevfileName {
					// Check for devfile.yaml or .devfile.yaml
					devfileBytes, err := ioutil.ReadFile(path.Join(curPath, f.Name()))
					if err != nil {
						return nil, nil, nil, err
					}

					devfileMapFromRepo[context] = devfileBytes
					isDevfilePresent = true
				} else if f.IsDir() && f.Name() == HiddenDevfileDir {
					// Check for .devfile/devfile.yaml or .devfile/.devfile.yaml
					// if the dir is .devfile, we dont increment currentLevel
					// consider devfile.yaml and .devfile/devfile.yaml as the same level, for example
					hiddenDirPath := path.Join(curPath, HiddenDevfileDir)
					hiddenfiles, err := ioutil.ReadDir(hiddenDirPath)
					if err != nil {
						return nil, nil, nil, err
					}
					for _, f := range hiddenfiles {
						if f.Name() == DevfileName || f.Name() == HiddenDevfileName {
							// Check for devfile.yaml or .devfile.yaml
							devfileBytes, err := ioutil.ReadFile(path.Join(hiddenDirPath, f.Name()))
							if err != nil {
								return nil, nil, nil, err
							}

							devfileMapFromRepo[context] = devfileBytes
							isDevfilePresent = true
						}
					}
				} else if f.Name() == DockerfileName {
					// Check for Dockerfile
					// NOTE: if a Dockerfile is named differently, for example, Dockerfile.jvm;
					// thats ok. As we finish iterating through all the files in the localpath
					// we will read the devfile to ensure a dockerfile has been referenced.
					// However, if a Dockerfile is named differently and not referenced in the devfile
					// it will go undetected
					dockerfileContextMapFromRepo[context] = path.Join(context, DockerfileName)
					isDockerfilePresent = true
				}
			}
			// unset the dockerfile context if we have both devfile and dockerfile
			// at this stage, we need to ensure the dockerfile has been referenced
			// in the devfile image component even if we detect both devfile and dockerfile
			if isDevfilePresent && isDockerfilePresent {
				delete(dockerfileContextMapFromRepo, context)
				isDockerfilePresent = false
			}

			if (!isDevfilePresent && !isDockerfilePresent) || (isDevfilePresent && !isDockerfilePresent) {
				err := AnalyzePath(a, curPath, context, devfileRegistryURL, devfileMapFromRepo, devfilesURLMapFromRepo, dockerfileContextMapFromRepo, isDevfilePresent, isDockerfilePresent)
				if err != nil {
					return nil, nil, nil, err
				}
			}
		}
	}

	if len(devfileMapFromRepo) == 0 {
		// if we didnt find any devfile we should return an err
		err = &NoDevfileFound{Location: localpath}
	}

	return devfileMapFromRepo, devfilesURLMapFromRepo, dockerfileContextMapFromRepo, err
}

// AnalyzePath checks if a devfile or a dockerfile can be found in the localpath for the given context, this is a helper func used by the CDQ controller
func AnalyzePath(a Alizer, localpath, context, devfileRegistryURL string, devfileMapFromRepo map[string][]byte, devfilesURLMapFromRepo, dockerfileContextMapFromRepo map[string]string, isDevfilePresent, isDockerfilePresent bool) error {
	if isDevfilePresent {
		// If devfile is present, check to see if we can determine a Dockerfile from it
		devfile := devfileMapFromRepo[context]

		if dockerfileUri, err := SearchForDockerfile(devfile); err != nil {
			return err
		} else if len(dockerfileUri) > 0 {
			isDockerfilePresent = true
		}
	}

	if !isDockerfilePresent {
		// if we didnt find any devfile/dockerfile upto our desired depth, then use alizer
		devfile, detectedDevfileEndpoint, detectedSampleName, err := AnalyzeAndDetectDevfile(a, localpath, devfileRegistryURL)
		if err != nil {
			if _, ok := err.(*NoDevfileFound); !ok {
				return err
			}
		}

		if !isDevfilePresent && len(devfile) > 0 {
			// If a devfile is not present at this stage, just update devfileMapFromRepo and devfilesURLMapFromRepo
			// Dockerfile is not needed because all the devfile registry samples will have a Dockerfile entry
			devfileMapFromRepo[context] = devfile
			devfilesURLMapFromRepo[context] = detectedDevfileEndpoint
		} else if isDevfilePresent && len(devfile) > 0 {
			// If a devfile is present but we could not determine a dockerfile, then update dockerfileContextMapFromRepo
			// by looking up the devfile from the detected alizer sample from the devfile registry
			sampleRepoURL, err := GetRepoFromRegistry(detectedSampleName, devfileRegistryURL)
			if err != nil {
				return err
			}

			dockerfileUri, err := SearchForDockerfile(devfile)
			if err != nil {
				return err
			}

			link, err := UpdateDockerfileLink(sampleRepoURL, "", dockerfileUri)
			if err != nil {
				return err
			}

			dockerfileContextMapFromRepo[context] = link
			isDockerfilePresent = true
		}
	}

	return nil
}

// SearchForDockerfile searches for a Dockerfile from a devfile image component and
// returns the dockerfile uri
func SearchForDockerfile(devfile []byte) (string, error) {
	var dockerfile string

	devfileData, err := ParseDevfileModel(string(devfile))
	if err != nil {
		return "", err
	}
	devfileImageComponents, err := devfileData.GetComponents(common.DevfileOptions{
		ComponentOptions: common.ComponentOptions{
			ComponentType: v1alpha2.ImageComponentType,
		},
	})
	if err != nil {
		return "", err
	}

	for _, component := range devfileImageComponents {
		// Only check for the Dockerfile Uri at this point, in later stages we need to account for Dockerfile from Git & the Registry
		if component.Image != nil && component.Image.Dockerfile != nil && component.Image.Dockerfile.DockerfileSrc.Uri != "" {
			dockerfile = component.Image.Dockerfile.DockerfileSrc.Uri
			break
		}
	}

	return dockerfile, nil
}

// Analyze is a wrapper call to Alizer's Analyze()
func (a AlizerClient) Analyze(path string) ([]language.Language, error) {
	return recognizer.Analyze(path)
}

// SelectDevFileFromTypes is a wrapper call to Alizer's SelectDevFileFromTypes()
func (a AlizerClient) SelectDevFileFromTypes(path string, devFileTypes []recognizer.DevFileType) (recognizer.DevFileType, error) {
	index, err := recognizer.SelectDevFileFromTypes(path, devFileTypes)
	return devFileTypes[index], err
}

func (a AlizerClient) DetectComponents(path string) ([]recognizer.Component, error) {
	return recognizer.DetectComponents(path)
}

// AnalyzeAndDetectDevfile analyzes and attempts to detect a devfile from the devfile registry for a given local path
func AnalyzeAndDetectDevfile(a Alizer, path, devfileRegistryURL string) ([]byte, string, string, error) {
	var devfileBytes []byte
	alizerDevfileTypes, err := getAlizerDevfileTypes(devfileRegistryURL)
	if err != nil {
		return nil, "", "", err
	}

	alizerComponents, err := a.DetectComponents(path)
	if err != nil {
		return nil, "", "", err
	}

	if len(alizerComponents) == 0 {
		return nil, "", "", &NoDevfileFound{Location: path}
	}

	// Assuming it's a single component. as multi-component should be handled before
	for _, language := range alizerComponents[0].Languages {
		if language.CanBeComponent {
			// if we get one language analysis that can be a component
			// we can then determine a devfile from the registry and return

			// The highest rank is the most suggested component. priorty: configuration file > high %

			detectedType, err := a.SelectDevFileFromTypes(path, alizerDevfileTypes)
			if err != nil && err.Error() != fmt.Sprintf("No valid devfile found for project in %s", path) {
				// No need to check for err, if a path does not have a detected devfile, ignore err
				// if a dir can be a component but we get an unrelated err, err out
				return nil, "", "", err
			} else if !reflect.DeepEqual(detectedType, recognizer.DevFileType{}) {
				detectedDevfileEndpoint := devfileRegistryURL + "/devfiles/" + detectedType.Name
				devfileBytes, err = util.CurlEndpoint(detectedDevfileEndpoint)
				if err != nil {
					return nil, "", "", err
				}

				if len(devfileBytes) > 0 {
					return devfileBytes, detectedDevfileEndpoint, detectedType.Name, nil
				}
			}
		}
	}

	return nil, "", "", &NoDevfileFound{Location: path}
}
