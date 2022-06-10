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
	routeKey = "deployment/route"

	// replicaKey is the key to reference replica
	replicaKey = "deployment/replicas"

	// storageLimitKey is the key to reference storage limit
	storageLimitKey = "deployment/storageLimit"

	// storageRequestKey is the key to reference storage request
	storageRequestKey = "deployment/storageRequest"

	// cpuLimitKey is the key to reference cpu limit
	cpuLimitKey = "deployment/cpuLimit"

	// cpuRequestKey is the key to reference cpu request
	cpuRequestKey = "deployment/cpuRequest"

	// memoryLimitKey is the key to reference memory limit
	memoryLimitKey = "deployment/memoryLimit"

	// memoryRequestKey is the key to reference memory request
	memoryRequestKey = "deployment/memoryRequest"

	// containerImagePortKey is the key to reference container image port
	containerImagePortKey = "deployment/container-port"

	// containerENVKey is the key to reference container environment variables
	containerENVKey = "deployment/containerENV"
)
