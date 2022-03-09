module github.com/redhat-appstudio/application-service

go 1.16

require (
	github.com/brianvoe/gofakeit/v6 v6.9.0
	github.com/devfile/api/v2 v2.0.0-20211018184408-84c44e563f58
	github.com/devfile/library v1.2.0
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-logr/logr v1.2.2
	github.com/google/go-cmp v0.5.7
	github.com/google/go-github/v41 v41.0.0
	github.com/migueleliasweb/go-github-mock v0.0.5
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/openshift/api v0.0.0-20200930075302-db52bc4ef99f
	github.com/redhat-appstudio/service-provider-integration-operator v0.2.4-0.20220305081755-23f3694d25f0
	github.com/spf13/afero v1.8.0
	github.com/stretchr/testify v1.7.0
	github.com/tektoncd/pipeline v0.33.2
	github.com/tektoncd/triggers v0.19.0
	golang.org/x/oauth2 v0.0.0-20220223155221-ee480838109b
	k8s.io/api v0.23.4
	k8s.io/apimachinery v0.23.4
	k8s.io/client-go v1.5.2
	sigs.k8s.io/controller-runtime v0.10.3
	sigs.k8s.io/yaml v1.3.0

)

replace (
	github.com/go-logr/logr => github.com/go-logr/logr v0.4.0
	k8s.io/api => k8s.io/api v0.21.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.4
	k8s.io/client-go => k8s.io/client-go v0.21.4
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.8.0
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.9.5
)
