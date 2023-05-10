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

import "fmt"

// NoFileFound returns an error if no file was found
type NoFileFound struct {
	Location string
	Err      error
}

func (e *NoFileFound) Error() string {
	errMsg := fmt.Sprintf("unable to find file in the specified location %s", e.Location)
	if e.Err != nil {
		errMsg = fmt.Sprintf("%s due to %v", errMsg, e.Err)
	}
	return errMsg
}

// NoDevfileFound returns an error if no devfile was found
type NoDevfileFound struct {
	Location string
	Err      error
}

func (e *NoDevfileFound) Error() string {
	errMsg := fmt.Sprintf("unable to find devfile in the specified location %s", e.Location)
	if e.Err != nil {
		errMsg = fmt.Sprintf("%s due to %v", errMsg, e.Err)
	}
	return errMsg
}

// NoDockerfileFound returns an error if no Dockerfile was found
type NoDockerfileFound struct {
	Location string
	Err      error
}

func (e *NoDockerfileFound) Error() string {
	errMsg := fmt.Sprintf("unable to find Dockerfile in the specified location %s", e.Location)
	if e.Err != nil {
		errMsg = fmt.Sprintf("%s due to %v", errMsg, e.Err)
	}
	return errMsg
}
