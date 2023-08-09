/*
Copyright 2021-2023 Red Hat, Inc.

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

package controllers

import (
	"context"
	"errors"
	"reflect"
	"testing"

	gitopsjoblib "github.com/redhat-appstudio/application-service/gitops-generator/pkg/generate"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	data "github.com/devfile/library/v2/pkg/devfile/parser/data"
	v2 "github.com/devfile/library/v2/pkg/devfile/parser/data/v2"
	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/pkg/github"
	"github.com/redhat-appstudio/application-service/pkg/util/ioutils"
	"github.com/spf13/afero"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	devfileApi "github.com/devfile/api/v2/pkg/devfile"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//+kubebuilder:scaffold:imports
)

var kubernetesInlinedDevfile = `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- composite:
    commands:
    - build-image
    - deployk8s
    group:
      isDefault: true
      kind: deploy
    parallel: false
  id: deploy
components:
- image:
    autoBuild: false
    dockerfile:
      buildContext: .
      rootRequired: false
      uri: docker/Dockerfile
    imageName: java-springboot-image:latest
  name: image-build
- attributes:
    api.devfile.io/k8sLikeComponent-originalURI: deploy.yaml
    deployment/container-port: 5566
    deployment/containerENV:
    - name: FOO
      value: foo11
    - name: BAR
      value: bar11
    deployment/cpuLimit: "2"
    deployment/cpuRequest: 701m
    deployment/memoryLimit: 500Mi
    deployment/memoryRequest: 401Mi
    deployment/replicas: 5
    deployment/route: route111222
    deployment/storageLimit: 400Mi
    deployment/storageRequest: 201Mi
  kubernetes:
    deployByDefault: false
    endpoints:
    - name: http-8081
      path: /
      secure: false
      targetPort: 8081
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        creationTimestamp: null
        labels:
          app.kubernetes.io/created-by: application-service
          app.kubernetes.io/instance: component-sample
          app.kubernetes.io/managed-by: kustomize
          app.kubernetes.io/name: backend
          app.kubernetes.io/part-of: application-sample
          maysun: test
        name: deploy-sample
      spec:
        replicas: 1
        selector:
          matchLabels:
            app.kubernetes.io/instance: component-sample
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              app.kubernetes.io/instance: component-sample
          spec:
            containers:
            - env:
              - name: FOO
                value: foo1
              - name: BARBAR
                value: bar1
              image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              imagePullPolicy: Always
              livenessProbe:
                httpGet:
                  path: /
                  port: 1111
                initialDelaySeconds: 10
                periodSeconds: 10
              name: container-image
              ports:
              - containerPort: 1111
              readinessProbe:
                initialDelaySeconds: 10
                periodSeconds: 10
                tcpSocket:
                  port: 1111
              resources:
                limits:
                  cpu: "2"
                  memory: 500Mi
                  storage: 400Mi
                requests:
                  cpu: 700m
                  memory: 400Mi
                  storage: 200Mi
      status: {}
      ---
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        creationTimestamp: null
        labels:
          app.kubernetes.io/created-by: application-service
          app.kubernetes.io/instance: component-sample
          app.kubernetes.io/managed-by: kustomize
          app.kubernetes.io/name: backend
          app.kubernetes.io/part-of: application-sample
          maysun: test
        name: deploy-sample-2
      spec:
        replicas: 1
        selector:
          matchLabels:
            app.kubernetes.io/instance: component-sample
        strategy: {}
        template:
          metadata:
            creationTimestamp: null
            labels:
              app.kubernetes.io/instance: component-sample
          spec:
            containers:
            - env:
              - name: FOO
                value: foo1
              - name: BAR
                value: bar1
              image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              imagePullPolicy: Always
              livenessProbe:
                httpGet:
                  path: /
                  port: 1111
                initialDelaySeconds: 10
                periodSeconds: 10
              name: container-image
              ports:
              - containerPort: 1111
              readinessProbe:
                initialDelaySeconds: 10
                periodSeconds: 10
                tcpSocket:
                  port: 1111
              resources:
                limits:
                  cpu: "2"
                  memory: 500Mi
                  storage: 400Mi
                requests:
                  cpu: 700m
                  memory: 400Mi
                  storage: 200Mi
      status: {}
      ---
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        labels:
          app.kubernetes.io/created-by: application-service
          app.kubernetes.io/instance: component-sample
          app.kubernetes.io/managed-by: kustomize
          app.kubernetes.io/name: backend
          app.kubernetes.io/part-of: application-sample
          maysun: test
        name: service-sample
      spec:
        ports:
        - port: 1111
          targetPort: 1111
        selector:
          app.kubernetes.io/instance: component-sample
      status:
        loadBalancer: {}
      ---
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        labels:
          app.kubernetes.io/created-by: application-service
          app.kubernetes.io/instance: component-sample
          app.kubernetes.io/managed-by: kustomize
          app.kubernetes.io/name: backend
          app.kubernetes.io/part-of: application-sample
          maysun: test
        name: service-sample-2
      spec:
        ports:
        - port: 1111
          targetPort: 1111
        selector:
          app.kubernetes.io/instance: component-sample
      status:
        loadBalancer: {}
      ---
      apiVersion: route.openshift.io/v1
      kind: Route
      metadata:
        creationTimestamp: null
        labels:
          app.kubernetes.io/created-by: application-service
          app.kubernetes.io/instance: component-sample
          app.kubernetes.io/managed-by: kustomize
          app.kubernetes.io/name: backend
          app.kubernetes.io/part-of: application-sample
          maysun: test
        name: route-sample
      spec:
        host: route111
        port:
          targetPort: 1111
        tls:
          insecureEdgeTerminationPolicy: Redirect
          termination: edge
        to:
          kind: Service
          name: component-sample
          weight: 100
      status: {}
      ---
      apiVersion: route.openshift.io/v1
      kind: Route
      metadata:
        creationTimestamp: null
        labels:
          app.kubernetes.io/created-by: application-service
          app.kubernetes.io/instance: component-sample
          app.kubernetes.io/managed-by: kustomize
          app.kubernetes.io/name: backend
          app.kubernetes.io/part-of: application-sample
          maysun: test
        name: route-sample-2
      spec:
        host: route111
        port:
          targetPort: 1111
        tls:
          insecureEdgeTerminationPolicy: Redirect
          termination: edge
        to:
          kind: Service
          name: component-sample
          weight: 100
      status: {}
      ---
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      metadata:
        name: ingress-sample
        annotations:
          nginx.ingress.kubernetes.io/rewrite-target: /
          maysun: test
      spec:
        ingressClassName: nginx-example
        rules:
        - http:
            paths:
            - path: /testpath
              pathType: Prefix
              backend:
                service:
                  name: test
                  port:
                    number: 80
      ---
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      metadata:
        name: ingress-sample-2
        annotations:
          nginx.ingress.kubernetes.io/rewrite-target: /
          maysun: test
      spec:
        ingressClassName: nginx-example
        rules:
        - http:
            paths:
            - path: /testpath
              pathType: Prefix
              backend:
                service:
                  name: test
                  port:
                    number: 80
      ---
      apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: pvc-sample
        labels:
          maysun: test
      spec:
        accessModes:
          - ReadWriteOnce
        volumeMode: Filesystem
        resources:
          requests:
            storage: 8Gi
        storageClassName: slow
        selector:
          matchLabels:
            release: "stable"
          matchExpressions:
            - {key: environment, operator: In, values: [dev]}
      ---
      apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: pvc-sample-2
        labels:
          maysun: test
      spec:
        accessModes:
          - ReadWriteOnce
        volumeMode: Filesystem
        resources:
          requests:
            storage: 8Gi
        storageClassName: slow
        selector:
          matchLabels:
            release: "stable"
          matchExpressions:
            - {key: environment, operator: In, values: [dev]}
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

var noDeployDevfile = `
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

var invalidDevfile = `safdsfsdl32432423\n\t`

func TestSetGitOpsStatus(t *testing.T) {
	tests := []struct {
		name             string
		devfileData      *v2.DevfileV2
		component        appstudiov1alpha1.Component
		wantGitOpsStatus appstudiov1alpha1.GitOpsStatus
		wantErr          bool
	}{
		{
			name: "Simple application devfile, only gitops url",
			devfileData: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfileApi.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfileApi.DevfileMetadata{
							Name:       "petclinic",
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
						},
					},
				},
			},
			wantGitOpsStatus: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "https://github.com/testorg/petclinic-gitops",
			},
			wantErr: false,
		},
		{
			name: "Simple application devfile, no gitops fields",
			devfileData: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfileApi.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfileApi.DevfileMetadata{
							Name: "petclinic",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Application devfile, all gitops fields",
			devfileData: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfileApi.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfileApi.DevfileMetadata{
							Name:       "petclinic",
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("gitOpsRepository.branch", "main").PutString("gitOpsRepository.context", "/test"),
						},
					},
				},
			},
			wantGitOpsStatus: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "https://github.com/testorg/petclinic-gitops",
				Branch:        "main",
				Context:       "/test",
			},
			wantErr: false,
		},
		{
			name: "Application devfile, gitops branch with invalid value",
			devfileData: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfileApi.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfileApi.DevfileMetadata{
							Name:       "petclinic",
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").Put("gitOpsRepository.branch", appstudiov1alpha1.Component{}, nil),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Application devfile, gitops context with invalid value",
			devfileData: &v2.DevfileV2{
				Devfile: v1alpha2.Devfile{
					DevfileHeader: devfileApi.DevfileHeader{
						SchemaVersion: string(data.APISchemaVersion220),
						Metadata: devfileApi.DevfileMetadata{
							Name:       "petclinic",
							Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").Put("gitOpsRepository.context", appstudiov1alpha1.Component{}, nil),
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setGitopsStatus(&tt.component, tt.devfileData)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestSetGitOpsAnnotations() unexpected error: %v", err)
			}
			if !tt.wantErr {
				compGitOps := tt.component.Status.GitOps
				if !reflect.DeepEqual(compGitOps, tt.wantGitOpsStatus) {
					t.Errorf("TestSetGitOpsAnnotations() error: expected %v got %v", tt.wantGitOpsStatus, compGitOps)
				}
			}
		})
	}

}

func TestGenerateGitops(t *testing.T) {
	appFS := ioutils.NewMemoryFilesystem()
	readOnlyFs := ioutils.NewReadOnlyFs()
	ctx := context.Background()

	fakeClient := fake.NewClientBuilder().Build()

	r := &ComponentReconciler{
		Log:               ctrl.Log.WithName("controllers").WithName("Component"),
		GitHubOrg:         github.AppStudioAppDataOrg,
		Generator:         gitopsjoblib.NewMockGenerator(),
		Client:            fakeClient,
		GitHubTokenClient: github.MockGitHubTokenClient{},
	}

	// Create a second reconciler for testing error scenarios
	errGen := gitopsjoblib.NewMockGenerator()
	errGen.Errors.Push(errors.New("Fatal error"))
	errReconciler := &ComponentReconciler{
		Log:               ctrl.Log.WithName("controllers").WithName("Component"),
		GitHubOrg:         github.AppStudioAppDataOrg,
		Generator:         errGen,
		Client:            fakeClient,
		GitHubTokenClient: github.MockGitHubTokenClient{},
	}

	componentSpec := appstudiov1alpha1.ComponentSpec{
		ComponentName: "test-component",
		Application:   "test-app",
		Source: appstudiov1alpha1.ComponentSource{
			ComponentSourceUnion: appstudiov1alpha1.ComponentSourceUnion{
				GitSource: &appstudiov1alpha1.GitSource{
					URL: "git@github.com:testing/testing.git",
				},
			},
		},
	}

	tests := []struct {
		name       string
		reconciler *ComponentReconciler
		fs         afero.Afero
		component  *appstudiov1alpha1.Component
		devfile    string
		wantErr    bool
	}{
		{
			name:       "Simple application component, no errors",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
				},
				Spec: componentSpec,
				Status: appstudiov1alpha1.ComponentStatus{
					GitOps: appstudiov1alpha1.GitOpsStatus{
						RepositoryURL: "https://github.com/test/repo",
						Branch:        "main",
						Context:       "/test",
					},
				},
			},
			devfile: kubernetesInlinedDevfile,
			wantErr: false,
		},
		{
			name:       "Simple application component invalid devfile",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
				},
				Spec: componentSpec,
				Status: appstudiov1alpha1.ComponentStatus{
					GitOps: appstudiov1alpha1.GitOpsStatus{
						RepositoryURL: "https://github.com/test/repo",
						Branch:        "main",
						Context:       "/test",
					},
				},
			},
			devfile: invalidDevfile,
			wantErr: true,
		},
		{
			name:       "Simple application component no outerloop deploy",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
				},
				Spec: componentSpec,
				Status: appstudiov1alpha1.ComponentStatus{
					GitOps: appstudiov1alpha1.GitOpsStatus{
						RepositoryURL: "https://github.com/test/repo",
						Branch:        "main",
						Context:       "/test",
					},
				},
			},
			devfile: noDeployDevfile,
			wantErr: true,
		},
		{
			name:       "Invalid application component, no labels",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-component",
					Namespace:   "test-namespace",
					Annotations: nil,
				},
				Spec: componentSpec,
			},
			wantErr: true,
		},
		{
			name:       "Invalid application component, no gitops URL",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						"fake": "fake",
					},
				},
				Spec: componentSpec,
			},
			wantErr: true,
		},
		{
			name:       "Invalid application component, invalid gitops url",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						"gitOpsRepository.url": "dsfdsf sdfsdf sdk;;;fsd ppz mne@ddsfj#$*(%",
					},
				},
				Spec: componentSpec,
			},
			wantErr: true,
		},
		{
			name:       "Application component, only gitops URL set",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
				},
				Spec: componentSpec,
				Status: appstudiov1alpha1.ComponentStatus{
					GitOps: appstudiov1alpha1.GitOpsStatus{
						RepositoryURL: "https://github.com/test/repo",
					},
				},
			},
			devfile: kubernetesInlinedDevfile,
			wantErr: false,
		},
		{
			name:       "Gitops generation fails",
			reconciler: errReconciler,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
				},
				Spec: componentSpec,
				Status: appstudiov1alpha1.ComponentStatus{
					GitOps: appstudiov1alpha1.GitOpsStatus{
						RepositoryURL: "https://github.com/test/repo",
					},
				},
			},
			wantErr: true,
		},
		{
			name:       "Fail to create temp folder",
			reconciler: errReconciler,
			fs:         readOnlyFs,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-component",
					Namespace: "test-namespace",
				},
				Spec: componentSpec,
				Status: appstudiov1alpha1.ComponentStatus{
					GitOps: appstudiov1alpha1.GitOpsStatus{
						RepositoryURL: "https://github.com/test/repo",
					},
				},
			},
			wantErr: true,
		},
		{
			name:       "Fail to retrieve commit ID for GitOps repository [Mock]",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-git-error",
					Namespace: "test-namespace",
				},
				Spec: componentSpec,
				Status: appstudiov1alpha1.ComponentStatus{
					GitOps: appstudiov1alpha1.GitOpsStatus{
						RepositoryURL: "https://github.com/test/test-error-response",
					},
				},
			},
			wantErr: true,
		},
		{
			name:       "Fail to retrieve commit ID for GitOps repository with invalid repo [Mock]",
			reconciler: r,
			fs:         appFS,
			component: &appstudiov1alpha1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Component",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-git-error",
					Namespace: "test-namespace",
				},
				Spec: componentSpec,
				Status: appstudiov1alpha1.ComponentStatus{
					GitOps: appstudiov1alpha1.GitOpsStatus{
						RepositoryURL: "https://github.com///",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt.reconciler.AppFS = tt.fs
		t.Run(tt.name, func(t *testing.T) {

			tt.component.Status.Devfile = string(tt.devfile)
			mockedClient := &github.GitHubClient{
				Client:    github.GetMockedClient(),
				TokenName: "some-token",
			}
			err := tt.reconciler.generateGitops(ctx, mockedClient, tt.component)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestGenerateGitops() unexpected error: %v", err)
			}
		})
	}

}
