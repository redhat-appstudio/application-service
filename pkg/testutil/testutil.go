//
// Copyright 2023 Red Hat, Inc.
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

package util

import (
	"testing"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	"github.com/devfile/library/v2/pkg/devfile/parser/data/v2/common"
	. "github.com/onsi/gomega"
	devfilePkg "github.com/redhat-appstudio/application-service/pkg/devfile"

	corev1 "k8s.io/api/core/v1"
)

type UpdateChecklist struct {
	Route     string
	Port      int
	Replica   int
	Env       []corev1.EnvVar
	Resources corev1.ResourceRequirements
}

// verifyHASComponentUpdates verifies if the devfile data has been properly updated with the Component CR values
func VerifyHASComponentUpdates(devfile data.DevfileData, checklist UpdateChecklist, goPkgTest *testing.T) {
	// container component should be updated with the necessary hasComp properties
	components, err := devfile.GetComponents(common.DevfileOptions{
		ComponentOptions: common.ComponentOptions{
			ComponentType: v1alpha2.KubernetesComponentType,
		},
	})
	if goPkgTest == nil {
		Expect(err).Should(Not(HaveOccurred()))
	} else if err != nil {
		goPkgTest.Error(err)
	}

	requests := checklist.Resources.Requests
	limits := checklist.Resources.Limits

	for _, component := range components {
		componentAttributes := component.Attributes
		var err error

		// Check the route
		if checklist.Route != "" {
			route := componentAttributes.Get(devfilePkg.RouteKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(route).Should(Equal(checklist.Route))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if route != checklist.Route {
				goPkgTest.Errorf("expected: %v, got: %v", checklist.Route, route)
			}
		}

		// Check the replica
		if checklist.Replica != 0 {
			replicas := componentAttributes.Get(devfilePkg.ReplicaKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(replicas).Should(Equal(float64(checklist.Replica)))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if int(replicas.(float64)) != checklist.Replica {
				goPkgTest.Errorf("expected: %v, got: %v", checklist.Replica, replicas)
			}
		}

		// Check the storage limit
		if _, ok := limits[corev1.ResourceStorage]; ok {
			storageLimitChecklist := limits[corev1.ResourceStorage]
			storageLimit := componentAttributes.Get(devfilePkg.StorageLimitKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(storageLimit).Should(Equal(storageLimitChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if storageLimit.(string) != storageLimitChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", storageLimitChecklist.String(), storageLimit)
			}
		}

		// Check the storage request
		if _, ok := requests[corev1.ResourceStorage]; ok {
			storageRequestChecklist := requests[corev1.ResourceStorage]
			storageRequest := componentAttributes.Get(devfilePkg.StorageRequestKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(storageRequest).Should(Equal(storageRequestChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if storageRequest.(string) != storageRequestChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", storageRequestChecklist.String(), storageRequest)
			}
		}

		// Check the memory limit
		if _, ok := limits[corev1.ResourceMemory]; ok {
			memoryLimitChecklist := limits[corev1.ResourceMemory]
			memoryLimit := componentAttributes.Get(devfilePkg.MemoryLimitKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(memoryLimit.(string)).Should(Equal(memoryLimitChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if memoryLimit.(string) != memoryLimitChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", memoryLimitChecklist.String(), memoryLimit)
			}
		}

		// Check the memory request
		if _, ok := requests[corev1.ResourceMemory]; ok {
			memoryRequestChecklist := requests[corev1.ResourceMemory]
			memoryRequest := componentAttributes.Get(devfilePkg.MemoryRequestKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(memoryRequest).Should(Equal(memoryRequestChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if memoryRequest.(string) != memoryRequestChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", memoryRequestChecklist.String(), memoryRequest)
			}
		}

		// Check the cpu limit
		if _, ok := limits[corev1.ResourceCPU]; ok {
			cpuLimitChecklist := limits[corev1.ResourceCPU]
			cpuLimit := componentAttributes.Get(devfilePkg.CpuLimitKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(cpuLimit).Should(Equal(cpuLimitChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if cpuLimit.(string) != cpuLimitChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", cpuLimitChecklist.String(), cpuLimit)
			}
		}

		// Check the cpu request
		if _, ok := requests[corev1.ResourceCPU]; ok {
			cpuRequestChecklist := requests[corev1.ResourceCPU]
			cpuRequest := componentAttributes.Get(devfilePkg.CpuRequestKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(cpuRequest).Should(Equal(cpuRequestChecklist.String()))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if cpuRequest.(string) != cpuRequestChecklist.String() {
				goPkgTest.Errorf("expected: %v, got: %v", cpuRequestChecklist.String(), cpuRequest)
			}
		}

		// Check for container port
		if checklist.Port != 0 {
			containerPort := componentAttributes.Get(devfilePkg.ContainerImagePortKey, &err)
			if goPkgTest == nil {
				Expect(err).Should(Not(HaveOccurred()))
				Expect(containerPort).Should(Equal(float64(checklist.Port)))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if int(containerPort.(float64)) != checklist.Port {
				goPkgTest.Errorf("expected: %v, got: %v", checklist.Port, containerPort)
			}
		}
		// Check for env
		for _, checklistEnv := range checklist.Env {
			isMatched := false
			var containerENVs []corev1.EnvVar
			err := componentAttributes.GetInto(devfilePkg.ContainerENVKey, &containerENVs)
			for _, containerEnv := range containerENVs {
				if containerEnv.Name == checklistEnv.Name && containerEnv.Value == checklistEnv.Value {
					isMatched = true
				}
			}
			if goPkgTest == nil {
				Expect(isMatched).Should(Equal(true))
			} else if err != nil {
				goPkgTest.Error(err)
			} else if !isMatched {
				goPkgTest.Errorf("expected: %v, got: %v", true, isMatched)
			}
		}
	}
}
