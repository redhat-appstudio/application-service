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

	ApplicationCreationTotalReqs = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_application_creation_total",
			Help: "Number of application creation requests processed",
		},
	)
	ApplicationCreationFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_application_failed_creation_total",
			Help: "Number of failed application creation requests",
		},
	)

	ApplicationCreationSucceeded = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_application_successful_creation_total",
			Help: "Number of successful application creation requests",
		},
	)
)
