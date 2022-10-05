package log

import (
	"fmt"

	"github.com/go-logr/logr"
)

type ResourceChangeType string

const (
	ResourceCreate   ResourceChangeType = "Create"
	ResourceUpdate   ResourceChangeType = "Update"
	ResourceDelete   ResourceChangeType = "Delete"
	ResourceComplete ResourceChangeType = "Complete"
)

func LogAPIResourceChangeEvent(log logr.Logger, resourceName string, resource string, resourceChangeType ResourceChangeType) {
	log = log.WithValues("audit", "true")

	if resource == "" {
		log.Error(nil, "resource passed to LogAPIResourceChangeEvent was empty")
		return
	}

	log.Info(fmt.Sprintf("API Resource changed: %s", string(resourceChangeType)), "name", resourceName, "resource", resource)
}

func LogAPIResourceChangeEventFailure(log logr.Logger, resourceName string, resource string, resourceChangeType ResourceChangeType, err error) {
	log = log.WithValues("audit", "true")

	if resource == "" {
		log.Error(nil, "resource passed to LogAPIResourceChangeEventFailure was empty")
		return
	}

	log.Info(fmt.Sprintf("API Resource change event failed: %s, error: %v", string(resourceChangeType), err), "name", resourceName, "resource", resource)
}
