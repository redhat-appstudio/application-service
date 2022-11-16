/*
Copyright 2022 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"net/http"

	"github.com/kcp-dev/logicalcluster/v2"

	"k8s.io/client-go/rest"
)

// NewClusterConfig wraps an existing config's roundtripper
// with a custom cluster aware roundtripper.
func NewClusterConfig(cfg *rest.Config) *rest.Config {
	copyCfg := rest.CopyConfig(cfg)
	copyCfg.Wrap(func(rt http.RoundTripper) http.RoundTripper {
		return NewClusterRoundTripper(rt)
	})

	return copyCfg
}

// ConfigWithCluster modifies the config host path to include the
// cluster endpoint.
func ConfigWithCluster(cfg *rest.Config, clusterName logicalcluster.Name) *rest.Config {
	r := rest.CopyConfig(cfg)
	r.Host = r.Host + clusterName.Path()
	return r
}
