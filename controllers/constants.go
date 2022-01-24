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

const (
	// routeKey is the key to reference route
	routeKey = "appstudio.has/route"

	// replicaKey is the key to reference replica
	replicaKey = "appstudio.has/replicas"

	// storageLimitKey is the key to reference storage limit
	storageLimitKey = "appstudio.has/storageLimit"

	// ephemeralStorageLimitKey is the key to reference ephemeral storage limit
	ephemeralStorageLimitKey = "appstudio.has/ephemeralStorageLimit"

	// storageRequestKey is the key to reference storage request
	storageRequestKey = "appstudio.has/storageRequest"

	// ephemeralStorageRequestKey is the key to reference ephemeral storage request
	ephemeralStorageRequestKey = "appstudio.has/ephemeralStorageRequest"

	// applicationKey is the key to reference an application-service CR application
	applicationKey = "appstudio.has/application"

	// componentKey is the key to reference an application-service CR component
	componentKey = "appstudio.has/component"
)

// Github related constants.
const (
	GithubOrgConfMapKey = "GITHUB_ORG"

	githubTokenSecret = "has-github-token"

	githubTokenSecretKey = "token"
)
