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

package resources

// Resources represents a set of filename -> Go struct with the filenames as
// keys, and the values are values to be serialized to YAML.
type Resources map[string]interface{}

// Merge merges a set of resources in from to the set of resources in to,
// replacing existing keys and returns a new set of resources.
func Merge(from, to Resources) Resources {
	merged := Resources{}
	for k, v := range to {
		merged[k] = v
	}
	for k, v := range from {
		merged[k] = v
	}
	return merged
}
