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

import "github.com/prometheus/client_golang/prometheus"

var (
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

	ImportGitRepoTotalReqs = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_import_git_repo_total",
			Help: "Number of import from git repository requests processed",
		},
	)
	ImportGitRepoFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_failed_importc_git_repo_total",
			Help: "Number of failed import from git repository requests",
		},
	)

	ImportGitRepoSucceeded = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_successful_import_git_repo_total",
			Help: "Number of successful import from git repository requests",
		},
	)
)
