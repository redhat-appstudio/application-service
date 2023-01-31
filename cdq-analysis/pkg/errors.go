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

package pkg

import "fmt"

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

// NoDockerfileFound returns an error if no dockerfile was found
type NoDockerfileFound struct {
	Location string
	Err      error
}

func (e *NoDockerfileFound) Error() string {
	errMsg := fmt.Sprintf("unable to find dockerfile in the specified location %s", e.Location)
	if e.Err != nil {
		errMsg = fmt.Sprintf("%s due to %v", errMsg, e.Err)
	}
	return errMsg
}
