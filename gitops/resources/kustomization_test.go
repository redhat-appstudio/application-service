//
// Copyright 2021-2022 Red Hat, Inc.
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

// From https://github.com/redhat-developer/kam/tree/master/pkg/pipelines/resources
package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_AddResource(t *testing.T) {
	k := Kustomization{}
	k.AddResources("testing.yaml", "testing2.yaml")

	if diff := cmp.Diff([]string{"testing.yaml", "testing2.yaml"}, k.Resources); diff != "" {
		t.Fatalf("failed to add resources:\n%s", diff)
	}
}

func Test_AddBases(t *testing.T) {
	k := Kustomization{}
	k.AddBases("testing.yaml", "testing2.yaml")

	if diff := cmp.Diff([]string{"testing.yaml", "testing2.yaml"}, k.Bases); diff != "" {
		t.Fatalf("failed to add resources:\n%s", diff)
	}
}

func Test_AddPatches(t *testing.T) {
	k := Kustomization{}
	k.AddPatches("testing.yaml", "testing2.yaml")

	if diff := cmp.Diff([]string{"testing.yaml", "testing2.yaml"}, k.Patches); diff != "" {
		t.Fatalf("failed to add resources:\n%s", diff)
	}
}

func Test_AddResource_with_duplicates(t *testing.T) {
	k := Kustomization{}
	k.AddResources("testing.yaml", "testing2.yaml")
	k.AddResources("testing.yaml")

	if diff := cmp.Diff([]string{"testing.yaml", "testing2.yaml"}, k.Resources); diff != "" {
		t.Fatalf("failed to add resources:\n%s", diff)
	}
}

func Test_AddResource_sorts_elements(t *testing.T) {
	k := Kustomization{}
	k.AddResources("service.yaml", "deployment.yaml", "namespace.yaml")

	if diff := cmp.Diff([]string{"deployment.yaml", "namespace.yaml", "service.yaml"}, k.Resources); diff != "" {
		t.Fatalf("failed to sort resources:\n%s", diff)
	}
}
