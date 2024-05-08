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

package availability

import (
	"context"
	"testing"

	"github.com/konflux-ci/application-service/pkg/github"
	"github.com/konflux-ci/application-service/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestCheckAvailability(t *testing.T) {

	t.Run("check availability", func(t *testing.T) {
		checkGitLabel := prometheus.Labels{"check": "github"}
		r := AvailabilityWatchdog{
			GitHubTokenClient: github.MockGitHubTokenClient{},
		}

		r.checkAvailability(context.TODO())

		assert.Equal(t, 1.0, testutil.ToFloat64(metrics.HASAvailabilityGauge.With(checkGitLabel)))
	})
}
