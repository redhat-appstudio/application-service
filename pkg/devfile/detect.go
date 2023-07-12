//
// Copyright 2022-2023 Red Hat, Inc.
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
	"strings"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	"github.com/go-logr/logr"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-developer/alizer/go/pkg/apis/model"
	"github.com/redhat-developer/alizer/go/pkg/apis/recognizer"
	"sigs.k8s.io/yaml"
)

type Alizer interface {
	SelectDevFileFromTypes(path string, devFileTypes []model.DevFileType) (model.DevFileType, error)
	DetectComponents(path string) ([]model.Component, error)
}

type AlizerClient struct {
}

// search attempts to read and return devfiles and Dockerfiles/Containerfiles from the local path upto the specified depth
// If no devfile(s) or Dockerfile(s)/Containerfile(s) are found, then the Alizer tool is used to detect and match a devfile/Dockerfile from the devfile registry
// search returns 3 maps and an error:
// Map 1 returns a context to the devfile bytes if present.
// Map 2 returns a context to the matched devfileURL from the github repository. If no devfile was present, then a link to a matching devfile in the devfile registry will be used instead.
// Map 3 returns a context to the Dockerfile uri or a matched DockerfileURL from the devfile registry if no Dockerfile is present in the context
// Map 4 returns a context to the list of ports that were detected by alizer in the source code, at that given context
func search(log logr.Logger, a Alizer, localpath string, devfileRegistryURL string, source appstudiov1alpha1.GitSource) (map[string][]byte, map[string]string, map[string]string, map[string][]int, error) {
	devfileMapFromRepo := make(map[string][]byte)
	devfilesURLMapFromRepo := make(map[string]string)
	dockerfileContextMapFromRepo := make(map[string]string)
	componentPortsMapFromRepo := make(map[string][]int)

	files, err := ioutil.ReadDir(localpath)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	for _, f := range files {
		if f.IsDir() {
			isDevfilePresent := false
			isDockerfilePresent := false
			curPath := path.Join(localpath, f.Name())
			dirName := f.Name()
			context := path.Join(source.Context, f.Name())
			files, err := ioutil.ReadDir(curPath)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			for _, f := range files {
				if f.Name() == DevfileName || f.Name() == HiddenDevfileName {
					// Check for devfile.yaml or .devfile.yaml
					/* #nosec G304 -- false positive, filename is not based on user input*/
					devfilePath := path.Join(curPath, f.Name())
					// Set the proper devfile URL for the detected devfile
					updatedLink, err := UpdateGitLink(source.URL, source.Revision, path.Join(source.Context, path.Join(dirName, f.Name())))
					if err != nil {
						return nil, nil, nil, nil, err
					}
					shouldIgnoreDevfile, devfileBytes, err := ValidateDevfile(log, devfilePath)
					if err != nil {
						return nil, nil, nil, nil, err
					}
					if shouldIgnoreDevfile {
						isDevfilePresent = false
					} else {
						devfileMapFromRepo[context] = devfileBytes
						devfilesURLMapFromRepo[context] = updatedLink
						isDevfilePresent = true
					}
				} else if f.IsDir() && f.Name() == HiddenDevfileDir {
					// Check for .devfile/devfile.yaml or .devfile/.devfile.yaml
					// if the dir is .devfile, we dont increment currentLevel
					// consider devfile.yaml and .devfile/devfile.yaml as the same level, for example
					hiddenDirPath := path.Join(curPath, HiddenDevfileDir)
					hiddenfiles, err := ioutil.ReadDir(hiddenDirPath)
					if err != nil {
						return nil, nil, nil, nil, err
					}
					for _, f := range hiddenfiles {
						if f.Name() == DevfileName || f.Name() == HiddenDevfileName {
							// Check for devfile.yaml or .devfile.yaml
							/* #nosec G304 -- false positive, filename is not based on user input*/
							devfilePath := path.Join(hiddenDirPath, f.Name())

							// Set the proper devfile URL for the detected devfile
							updatedLink, err := UpdateGitLink(source.URL, source.Revision, path.Join(source.Context, path.Join(dirName, HiddenDevfileDir, f.Name())))
							if err != nil {
								return nil, nil, nil, nil, err
							}
							shouldIgnoreDevfile, devfileBytes, err := ValidateDevfile(log, devfilePath)
							if err != nil {
								return nil, nil, nil, nil, err
							}

							if shouldIgnoreDevfile {
								isDevfilePresent = false
							} else {
								devfileMapFromRepo[context] = devfileBytes
								devfilesURLMapFromRepo[context] = updatedLink

								isDevfilePresent = true
							}
						}
					}
				} else if f.Name() == DockerfileName {
					// Check for Dockerfile
					// NOTE: if a Dockerfile is named differently, for example, Dockerfile.jvm;
					// thats ok. As we finish iterating through all the files in the localpath
					// we will read the devfile to ensure a Dockerfile has been referenced.
					// However, if a Dockerfile is named differently and not referenced in the devfile
					// it will go undetected
					dockerfileContextMapFromRepo[context] = DockerfileName
					isDockerfilePresent = true
				} else if f.Name() == ContainerfileName {
					// Check for Containerfile
					dockerfileContextMapFromRepo[context] = ContainerfileName
					isDockerfilePresent = true
				} else if f.IsDir() && (f.Name() == DockerDir || f.Name() == HiddenDockerDir || f.Name() == BuildDir) {
					// Check for docker/Dockerfile, .docker/Dockerfile and build/Dockerfile
					// OR docker/Containerfile, .docker/Containerfile and build/Containerfile
					dirName := f.Name()
					dirPath := path.Join(curPath, dirName)
					files, err := ioutil.ReadDir(dirPath)
					if err != nil {
						return nil, nil, nil, nil, err
					}
					for _, f := range files {
						if f.Name() == DockerfileName || f.Name() == ContainerfileName {
							dockerfileContextMapFromRepo[context] = path.Join(dirName, f.Name())
							isDockerfilePresent = true
						}
					}
				}
			}
			// unset the Dockerfile context if we have both devfile and Dockerfile
			// at this stage, we need to ensure the Dockerfile has been referenced
			// in the devfile image component even if we detect both devfile and Dockerfile
			if isDevfilePresent && isDockerfilePresent {
				delete(dockerfileContextMapFromRepo, context)
				isDockerfilePresent = false
			}

			if (!isDevfilePresent && !isDockerfilePresent) || (isDevfilePresent && !isDockerfilePresent) {
				err := AnalyzePath(log, a, curPath, context, devfileRegistryURL, devfileMapFromRepo, devfilesURLMapFromRepo, dockerfileContextMapFromRepo, componentPortsMapFromRepo, isDevfilePresent, isDockerfilePresent)
				if err != nil {
					return nil, nil, nil, nil, err
				}
			}
		}
	}

	if len(devfilesURLMapFromRepo) == 0 && len(devfileMapFromRepo) == 0 && len(dockerfileContextMapFromRepo) == 0 {
		// if we didnt find any devfile or Dockerfile we should return an err
		log.Info(fmt.Sprintf("no devfile or Dockerfile found in the specified location %s", localpath))
	}

	return devfileMapFromRepo, devfilesURLMapFromRepo, dockerfileContextMapFromRepo, componentPortsMapFromRepo, err
}

