module github.com/redhat-appstudio/application-service/gitops-generator

go 1.19

require (
	github.com/brianvoe/gofakeit/v6 v6.9.0
	github.com/devfile/api/v2 v2.2.1-alpha.0.20230413012049-a6c32fca0dbd
	github.com/devfile/library/v2 v2.2.1-0.20230418160146-e75481b7eebd
	github.com/go-logr/logr v1.2.3
	github.com/gofri/go-github-ratelimit v1.0.3-0.20230428184158-a500e14de53f
	github.com/golang/mock v1.6.0
	github.com/google/go-github/v52 v52.0.1-0.20230514113659-60429b4ba0ba
	github.com/migueleliasweb/go-github-mock v0.0.17
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.24.1
	github.com/openshift-pipelines/pipelines-as-code v0.0.0-20220622161720-2a6007e17200
	github.com/openshift/api v0.0.0-20210503193030-25175d9d392d
	github.com/pact-foundation/pact-go v1.7.0
	github.com/prometheus/client_golang v1.14.0
	github.com/redhat-appstudio/application-api v0.0.0-20230704143842-035c661f115f
	github.com/redhat-appstudio/application-service/cdq-analysis v0.0.0
	github.com/redhat-appstudio/service-provider-integration-scm-file-retriever v0.8.3
	github.com/redhat-developer/gitops-generator v0.0.0-20230614175323-aff86c6bc55e
	github.com/spf13/afero v1.8.0
	github.com/stretchr/testify v1.8.1
	go.uber.org/zap v1.24.0
	golang.org/x/exp v0.0.0-20230206171751-46f607a40771
	golang.org/x/oauth2 v0.7.0
	k8s.io/api v0.26.1
	k8s.io/apimachinery v0.26.1
	k8s.io/client-go v0.26.1
	sigs.k8s.io/controller-runtime v0.14.4
	sigs.k8s.io/yaml v1.3.0

)