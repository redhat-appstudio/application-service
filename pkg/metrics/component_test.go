//
// Copyright 2024 Red Hat, Inc.
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

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestComponentMetricsIncrement(t *testing.T) {

	tests := []struct {
		name                     string
		oldErr                   string
		newErr                   string
		expectSuccessIncremented bool
		expectFailureIncremented bool
	}{
		{
			name:                     "no errors",
			oldErr:                   "",
			newErr:                   "",
			expectSuccessIncremented: true,
			expectFailureIncremented: false,
		},
		{
			name:                     "no old error, new error",
			oldErr:                   "",
			newErr:                   "error",
			expectSuccessIncremented: true,
			expectFailureIncremented: true,
		},
		{
			name:                     "old error, no new error",
			oldErr:                   "error",
			newErr:                   "",
			expectSuccessIncremented: true,
			expectFailureIncremented: false,
		},
		{
			name:                     "old error, new error - same error",
			oldErr:                   "error",
			newErr:                   "error",
			expectSuccessIncremented: false,
			expectFailureIncremented: false,
		},
		{
			name:                     "old error, new error - different error",
			oldErr:                   "error",
			newErr:                   "error 2",
			expectSuccessIncremented: true,
			expectFailureIncremented: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beforeCreateSuccess := testutil.ToFloat64(componentCreationSucceeded)
			beforeCreateFailed := testutil.ToFloat64(componentCreationFailed)

			IncrementComponentCreationSucceeded(tt.oldErr, tt.newErr)

			if tt.expectSuccessIncremented && testutil.ToFloat64(componentCreationSucceeded) <= beforeCreateSuccess {
				t.Errorf("TestComponentMetricsIncrement error: expected component create success metrics to be incremented but was not incremented")
			} else if !tt.expectSuccessIncremented && testutil.ToFloat64(componentCreationSucceeded) > beforeCreateSuccess {
				t.Errorf("TestComponentMetricsIncrement error: expected component create success metrics not to be incremented but it was incremented")
			}

			IncrementComponentCreationFailed(tt.oldErr, tt.newErr)

			if tt.expectFailureIncremented && testutil.ToFloat64(componentCreationFailed) <= beforeCreateFailed {
				t.Errorf("TestComponentMetricsIncrement error: expected component create failed metrics to be incremented but was not incremented")
			} else if !tt.expectFailureIncremented && testutil.ToFloat64(componentCreationFailed) > beforeCreateFailed {
				t.Errorf("TestComponentMetricsIncrement error: expected component create failed metrics not to be incremented but it was incremented")
			}

		})
	}
}
