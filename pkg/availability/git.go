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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redhat-appstudio/application-service/pkg/github"
	"github.com/redhat-appstudio/application-service/pkg/metrics"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Inspired from https://github.com/konflux-ci/remote-secret/blob/main/pkg/availability/storage_watchdog.go

type AvailabilityWatchdog struct {
	GitHubTokenClient github.GitHubToken
}

func (r *AvailabilityWatchdog) Start(ctx context.Context) error {
	// Check every 20 minutes
	ticker := time.NewTicker(20 * time.Minute)
	go func() {
		for {
			// make call immediately to avoid waiting for the first tick
			r.checkAvailability(ctx)
			select {
			case <-ticker.C:
				continue
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
	return nil
}

func (r *AvailabilityWatchdog) checkAvailability(ctx context.Context) {
	log := ctrl.LoggerFrom(ctx)
	checkGitLabel := prometheus.Labels{"check": "github"}

	ghClient, err := r.GitHubTokenClient.GetNewGitHubClient("")
	if err != nil {
		log.Error(err, "Unable to create Go-GitHub client due to error, marking HAS availability as down")
		metrics.HASAvailabilityGauge.With(checkGitLabel).Set(0)
	}

	isGitAvailable, err := ghClient.GetGitStatus(ctx)
	if !isGitAvailable {
		log.Error(err, "Unable to create Go-GitHub client due to error, marking HAS availability as down")
		metrics.HASAvailabilityGauge.With(checkGitLabel).Set(0)
	} else {
		log.Info("HAS is marked as available, Git check has passed...")
		metrics.HASAvailabilityGauge.With(checkGitLabel).Set(1)
	}
}
