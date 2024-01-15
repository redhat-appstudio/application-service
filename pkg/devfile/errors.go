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

// MissingOuterloop returns an error if no Kubernetes Component was found in a Devfile
type MissingOuterloop struct {
}

func (e *MissingOuterloop) Error() string {
	return "the devfile has no kubernetes components defined, missing outerloop definition"
}

// IncompatibleDevfile returns an error if the Devfile being read is incompatible due to user error
type IncompatibleDevfile struct {
	Err error
}

func (e *IncompatibleDevfile) Error() string {
	return fmt.Sprintf("devfile is incompatible: %v", e.Err)
}

// DevfileAttributeParse returns an error if was an issue parsing the attribute key
type DevfileAttributeParse struct {
	Key string
	Err error
}

func (e *DevfileAttributeParse) Error() string {
	errMsg := fmt.Sprintf("error parsing key %s: %v", e.Key, e.Err)

	return errMsg
}
