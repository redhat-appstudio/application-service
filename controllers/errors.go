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

package controllers

import (
	"fmt"

	"github.com/redhat-developer/gitops-generator/pkg/util"
)

type GitOpsParseRepoError struct {
	remoteURL string
	err       error
}

func (e *GitOpsParseRepoError) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf("unable to parse gitops repository %s due to error: %v", e.remoteURL, e.err)).Error()
}

type GitOpsCommitIdError struct {
	err error
}

func (e *GitOpsCommitIdError) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf("unable to retrieve gitops repository commit id due to error: %v", e.err)).Error()
}

type NotSupported struct {
	err error
}

func (e *NotSupported) Error() string {
	return util.SanitizeErrorMessage(fmt.Errorf("not supported error: %v", e.err)).Error()
}
