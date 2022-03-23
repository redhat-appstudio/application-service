module github.com/redhat-appstudio/application-service

go 1.16

require (
	github.com/google/uuid v1.3.0
	github.com/onsi/ginkgo/v2 v2.1.3
	github.com/onsi/gomega v1.18.1
	github.com/openshift/api v0.0.0-20200930075302-db52bc4ef99f
	github.com/redhat-appstudio/application-service v0.0.0-20220312031926-2976522a9052
	github.com/tektoncd/pipeline v0.32.1
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/client-go v0.22.5
	k8s.io/klog/v2 v2.9.0
	sigs.k8s.io/controller-runtime v0.10.3
)