// AnalyzePath checks if a devfile or a Dockerfile can be found in the localpath for the given context, this is a helper func used by the CDQ controller
// In addition to returning an error, the following maps may be updated:
// devfileMapFromRepo: a context to the devfile bytes if present
// devfilesURLMapFromRepo: a context to the matched devfileURL from the github repository. If no devfile was present, then a link to a matching devfile in the devfile registry will be used instead.
// dockerfileContextMapFromRepo: a context to the Dockerfile uri or a matched DockerfileURL from the devfile registry if no Dockerfile is present in the context
// componentPortsMapFromRepo: a context to the list of ports that were detected by alizer in the source code, at that given context
func AnalyzePath(log logr.Logger, a Alizer, localpath, context, devfileRegistryURL string, devfileMapFromRepo map[string][]byte, devfilesURLMapFromRepo, dockerfileContextMapFromRepo map[string]string, componentPortsMapFromRepo map[string][]int, isDevfilePresent, isDockerfilePresent bool) error {
	if isDevfilePresent {
		// If devfile is present, check to see if we can determine a Dockerfile from it
		devfileBytes := devfileMapFromRepo[context]
		dockerfileImage, err := SearchForDockerfile(devfileBytes)
		if err != nil {
			return err
		}
		if dockerfileImage != nil {
			// if it is an absolute uri, add it to the Dockerfile context map
			// If it's relative URI, leave it out, as the build will process the devfile and find the Dockerfile
			if strings.HasPrefix(dockerfileImage.Uri, "http") {
				dockerfileContextMapFromRepo[context] = dockerfileImage.Uri
			}
			isDockerfilePresent = true
		}

	}

	if !isDockerfilePresent {
		// if we didnt find any devfile/Dockerfile/Containerfile upto our desired depth, then use alizer
		detectedDevfile, detectedDevfileEndpoint, detectedSampleName, detectedPorts, err := AnalyzeAndDetectDevfile(a, localpath, devfileRegistryURL)
		if err != nil {
			if _, ok := err.(*NoDevfileFound); !ok {
				return err
			}
		}

		if len(detectedDevfile) > 0 {
			if !isDevfilePresent {
				// If a devfile is not present at this stage, just update devfileMapFromRepo and devfilesURLMapFromRepo
				// Dockerfile is not needed because all the devfile registry samples will have a Dockerfile entry
				devfileMapFromRepo[context] = detectedDevfile
				devfilesURLMapFromRepo[context] = detectedDevfileEndpoint
			}
			// 1. If a devfile is present but we could not determine a Dockerfile or,
			// 2. If a devfile is not present and we matched from the registry with Alizer
			// update dockerfileContextMapFromRepo with the Dockerfile full uri
			// by looking up the devfile from the detected alizer sample from the devfile registry
			sampleRepoURL, err := GetRepoFromRegistry(detectedSampleName, devfileRegistryURL)
			if err != nil {
				return err
			}

			dockerfileImage, err := SearchForDockerfile(detectedDevfile)
			if err != nil {
				return err
			}

			var dockerfileUri string
			if dockerfileImage != nil {
				dockerfileUri = dockerfileImage.Uri
			}
			link, err := UpdateGitLink(sampleRepoURL, "", dockerfileUri)
			if err != nil {
				return err
			}

			dockerfileContextMapFromRepo[context] = link
			componentPortsMapFromRepo[context] = detectedPorts
			isDockerfilePresent = true
		}
	}

	if !isDevfilePresent && isDockerfilePresent {
		// Still invoke alizer to detect the ports from the component
		_, _, _, detectedPorts, err := AnalyzeAndDetectDevfile(a, localpath, devfileRegistryURL)
		if err == nil {
			componentPortsMapFromRepo[context] = detectedPorts
		} else {
			log.Info(fmt.Sprintf("failed to detect port from context: %v, error: %v", context, err))
		}
	}

	return nil
}

