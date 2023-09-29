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
	gh "github.com/google/go-github/v52/github"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	GitOpsRepoCreationTotalReqs = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_gitops_repo_creation_total",
			Help: "Number of gitops creation requests processed",
		},
	)
	GitOpsRepoCreationFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_gitops_failed_repo_creation_total",
			Help: "Number of failed gitops creation requests",
		},
	)

	GitOpsRepoCreationSucceeded = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_gitops_successful_repo_creation_total",
			Help: "Number of successful gitops creation requests",
		},
	)

	ControllerGitRequest = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controller_git_request",
			Help: "Number of git operation requests.  Not an SLI metric",
		},
		[]string{"controller", "tokenName", "operation"},
	)

	SecondaryRateLimitCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "secondary_rate_limit_total",
			Help: "Number of times the secondary rate limit has been reached.  Not an SLI metric",
		},
		[]string{"controller", "tokenName", "operation"},
	)

	PrimaryRateLimitCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "primary_rate_limit_total",
			Help: "Number of times the primary rate limit has been reached.  Not an SLI metric",
		},
		[]string{"controller", "tokenName", "operation"},
	)

	TokenPoolGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "token_pool_gauge",
			Help: "Gauge counter to track whether a token has been primary/secondary rate limited",
		},

		//rateLimited - can have the value of "primary" or "secondary"
		//tokenName - the name of the token being rate limited
		[]string{"rateLimited", "tokenName"},
	)

	ApplicationDeletionTotalReqs = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_application_deletion_total",
			Help: "Number of application deletion requests processed",
		},
	)
	ApplicationDeletionFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_application_failed_deletion_total",
			Help: "Number of failed application deletion requests",
		},
	)

	ApplicationDeletionSucceeded = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_application_successful_deletion_total",
			Help: "Number of successful application deletion requests",
		},
	)

	ComponentDeletionTotalReqs = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_component_deletion_total",
			Help: "Number of component deletion requests processed",
		},
	)
	ComponentDeletionFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_component_failed_deletion_total",
			Help: "Number of failed component deletion requests",
		},
	)

	ComponentDeletionSucceeded = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_component_successful_deletion_total",
			Help: "Number of successful component deletion requests",
		},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(GitOpsRepoCreationTotalReqs, GitOpsRepoCreationFailed, GitOpsRepoCreationSucceeded, ControllerGitRequest, SecondaryRateLimitCounter,
		PrimaryRateLimitCounter, TokenPoolGauge, ApplicationDeletionTotalReqs, ApplicationDeletionSucceeded, ApplicationDeletionFailed,
		ComponentDeletionTotalReqs, ComponentDeletionSucceeded, ComponentDeletionFailed)
}

// HandleRateLimitMetrics checks the error type to verify a primary or secondary rate limit has been encountered
func HandleRateLimitMetrics(err error, labels prometheus.Labels) {
	if _, ok := err.(*gh.RateLimitError); ok {
		PrimaryRateLimitCounter.With(labels).Inc()
	} else if _, ok := err.(*gh.AbuseRateLimitError); ok {
		SecondaryRateLimitCounter.With(labels).Inc()
	}
}
