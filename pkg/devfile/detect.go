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
	Analyze(path string) ([]language.Language, error)
	SelectDevFileFromTypes(path string, devFileTypes []recognizer.DevFileType) (recognizer.DevFileType, error)
}

type AlizerClient struct {
}

// searchDevfiles searches a given localpath for a devfile upto the specified depth. If no devfile is present until
// the depth, alizer is used to analyze and detect a devfile from the registry. It returns a map of repo context to the devfile
// bytes, a map of repo context to the devfile detected(if any) and an error
func searchDevfiles(log logr.Logger, a Alizer, localpath string, currentLevel, depth int, devfileRegistryURL string) (map[string][]byte, map[string]string, map[string]string, error) {
	// TODO - maysunfaisal
	// There seems to a gap in the logic if we extend past depth 1 and discovering devfile logic
	// Revisit post M4

	devfileMapFromRepo := make(map[string][]byte)
	devfilesURLMapFromRepo := make(map[string]string)
	dockerfileContextMapFromRepo := make(map[string]string)

	isDevfilePresent := false
	isDockerfilePresent := false

	files, err := ioutil.ReadDir(localpath)
	if err != nil {
		return nil, nil, nil, err
	}

	context := getContext(localpath, currentLevel)

	for _, f := range files {
		if (f.Name() == DevfileName || f.Name() == HiddenDevfileName) && currentLevel != 0 {
			// Check for devfile.yaml or .devfile.yaml
			devfileBytes, err := ioutil.ReadFile(path.Join(localpath, f.Name()))
			if err != nil {
				return nil, nil, nil, err
			}

			devfileMapFromRepo[context] = devfileBytes
			isDevfilePresent = true
		} else if f.IsDir() && f.Name() == HiddenDevfileDir && currentLevel != 0 {
			// Check for .devfile/devfile.yaml or .devfile/.devfile.yaml
			// if the dir is .devfile, we dont increment currentLevel
			// consider devfile.yaml and .devfile/devfile.yaml as the same level, for example
			recursiveDevfileMap, recursiveDevfileURLMap, _, err := searchDevfiles(log, a, path.Join(localpath, f.Name()), currentLevel, depth, devfileRegistryURL)
			if err != nil {
				return nil, nil, nil, err
			}

			for recursiveContext := range recursiveDevfileMap {
				if recursiveContext == HiddenDevfileDir {
					devfileMapFromRepo[context] = recursiveDevfileMap[HiddenDevfileDir]
					devfilesURLMapFromRepo[context] = recursiveDevfileURLMap[HiddenDevfileDir]
					isDevfilePresent = true
				}
			}
		} else if f.Name() == DockerfileName && currentLevel != 0 {
			// Check for Dockerfile
			// NOTE: if a Dockerfile is named differently, for example, Dockerfile.jvm;
			// thats ok. As we finish iterating through all the files in the localpath
			// we will read the devfile to ensure a dockerfile has been referenced.
			// However, if a Dockerfile is named differently and not referenced in the devfile
			// it will go undetected
			dockerfileContextMapFromRepo[context] = path.Join(context, DockerfileName)
			isDockerfilePresent = true
		} else if f.IsDir() {
			if currentLevel+1 <= depth {
				recursiveDevfileMap, recursiveDevfileURLMap, recursiveDockerfileContextMap, err := searchDevfiles(log, a, path.Join(localpath, f.Name()), currentLevel+1, depth, devfileRegistryURL)
				if err != nil {
					return nil, nil, nil, err
				}
				for context, devfile := range recursiveDevfileMap {
					devfileMapFromRepo[context] = devfile
					devfilesURLMapFromRepo[context] = recursiveDevfileURLMap[context]
					isDevfilePresent = true
				}
				for context := range recursiveDockerfileContextMap {
					dockerfileContextMapFromRepo[context] = recursiveDockerfileContextMap[context]
					isDockerfilePresent = true
				}
			}
		}
	}

	// unset the dockerfile context if we have both devfile and dockerfile
	// at this stage, we need to ensure the dockerfile has been referenced
	// in the devfile image component even if we detect both devfile and dockerfile
	if isDevfilePresent && isDockerfilePresent && currentLevel == depth {
		delete(dockerfileContextMapFromRepo, context)
		isDockerfilePresent = false
	}

	if len(devfileMapFromRepo) == 0 && currentLevel == 0 {
		// if we didnt find any devfile we should return an err
		err = &NoDevfileFound{location: localpath}
	} else if ((!isDevfilePresent && !isDockerfilePresent) || (isDevfilePresent && !isDockerfilePresent)) && currentLevel == depth {
		// If devfile is present, check to see if we can determine a Dockerfile from it
		if isDevfilePresent {
			devfile := devfileMapFromRepo[context]

			dockerfileUri, err := searchForDockerfile(devfile)
			if err != nil {
				return nil, nil, nil, err
			}
			if len(dockerfileUri) > 0 {
				isDockerfilePresent = true
			}
		}

		if !isDockerfilePresent {
			// if we didnt find any devfile/dockerfile upto our desired depth, then use alizer
			devfile, detectedDevfileEndpoint, detectedSampleName, err := AnalyzeAndDetectDevfile(a, localpath, devfileRegistryURL)
			if err != nil {
				if _, ok := err.(*NoDevfileFound); !ok {
					return nil, nil, nil, err
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

				sampleRepoURL, err := getRepoFromRegistry(detectedSampleName, devfileRegistryURL)
				if err != nil {
					return nil, nil, nil, err
				}

				dockerfileUri, err := searchForDockerfile(devfile)
				if err != nil {
					return nil, nil, nil, err
				}

				link, err := UpdateDockerfileLink(sampleRepoURL, dockerfileUri)
				if err != nil {
					return nil, nil, nil, err
				}

				dockerfileContextMapFromRepo[context] = link
				isDockerfilePresent = true
			}
		}

	}

	return devfileMapFromRepo, devfilesURLMapFromRepo, dockerfileContextMapFromRepo, err
}

// searchForDockerfile searches for a Dockerfile from a devfile image component and
// returns the dockerfile uri
func searchForDockerfile(devfile []byte) (string, error) {
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
	return recognizer.SelectDevFileFromTypes(path, devFileTypes)
}

// AnalyzeAndDetectDevfile analyzes and attempts to detect a devfile from the devfile registry for a given local path
func AnalyzeAndDetectDevfile(a Alizer, path, devfileRegistryURL string) ([]byte, string, string, error) {
	var devfileBytes []byte

	alizerLanguages, err := a.Analyze(path)
	if err != nil {
		return nil, "", "", err
	}

	alizerDevfileTypes, err := getAlizerDevfileTypes(devfileRegistryURL)
	if err != nil {
		return nil, "", "", err
	}

	for _, language := range alizerLanguages {
		if language.CanBeComponent {
			// if we get one language analysis that can be a component
			// we can then determine a devfile from the registry and return

			// TODO maysunfaisal
			// This is not right, check for the highest % in use rather than opting for the first & returning

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

	return nil, "", "", &NoDevfileFound{location: path}
}
