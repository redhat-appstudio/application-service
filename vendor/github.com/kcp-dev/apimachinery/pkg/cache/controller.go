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

package cache

import (
	"fmt"
	"strings"

	"github.com/kcp-dev/logicalcluster/v2"
	"k8s.io/apimachinery/pkg/api/meta"
)

const (
	// ClusterIndexName is the name of the index that allows you to filter by cluster
	ClusterIndexName = "cluster"
	// ClusterAndNamespaceIndexName is the name of index that allows you to filter by cluster and namespace
	ClusterAndNamespaceIndexName = "cluster-and-namespace"
)

// ClusterIndexFunc indexes by cluster name
func ClusterIndexFunc(obj interface{}) ([]string, error) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return []string{}, fmt.Errorf("object has no meta: %v", err)
	}
	clusterName := logicalcluster.From(meta).String()
	return []string{ToClusterAwareKey(clusterName, "", "")}, nil
}

// ClusterAndNamespaceIndexFunc indexes by cluster and namespace name
func ClusterAndNamespaceIndexFunc(obj interface{}) ([]string, error) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return []string{}, fmt.Errorf("object has no meta: %v", err)
	}
	clusterName := logicalcluster.From(meta).String()
	return []string{ToClusterAwareKey(clusterName, meta.GetNamespace(), "")}, nil

}

// ClusterAwareKeyFunc keys on cluster, namespace and name
func ClusterAwareKeyFunc(obj interface{}) (string, error) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return "", fmt.Errorf("object has no meta: %v", err)
	}
	clusterName := logicalcluster.From(meta).String()
	namespace := meta.GetNamespace()
	name := meta.GetName()

	return ToClusterAwareKey(clusterName, namespace, name), nil
}

// ToClusterAwareKey is a helper function that formats cluster, namespace, and name for key and index functions
func ToClusterAwareKey(cluster, namespace, name string) string {
	return strings.Join([]string{cluster, namespace, name}, "/")
}

// SplitClusterAwareKey is a helper function that extracts the cluster name, namespace, and name from a cluster-aware key
func SplitClusterAwareKey(clusterKey string) (string, string, string, error) {
	bits := strings.Split(clusterKey, "/")
	if len(bits) != 3 {
		return "", "", "", fmt.Errorf("%s is not a valid cluster-aware key", clusterKey)
	}
	return bits[0], bits[1], bits[2], nil
}
