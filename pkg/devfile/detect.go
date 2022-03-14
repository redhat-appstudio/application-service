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

	"github.com/redhat-appstudio/application-service/pkg/util"
	"github.com/redhat-developer/alizer/go/pkg/apis/recognizer"
)

// TODO - maysunfaisal
// There seems to a gap in the logic if we extend past depth 1 and discovering devfile logic
// Revisit post M4

// searchDevfiles searches a given localpath for a devfile upto the specified depth. If no devfile is present until
// the depth, alizer is used to analyze and detect a devfile from the registry. It returns a map of repo context to the devfile
// bytes, a map of repo context to the devfile detected(if any) and an error
func searchDevfiles(localpath string, currentLevel, depth int) (map[string][]byte, map[string]string, error) {

	devfileMapFromRepo := make(map[string][]byte)
	devfilesURLMapFromRepo := make(map[string]string)

	isDevfilePresent := false

	files, err := ioutil.ReadDir(localpath)
	if err != nil {
		return nil, nil, err
	}

	for _, f := range files {
		if (f.Name() == DevfileName || f.Name() == HiddenDevfileName) && currentLevel != 0 {
			devfileBytes, err := ioutil.ReadFile(path.Join(localpath, f.Name()))
			if err != nil {
				return nil, nil, err
			}

			context := getContext(localpath, currentLevel)
			devfileMapFromRepo[context] = devfileBytes
			isDevfilePresent = true
		} else if f.IsDir() && f.Name() == HiddenDevfileDir {
			// if the dir is .devfile, we dont increment currentLevel
			// consider devfile.yaml and .devfile/devfile.yaml as the same level
			recursiveDevfileMap, recursiveDevfileURLMap, err := searchDevfiles(path.Join(localpath, f.Name()), currentLevel, depth)
			if err != nil {
				return nil, nil, err
			}

			context := getContext(localpath, currentLevel)
			for recursiveContext := range recursiveDevfileMap {
				if recursiveContext == HiddenDevfileDir {
					devfileMapFromRepo[context] = recursiveDevfileMap[HiddenDevfileDir]
					devfilesURLMapFromRepo[context] = recursiveDevfileURLMap[HiddenDevfileDir]
					isDevfilePresent = true
				}
			}
		} else if f.IsDir() {
			if currentLevel+1 <= depth {
				recursiveDevfileMap, recursiveDevfileURLMap, err := searchDevfiles(path.Join(localpath, f.Name()), currentLevel+1, depth)
				if err != nil {
					return nil, nil, err
				}
				for context, devfile := range recursiveDevfileMap {
					devfileMapFromRepo[context] = devfile
					devfilesURLMapFromRepo[context] = recursiveDevfileURLMap[context]
					isDevfilePresent = true
				}
			}
		}
	}

	if len(devfileMapFromRepo) == 0 && currentLevel == 0 {
		// if we didnt find any devfile we should return an err
		err = &NoDevfileFound{location: localpath}
	} else if !isDevfilePresent && currentLevel == depth {
		// if we didnt find any devfile upto our desired depth, then use alizer
		devfileBytes, detectedDevfileEndpoint, err := AnalyzeAndDetectDevfile(localpath)
		if err != nil {
			if _, ok := err.(*NoDevfileFound); !ok {
				return nil, nil, err
			}
		}

		if len(devfileBytes) > 0 {
			context := getContext(localpath, currentLevel)
			devfileMapFromRepo[context] = devfileBytes
			devfilesURLMapFromRepo[context] = detectedDevfileEndpoint
		}

	}

	return devfileMapFromRepo, devfilesURLMapFromRepo, err
}

// AnalyzeAndDetectDevfile analyzes and attempts to detect a devfile from the devfile registry for a given local path
func AnalyzeAndDetectDevfile(path string) ([]byte, string, error) {
	var devfileBytes []byte

	alizerLanguages, err := recognizer.Analyze(path)
	if err != nil {
		return nil, "", err
	}

	alizerTypes, err := getAlizerDevfileTypes(DevfileStageRegistryEndpoint)
	if err != nil {
		return nil, "", err
	}

	for _, language := range alizerLanguages {
		if language.CanBeComponent {
			// if we get one language analysis that can be a component
			// we can then determine a devfile from the registry and return

			detectedType, err := recognizer.SelectDevFileFromTypes(path, alizerTypes)
			if err != nil && err.Error() != fmt.Sprintf("No valid devfile found for project in %s", path) {
				// No need to check for err, if a path does not have a detected devfile, ignore err
				// if a dir can be a component but we get an unrelated err, err out
				return nil, "", err
			} else if !reflect.DeepEqual(detectedType, recognizer.DevFileType{}) {
				detectedDevfileEndpoint := DevfileStageRegistryEndpoint + "/devfiles/" + detectedType.Name

				devfileBytes, err = util.CurlEndpoint(detectedDevfileEndpoint)
				if err != nil {
					return nil, "", err
				}

				if len(devfileBytes) > 0 {
					return devfileBytes, detectedDevfileEndpoint, nil
				}
			}
		}
	}

	return nil, "", &NoDevfileFound{location: path}
}
