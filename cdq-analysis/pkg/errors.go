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

// RepoNotFound returns an error if no git repo was found
type RepoNotFound struct {
	URL      string
	Revision string
	Err      error
}

func (e *RepoNotFound) Error() string {
	errMsg := fmt.Sprintf("unable to find git repository %s %s", e.URL, e.Revision)
	if e.Err != nil {
		errMsg = fmt.Sprintf("%s due to %v", errMsg, e.Err)
	}
	return errMsg
}

// InvalidDevfile returns an error if no devfile is invalid
type InvalidDevfile struct {
	Err error
}

func (e *InvalidDevfile) Error() string {
	var errMsg string
	if e.Err != nil {
		errMsg = fmt.Sprintf("invalid devfile due to %v", e.Err)
	}
	return errMsg
}

// InvalidURL returns an error if URL is invalid to be parsed
type InvalidURL struct {
	URL string
	Err error
}

func (e *InvalidURL) Error() string {
	var errMsg string
	if e.Err != nil {
		errMsg = fmt.Sprintf("invalid URL %v due to %v", e.URL, e.Err)
	}
	return errMsg
}

// AuthenticationFailed returns an error if authenticated failed when cloning the repository
// indicates the token is not valid
type AuthenticationFailed struct {
	URL string
	Err error
}

func (e *AuthenticationFailed) Error() string {
	var errMsg string
	if e.Err != nil {
		errMsg = fmt.Sprintf("authentication failed to URL %v: %v", e.URL, e.Err)
	}
	return errMsg
}

// InternalError returns cdq errors other than user error
type InternalError struct {
	Err error
}

func (e *InternalError) Error() string {
	errMsg := fmt.Sprintf("internal error: %v", e.Err)
	return errMsg
}
