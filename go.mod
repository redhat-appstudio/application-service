module github.com/redhat-appstudio/application-service

go 1.16

require (
	github.com/brianvoe/gofakeit/v6 v6.9.0
	github.com/devfile/api/v2 v2.0.0-20211021164004-dabee4e633ed
	github.com/devfile/library v1.2.1-0.20211104222135-49d635cb492f
	github.com/devfile/registry-support/index/generator v0.0.0-20220222194908-7a90a4214f3e
	github.com/devfile/registry-support/registry-library v0.0.0-20220222194908-7a90a4214f3e
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-logr/logr v1.2.3
	github.com/google/go-github/v41 v41.0.0
	github.com/kcp-dev/apimachinery v0.0.0-20220627134323-8c44889e6e09
	github.com/kcp-dev/kcp/pkg/apis v0.5.0-alpha.1
	github.com/kcp-dev/logicalcluster v1.0.0
	github.com/migueleliasweb/go-github-mock v0.0.10
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.19.0
	github.com/openshift-pipelines/pipelines-as-code v0.0.0-20220622161720-2a6007e17200
	github.com/openshift/api v0.0.0-20210503193030-25175d9d392d
	github.com/redhat-appstudio/managed-gitops/appstudio-shared v0.0.0-20220826075641-33705d2bf7fa // Update mod version in suite_test.go for tests
	github.com/redhat-appstudio/service-provider-integration-scm-file-retriever v0.6.10
	github.com/redhat-developer/alizer/go v0.0.0-20220704150640-ef50ead0b279
	github.com/redhat-developer/gitops-generator v0.0.0-20220831163837-f095cbb71b42
	github.com/spf13/afero v1.8.0
	github.com/stretchr/testify v1.8.0
	github.com/tektoncd/pipeline v0.33.0
	github.com/tektoncd/triggers v0.19.1
	go.uber.org/zap v1.21.0
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/client-go v0.23.5
	sigs.k8s.io/controller-runtime v0.11.2
	sigs.k8s.io/yaml v1.3.0

)

replace github.com/antlr/antlr4 => github.com/antlr/antlr4 v0.0.0-20211106181442-e4c1a74c66bd

replace sigs.k8s.io/controller-runtime => github.com/kcp-dev/controller-runtime v0.11.3-0.20220624161137-f6e5a2f56683
