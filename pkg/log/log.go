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

package log

import (
	"fmt"

	"github.com/go-logr/logr"
)

type ResourceChangeType string

const (
	ResourceCreate ResourceChangeType = "Create"
	ResourceUpdate ResourceChangeType = "Update"
	ResourceDelete ResourceChangeType = "Delete"
	// complete is only for CDQ condition change upon completion
	ResourceComplete ResourceChangeType = "Complete"
)

func LogAPIResourceChangeEvent(log logr.Logger, resourceName string, resource string, resourceChangeType ResourceChangeType, err error) {
	log = log.WithValues("audit", "true")

	if resource == "" {
		log.Error(nil, "resource passed to LogAPIResourceChangeEvent was empty")
		return
	}
	log = log.WithValues("name", resourceName).WithValues("resource", resource).WithValues("action", resourceChangeType)
	if err != nil {
		log.Info(fmt.Sprintf("API Resource change event failed: %s, error: %v", string(resourceChangeType), err))
	} else {
		log.Info(fmt.Sprintf("API Resource changed: %s", string(resourceChangeType)))
	}
}
