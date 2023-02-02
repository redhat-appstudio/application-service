//
// Copyright 2023 Red Hat, Inc.
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

package generate

import (
	"context"
	"errors"
	"testing"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/application-service/gitops-generator/pkg/gitops"
	"github.com/redhat-developer/gitops-generator/pkg/util/ioutils"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGenerateGitopsBase(t *testing.T) {
	kubernetesInlinedDevfile := `
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

	appFS := ioutils.NewMemoryFilesystem()
	readOnlyFs := ioutils.NewReadOnlyFs()
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()

	errGen := gitops.NewMockGenerator()
	errGen.Errors.Push(errors.New("Fatal error"))

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
		name         string
		fs           afero.Afero
		component    *appstudiov1alpha1.Component
		gitopsParams GitOpsGenParams
		wantErr      bool
	}{
		{
			name: "Simple application component, no errors",
			fs:   appFS,
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
					Devfile: kubernetesInlinedDevfile,
				},
			},
			gitopsParams: GitOpsGenParams{
				Generator: gitops.NewMockGenerator(),
			},
			wantErr: false,
		},
		{
			name: "Simple application component - missing devfile",
			fs:   appFS,
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
			gitopsParams: GitOpsGenParams{
				Generator: gitops.NewMockGenerator(),
			},
			wantErr: true,
		},
		{
			name: "Generation error, Read only file system",
			fs:   readOnlyFs,
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
					Devfile: kubernetesInlinedDevfile,
				},
			},
			gitopsParams: GitOpsGenParams{
				Generator: gitops.NewMockGenerator(),
			},
			wantErr: true,
		},
		{
			name: "Generation error",
			fs:   appFS,
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
					Devfile: kubernetesInlinedDevfile,
				},
			},
			gitopsParams: GitOpsGenParams{
				Generator: errGen,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GenerateGitopsBase(ctx, fakeClient, *tt.component, tt.fs, tt.gitopsParams)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestGenerateGitops() unexpected error: %v", err)
			}
		})
	}
}

func TestGenerateGitopsOverlays(t *testing.T) {
	appFS := ioutils.NewMemoryFilesystem()
	readOnlyFs := ioutils.NewReadOnlyFs()
	ctx := context.Background()
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	appstudiov1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	errGen := gitops.NewMockGenerator()
	errGen.Errors.Push(errors.New("Fatal error"))

	// Before the test runs, make sure that Application, Component and associated resources all exist
	setUpResources(t, &fakeClient, ctx)
	newComponent := appstudiov1alpha1.Component{}
	err := fakeClient.Get(ctx, types.NamespacedName{Namespace: "test-namespace", Name: "test-component"}, &newComponent)
	if err != nil {
		t.Error(err)
	}

	// After the prerequisite resources have been set up, make sure it exists
	snapshotEnvironmentBinding := appstudiov1alpha1.SnapshotEnvironmentBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "SnapshotEnvironmentBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-seb",
			Namespace: "test-namespace",
		},
		Spec: appstudiov1alpha1.SnapshotEnvironmentBindingSpec{
			Application: "test-application",
			Environment: "test-environment",
			Snapshot:    "test-snapshot",
			Components: []appstudiov1alpha1.BindingComponent{
				{
					Name: "test-component",
					Configuration: appstudiov1alpha1.BindingComponentConfiguration{
						Env: []appstudiov1alpha1.EnvVarPair{
							{
								Name:  "FOO",
								Value: "BAR",
							},
						},
					},
				},
			},
		},
	}
	err = fakeClient.Create(ctx, &snapshotEnvironmentBinding)
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		name         string
		fs           afero.Afero
		seb          *appstudiov1alpha1.SnapshotEnvironmentBinding
		gitopsParams GitOpsGenParams
		wantErr      bool
	}{
		{
			name: "Gitops generation succeeds",
			fs:   appFS,
			seb:  &snapshotEnvironmentBinding,
			gitopsParams: GitOpsGenParams{
				Generator: gitops.NewMockGenerator(),
			},
		},
		{
			name: "Gitops generation succeeds - seb doesn't exist",
			fs:   appFS,
			seb: &appstudiov1alpha1.SnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "SnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake-seb",
					Namespace: "fake-namespace",
				},
			},
			gitopsParams: GitOpsGenParams{
				Generator: gitops.NewMockGenerator(),
			},
			wantErr: true,
		},
		{
			name: "Gitops generation error - file system error",
			fs:   readOnlyFs,
			seb:  &snapshotEnvironmentBinding,
			gitopsParams: GitOpsGenParams{
				Generator: gitops.NewMockGenerator(),
			},
			wantErr: true,
		},
		{
			name: "Gitops generation error - application doesn't exist",
			fs:   appFS,
			seb: &appstudiov1alpha1.SnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "SnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake-seb",
					Namespace: "test-namespace",
				},
				Spec: appstudiov1alpha1.SnapshotEnvironmentBindingSpec{
					Application: "app-that-doesnt-exist",
					Environment: "test-environment",
					Snapshot:    "test-snapshot",
				},
			},
			gitopsParams: GitOpsGenParams{
				Generator: gitops.NewMockGenerator(),
			},
			wantErr: true,
		},
		{
			name: "Gitops generation error - environment doesn't exist",
			fs:   appFS,
			seb: &appstudiov1alpha1.SnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "SnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake-seb",
					Namespace: "test-namespace",
				},
				Spec: appstudiov1alpha1.SnapshotEnvironmentBindingSpec{
					Application: "app-that-doesnt-exist",
					Snapshot:    "test-snapshot",
				},
			},
			gitopsParams: GitOpsGenParams{
				Generator: gitops.NewMockGenerator(),
			},
			wantErr: true,
		},
		{
			name: "Gitops generation error - snapshot doesn't exist",
			fs:   appFS,
			seb: &appstudiov1alpha1.SnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "SnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake-seb",
					Namespace: "test-namespace",
				},
				Spec: appstudiov1alpha1.SnapshotEnvironmentBindingSpec{
					Application: "app-that-doesnt-exist",
					Environment: "test-environment",
				},
			},
			gitopsParams: GitOpsGenParams{
				Generator: gitops.NewMockGenerator(),
			},
			wantErr: true,
		},
		{
			name: "Gitops generation error - snapshot doesn't exist",
			fs:   appFS,
			seb: &appstudiov1alpha1.SnapshotEnvironmentBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "SnapshotEnvironmentBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake-seb",
					Namespace: "test-namespace",
				},
				Spec: appstudiov1alpha1.SnapshotEnvironmentBindingSpec{
					Application: "test-application",
					Environment: "test-environment",
					Snapshot:    "test-snapshot",
					Components: []appstudiov1alpha1.BindingComponent{
						{
							Name: "non-existent-component",
						},
					},
				},
			},
			gitopsParams: GitOpsGenParams{
				Generator: gitops.NewMockGenerator(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GenerateGitopsOverlays(ctx, fakeClient, *tt.seb, tt.fs, tt.gitopsParams)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestGenerateGitops() unexpected error: %v", err)
			}
		})
	}
}

// setUpResources sets up the necessary Kubernetes resources for the TestGenerateGitopsOverlays test
// The following resources need to be created before the test can be run:
// Component, Environment, Snapshot, SnapshotEnvironmentBinding
func setUpResources(t *testing.T, client *client.WithWatch, ctx context.Context) {
	// Create the Component
	kubeClient := *client
	component := appstudiov1alpha1.Component{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Component",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-component",
			Namespace: "test-namespace",
		},
		Spec: appstudiov1alpha1.ComponentSpec{
			ComponentName: "test-component",
			Application:   "test-application",
		},
		Status: appstudiov1alpha1.ComponentStatus{
			GitOps: appstudiov1alpha1.GitOpsStatus{
				RepositoryURL: "https://github.com/testorg/repo",
				Branch:        "main",
				Context:       "/",
			},
		},
	}
	err := kubeClient.Create(ctx, &component)
	if err != nil {
		t.Error(err)
	}

	// Create the Environment
	environment := appstudiov1alpha1.Environment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Environment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-environment",
			Namespace: "test-namespace",
		},
		Spec: appstudiov1alpha1.EnvironmentSpec{
			Type:               appstudiov1alpha1.EnvironmentType_POC,
			DisplayName:        "Staging Environment",
			DeploymentStrategy: appstudiov1alpha1.DeploymentStrategy_AppStudioAutomated,
			Configuration: appstudiov1alpha1.EnvironmentConfiguration{
				Env: []appstudiov1alpha1.EnvVarPair{
					{
						Name:  "Test",
						Value: "Value",
					},
				},
			},
		},
	}
	err = kubeClient.Create(ctx, &environment)
	if err != nil {
		t.Error(err)
	}

	// Create the Snapshot
	snapshot := appstudiov1alpha1.Snapshot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appstudio.redhat.com/v1alpha1",
			Kind:       "Snapshot",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-snapshot",
			Namespace: "test-namespace",
		},
		Spec: appstudiov1alpha1.SnapshotSpec{
			Application:        "test-application",
			DisplayName:        "Test Snapshot",
			DisplayDescription: "My First Snapshot",
			Components: []appstudiov1alpha1.SnapshotComponent{
				{
					Name:           "test-component",
					ContainerImage: "quay.io/redhat-appstudio/user-workload:application-service-system-test-component",
				},
			},
		},
	}
	err = kubeClient.Create(ctx, &snapshot)
	if err != nil {
		t.Error(err)
	}
}
