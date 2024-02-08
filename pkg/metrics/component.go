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
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	componentCreationTotalReqs = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_component_creation_total",
			Help: "Number of component creation requests processed",
		},
	)
	componentCreationFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_component_failed_creation_total",
			Help: "Number of failed component creation requests",
		},
	)

	componentCreationSucceeded = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "has_component_successful_creation_total",
			Help: "Number of successful component creation requests",
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

// IncrementComponentCreationFailed increments the component creation failed metric.
// Pass in the new error to update the metric, otherwise it will be ignored.
func IncrementComponentCreationFailed(oldError, newError string) {
	if newError != "" && (oldError == "" || !strings.Contains(oldError, newError)) {
		// pair the componentCreationTotalReqs counter with componentCreationFailed because
		// we dont want a situation where we increment componentCreationTotalReqs in the
		// beginning of a reconcile, and we skip the componentCreationFailed metric because
		// the errors are the same. Otherwise we will have a situation where neither the success
		// nor the fail metric is increasing but the total request count is increasing.
		componentCreationTotalReqs.Inc()
		componentCreationFailed.Inc()
	}
}

func GetComponentCreationFailed() prometheus.Counter {
	return componentCreationFailed
}

// IncrementComponentCreationSucceeded increments the component creation succeeded metric.
func IncrementComponentCreationSucceeded(oldError, newError string) {
	if oldError == "" || newError == "" || !strings.Contains(oldError, newError) {
		// pair the componentCreationTotalReqs counter with componentCreationSucceeded because
		// we dont want a situation where we increment componentCreationTotalReqs in the
		// beginning of a reconcile, and we skip the componentCreationSucceeded metric because
		// the errors are the same. Otherwise we will have a situation where neither the success
		// nor the fail metric is increasing but the total request count is increasing.
		componentCreationTotalReqs.Inc()
		componentCreationSucceeded.Inc()
	}
}

func GetComponentCreationSucceeded() prometheus.Counter {
	return componentCreationSucceeded
}

func GetComponentCreationTotalReqs() prometheus.Counter {
	return componentCreationTotalReqs
}