// SearchForDockerfile searches for a Dockerfile from a devfile image component.
// If no Dockerfile is found, nil will be returned.
func SearchForDockerfile(devfileBytes []byte) (*v1alpha2.DockerfileImage, error) {
	if len(devfileBytes) == 0 {
		return nil, nil
	}
	devfileSrc := DevfileSrc{
		Data: string(devfileBytes),
	}
	devfileData, err := ParseDevfile(devfileSrc)
	if err != nil {
		return nil, err
	}
	devfileImageComponents, err := devfileData.GetComponents(common.DevfileOptions{
		ComponentOptions: common.ComponentOptions{
			ComponentType: v1alpha2.ImageComponentType,
		},
	})
	if err != nil {
		return nil, err
	}

	for _, component := range devfileImageComponents {
		// Only check for the Dockerfile Uri at this point, in later stages we need to account for Dockerfile from Git & the Registry
		if component.Image != nil && component.Image.Dockerfile != nil && component.Image.Dockerfile.DockerfileSrc.Uri != "" {
			return component.Image.Dockerfile, nil
		}
	}

	return nil, nil
}

// Analyze is a wrapper call to Alizer's Analyze()
func (a AlizerClient) Analyze(path string) ([]model.Language, error) {
	return recognizer.Analyze(path)
}

// SelectDevFileFromTypes is a wrapper call to Alizer's SelectDevFileFromTypes()
func (a AlizerClient) SelectDevFileFromTypes(path string, devFileTypes []model.DevFileType) (model.DevFileType, error) {
	index, err := recognizer.SelectDevFileFromTypes(path, devFileTypes)
	if err != nil {
		return model.DevFileType{}, err
	}
	return devFileTypes[index], err
}

func (a AlizerClient) DetectComponents(path string) ([]model.Component, error) {
	return recognizer.DetectComponents(path)
}

// AnalyzeAndDetectDevfile analyzes and attempts to detect a devfile from the devfile registry for a given local path
// The following values are returned, in addition to an error
// 1. the detected devfile, in bytes
// 2. the detected endpoints in the devfile
// 3. the detected type of the source code
// 4. the detected ports found in the source code
func AnalyzeAndDetectDevfile(a Alizer, path, devfileRegistryURL string) ([]byte, string, string, []int, error) {
	var devfileBytes []byte
	alizerDevfileTypes, err := getAlizerDevfileTypes(devfileRegistryURL)
	if err != nil {
		return nil, "", "", nil, err
	}

	alizerComponents, err := a.DetectComponents(path)
	if err != nil {
		return nil, "", "", nil, err
	}

	if len(alizerComponents) == 0 {
		return nil, "", "", nil, &NoDevfileFound{Location: path}
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
				return nil, "", "", nil, err
			} else if !reflect.DeepEqual(detectedType, model.DevFileType{}) {
				// Note: Do not use the Devfile registry endpoint devfileRegistry/devfiles/detectedType.Name
				// until the Devfile registry support uploads the Devfile Kubernetes component relative uri file
				// as an artifact and made accessible via devfile/library or devfile/registry-support
				sampleRepoURL, err := GetRepoFromRegistry(detectedType.Name, devfileRegistryURL)
				if err != nil {
					return nil, "", "", nil, err
				}
				detectedDevfileEndpoint, err := UpdateGitLink(sampleRepoURL, "", DevfileName)
				if err != nil {
					return nil, "", "", nil, err
				}

				devfileSrc := DevfileSrc{
					URL: detectedDevfileEndpoint,
				}
				compDevfileData, err := ParseDevfile(devfileSrc)
				if err != nil {
					return nil, "", "", nil, err
				}
				devfileBytes, err = yaml.Marshal(compDevfileData)
				if err != nil {
					return nil, "", "", nil, err
				}

				if len(devfileBytes) > 0 {
					return devfileBytes, detectedDevfileEndpoint, detectedType.Name, alizerComponents[0].Ports, nil
				}
			}
		}
	}

	return nil, "", "", nil, &NoDevfileFound{Location: path}
}
