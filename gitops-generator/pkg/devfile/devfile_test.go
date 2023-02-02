//
// Copyright 2021-2023 Red Hat, Inc.
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

package devfile

import (
	"testing"

	devfileParser "github.com/devfile/library/v2/pkg/devfile/parser"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetRouteFromEndpoint(t *testing.T) {

	var (
		name        = "route1"
		serviceName = "service1"
		path        = ""
		port        = "1234"
		secure      = true
		annotations = map[string]string{
			"key1": "value1",
		}
	)
	t.Run(name, func(t *testing.T) {
		actualRoute := GetRouteFromEndpoint(name, serviceName, port, path, secure, annotations)
		assert.Equal(t, "Route", actualRoute.Kind, "Kind did not match")
		assert.Equal(t, "route.openshift.io/v1", actualRoute.APIVersion, "APIVersion did not match")
		assert.Equal(t, name, actualRoute.Name, "Route name did not match")
		assert.Equal(t, "/", actualRoute.Spec.Path, "Route path did not match")
		assert.NotNil(t, actualRoute.Spec.Port, "Route Port should not be nil")
		assert.Equal(t, intstr.FromString(port), actualRoute.Spec.Port.TargetPort, "Route port did not match")
		assert.NotNil(t, actualRoute.Spec.TLS, "Route TLS should not be nil")
		assert.Equal(t, routev1.TLSTerminationEdge, actualRoute.Spec.TLS.Termination, "Route port did not match")
		actualRouteAnnotations := actualRoute.GetAnnotations()
		assert.NotEmpty(t, actualRouteAnnotations, "Route annotations should not be empty")
		assert.Equal(t, "value1", actualRouteAnnotations["key1"], "Route annotation did not match")
	})
}

func TestGetResourceFromDevfile(t *testing.T) {

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

	kubernetesInlinedDevfileSvc := `
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
  kubernetes:
    deployByDefault: false
    inlined: |-
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
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileDeploy := `
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
              - name: FOOFOO
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
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileRouteHostMissing := `
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
              - name: FOOFOO
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
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesWithoutInline := `
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
  kubernetes:
    deployByDefault: false
    uri: uri
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileErrCase_BadMemoryLimit := `
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
    deployment/memoryLimit: abc
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        labels:
          maysun: test
        name: deploy-sample
      spec:
        template:
          spec:
            containers:
            - image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              name: container-image
  name: kubernetes-deploy
metadata:
  language: Java
  name: java-springboot
  projectType: springboot
  version: 1.2.1
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileErrCase_BadStorageLimit := `
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
    deployment/storageLimit: abc
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        labels:
          maysun: test
        name: deploy-sample
      spec:
        template:
          spec:
            containers:
            - image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              name: container-image
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileErrCase_BadCPULimit := `
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
    deployment/cpuLimit: "abc"
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        labels:
          maysun: test
        name: deploy-sample
      spec:
        template:
          spec:
            containers:
            - image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              name: container-image
  name: kubernetes-deploy
metadata:
  language: Java
  name: java-springboot
  projectType: springboot
  version: 1.2.1
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileErrCase_BadMemoryRequest := `
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
    deployment/memoryRequest: abc
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        labels:
          maysun: test
        name: deploy-sample
      spec:
        template:
          spec:
            containers:
            - image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              name: container-image
  name: kubernetes-deploy
metadata:
  language: Java
  name: java-springboot
  projectType: springboot
  version: 1.2.1
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileErrCase_BadStorageRequest := `
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
    deployment/storageRequest: abc
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        labels:
          maysun: test
        name: deploy-sample
      spec:
        template:
          spec:
            containers:
            - image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              name: container-image
  name: kubernetes-deploy
metadata:
  language: Java
  name: java-springboot
  projectType: springboot
  version: 1.2.1
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileErrCase_BadCPUrequest := `
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
    deployment/cpuRequest: "abc"
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        labels:
          maysun: test
        name: deploy-sample
      spec:
        template:
          spec:
            containers:
            - image: quay.io/redhat-appstudio/user-workload:application-service-system-component-sample
              name: container-image
  name: kubernetes-deploy
metadata:
  language: Java
  name: java-springboot
  projectType: springboot
  version: 1.2.1
schemaVersion: 2.2.0`

	replica := int32(5)
	replicaUpdated := int32(1)

	tests := []struct {
		name          string
		devfileString string
		componentName string
		image         string
		wantDeploy    appsv1.Deployment
		wantService   corev1.Service
		wantRoute     routev1.Route
		wantErr       bool
	}{
		{
			name:          "Simple devfile from Inline",
			devfileString: kubernetesInlinedDevfile,
			componentName: "component-sample",
			image:         "image1",
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "deploy-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "backend",
						"app.kubernetes.io/part-of":    "application-sample",
						"maysun":                       "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replica,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/instance": "component-sample",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/instance": "component-sample",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "container-image",
									Env: []corev1.EnvVar{
										{
											Name:  "FOO",
											Value: "foo11",
										},
										{
											Name:  "BARBAR",
											Value: "bar1",
										},
										{
											Name:  "BAR",
											Value: "bar11",
										},
									},
									Image:           "image1",
									ImagePullPolicy: corev1.PullAlways,
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/",
												Port: intstr.FromInt(5566),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: int32(1111),
										},
										{
											ContainerPort: int32(5566),
										},
									},
									ReadinessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											TCPSocket: &corev1.TCPSocketAction{
												Port: intstr.FromInt(5566),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("2"),
											corev1.ResourceMemory:  resource.MustParse("500Mi"),
											corev1.ResourceStorage: resource.MustParse("400Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("701m"),
											corev1.ResourceMemory:  resource.MustParse("401Mi"),
											corev1.ResourceStorage: resource.MustParse("201Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantService: corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "service-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "backend",
						"app.kubernetes.io/part-of":    "application-sample",
						"maysun":                       "test",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:       int32(1111),
							TargetPort: intstr.FromInt(1111),
						},
						{
							Port:       int32(5566),
							TargetPort: intstr.FromInt(5566),
						},
					},
					Selector: map[string]string{
						"app.kubernetes.io/instance": "component-sample",
					},
				},
			},
			wantRoute: routev1.Route{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "http-8081",
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					Host: "route111222",
					Path: "/",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(5566),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "component-sample",
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with only Svc",
			devfileString: kubernetesInlinedDevfileSvc,
			componentName: "component-sample",
			image:         "image1",
			wantService: corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "service-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "backend",
						"app.kubernetes.io/part-of":    "application-sample",
						"maysun":                       "test",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:       int32(1111),
							TargetPort: intstr.FromInt(1111),
						},
					},
					Selector: map[string]string{
						"app.kubernetes.io/instance": "component-sample",
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with Deploy",
			devfileString: kubernetesInlinedDevfileDeploy,
			componentName: "component-sample",
			image:         "image1",
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "deploy-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "backend",
						"app.kubernetes.io/part-of":    "application-sample",
						"maysun":                       "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicaUpdated,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/instance": "component-sample",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/instance": "component-sample",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "container-image",
									Env: []corev1.EnvVar{
										{
											Name:  "FOOFOO",
											Value: "foo1",
										},
										{
											Name:  "BARBAR",
											Value: "bar1",
										},
									},
									Image:           "image1",
									ImagePullPolicy: corev1.PullAlways,
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/",
												Port: intstr.FromInt(1111),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: int32(1111),
										},
									},
									ReadinessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											TCPSocket: &corev1.TCPSocketAction{
												Port: intstr.FromInt(1111),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("2"),
											corev1.ResourceMemory:  resource.MustParse("500Mi"),
											corev1.ResourceStorage: resource.MustParse("400Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("700m"),
											corev1.ResourceMemory:  resource.MustParse("400Mi"),
											corev1.ResourceStorage: resource.MustParse("200Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantRoute: routev1.Route{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "http-8081",
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					Path: "/",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromString("8081"),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "component-sample",
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with Route Host missing",
			devfileString: kubernetesInlinedDevfileRouteHostMissing,
			componentName: "component-sample",
			image:         "image1",
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "deploy-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "backend",
						"app.kubernetes.io/part-of":    "application-sample",
						"maysun":                       "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicaUpdated,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/instance": "component-sample",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/instance": "component-sample",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "container-image",
									Env: []corev1.EnvVar{
										{
											Name:  "FOOFOO",
											Value: "foo1",
										},
										{
											Name:  "BARBAR",
											Value: "bar1",
										},
									},
									Image:           "image1",
									ImagePullPolicy: corev1.PullAlways,
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/",
												Port: intstr.FromInt(5566),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: int32(1111),
										},
										{
											ContainerPort: int32(5566),
										},
									},
									ReadinessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											TCPSocket: &corev1.TCPSocketAction{
												Port: intstr.FromInt(5566),
											},
										},
										InitialDelaySeconds: int32(10),
										PeriodSeconds:       int32(10),
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("2"),
											corev1.ResourceMemory:  resource.MustParse("500Mi"),
											corev1.ResourceStorage: resource.MustParse("400Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("700m"),
											corev1.ResourceMemory:  resource.MustParse("400Mi"),
											corev1.ResourceStorage: resource.MustParse("200Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantRoute: routev1.Route{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "http-8081",
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					Path: "/",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(5566),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "component-sample",
					},
				},
			},
		},
		{
			name:          "Simple devfile without inline",
			devfileString: kubernetesWithoutInline,
			componentName: "component-sample",
			image:         "image1",
		},
		{
			name:          "Bad Memory Limit",
			devfileString: kubernetesInlinedDevfileErrCase_BadMemoryLimit,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
		},
		{
			name:          "Bad Storage Limit",
			devfileString: kubernetesInlinedDevfileErrCase_BadStorageLimit,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
		},
		{
			name:          "Bad CPU Limit",
			devfileString: kubernetesInlinedDevfileErrCase_BadCPULimit,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
		},
		{
			name:          "Bad Memory Request",
			devfileString: kubernetesInlinedDevfileErrCase_BadMemoryRequest,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
		},
		{
			name:          "Bad Storage Request",
			devfileString: kubernetesInlinedDevfileErrCase_BadStorageRequest,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
		},
		{
			name:          "Bad CPU Request",
			devfileString: kubernetesInlinedDevfileErrCase_BadCPUrequest,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var devfileSrc DevfileSrc
			if tt.devfileString != "" {
				devfileSrc = DevfileSrc{
					Data: tt.devfileString,
				}
			}

			devfileData, err := ParseDevfile(devfileSrc)
			if err != nil {
				t.Errorf("TestGetResourceFromDevfile() unexpected parse error: %v", err)
			}
			deployAssociatedComponents, err := devfileParser.GetDeployComponents(devfileData)
			if err != nil {
				t.Errorf("TestGetResourceFromDevfile() unexpected get deploy components error: %v", err)
			}
			//logger := ctrl.Log.WithName("TestGetResourceFromDevfile")

			actualResources, err := GetResourceFromDevfile(devfileData, deployAssociatedComponents, tt.componentName, tt.image)
			if tt.wantErr && (err == nil) {
				t.Error("wanted error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("TestGetResourceFromDevfile() unexpected get resource from devfile error: %v", err)
			} else if err == nil {
				if len(actualResources.Deployments) > 0 {
					assert.Equal(t, tt.wantDeploy, actualResources.Deployments[0], "First Deployment did not match")
				}

				if len(actualResources.Services) > 0 {
					assert.Equal(t, tt.wantService, actualResources.Services[0], "First Service did not match")
				}

				if len(actualResources.Routes) > 0 {
					assert.Equal(t, tt.wantRoute, actualResources.Routes[0], "First Route did not match")
				}
			}
		})
	}
}
