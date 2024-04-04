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

package metrics

import (
	gh "github.com/google/go-github/v59/github"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(GitOpsRepoCreationTotalReqs, GitOpsRepoCreationFailed, GitOpsRepoCreationSucceeded, ControllerGitRequest,
		SecondaryRateLimitCounter, PrimaryRateLimitCounter, TokenPoolGauge, HASAvailabilityGauge,
		ApplicationDeletionTotalReqs, ApplicationDeletionSucceeded, ApplicationDeletionFailed,
		ApplicationCreationSucceeded, ApplicationCreationFailed, ApplicationCreationTotalReqs,
		componentCreationTotalReqs, componentCreationSucceeded, componentCreationFailed,
		ComponentDeletionTotalReqs, ComponentDeletionSucceeded, ComponentDeletionFailed,
		ImportGitRepoTotalReqs, ImportGitRepoFailed, ImportGitRepoSucceeded)
}

// HandleRateLimitMetrics checks the error type to verify a primary or secondary rate limit has been encountered
func HandleRateLimitMetrics(err error, labels prometheus.Labels) {
	if _, ok := err.(*gh.RateLimitError); ok {
		PrimaryRateLimitCounter.With(labels).Inc()
	} else if _, ok := err.(*gh.AbuseRateLimitError); ok {
		SecondaryRateLimitCounter.With(labels).Inc()
	}
}
