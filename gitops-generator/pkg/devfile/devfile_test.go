package devfile

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/api/v2/pkg/devfile"
	"github.com/devfile/library/pkg/devfile/parser/data"
	v2 "github.com/devfile/library/pkg/devfile/parser/data/v2"
	parser "github.com/devfile/library/v2/pkg/devfile/parser"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/stretchr/testify/assert"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
)

func TestGetIngressHostName(t *testing.T) {

	tests := []struct {
		name          string
		componentName string
		namespace     string
		ingressDomain string
		wantHostName  string
		wantErr       bool
	}{
		{
			name:          "all string present",
			componentName: "my-component",
			namespace:     "test",
			ingressDomain: "domain.example.com",
			wantHostName:  "my-component-test.domain.example.com",
		},
		{
			name:          "Capitalized component name should be ok",
			componentName: "my-Component",
			namespace:     "test",
			ingressDomain: "domain.example.com",
			wantHostName:  "my-Component-test.domain.example.com",
		},
		{
			name:          "invalid char in string",
			componentName: "&",
			namespace:     "$",
			ingressDomain: "$",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			gotHostName, err := GetIngressHostName(tt.componentName, tt.namespace, tt.ingressDomain)
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected err: %+v", err)
			} else if tt.wantErr && err == nil {
				t.Errorf("Expected error but got nil")
			} else if !reflect.DeepEqual(tt.wantHostName, gotHostName) {
				t.Errorf("Expected: %+v, \nGot: %+v", tt.wantHostName, gotHostName)
			}
		})
	}
}

func TestGetResourceFromDevfile(t *testing.T) {

	weight := int32(100)

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
          maysun: test
        name: deploy-sample
      spec:
        replicas: 1
        selector: {}
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
                requests:
                  cpu: 700m
                  memory: 400Mi
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

	kubernetesInlinedDevfileIngress := `
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
    inlined: |-
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      metadata:
        name: ingress-sample
        labels:
          test: test
        annotations:
          nginx.ingress.kubernetes.io/rewrite-target: /
          test: yes
      spec:
        rules:
        - host: "foo.bar.com"
          http:
            paths:
            - path: /testpath
              pathType: ImplementationSpecific
              backend:
                service:
                  name: test
                  port:
                    number: 80
      status: {}
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileRoute := `
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
    inlined: |-
      apiVersion: route.openshift.io/v1
      kind: Route
      metadata:
        creationTimestamp: null
        name: route-sample-2
        labels:
          test: test
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
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileRouteNoTargetPort := `
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
    inlined: |-
      apiVersion: route.openshift.io/v1
      kind: Route
      metadata:
        creationTimestamp: null
        name: route-sample-2
        labels:
          test: test
      spec:
        host: route111
        tls:
          insecureEdgeTerminationPolicy: Redirect
          termination: edge
        to:
          kind: Service
          name: component-sample
          weight: 100
      status: {}
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
    deployment/container-port: 1111
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        name: service-sample
      spec:
        ports:
        - port: 1111
          targetPort: 1111
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
    deployment/container-port: 1111
    deployment/storageLimit: 401Mi
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

	kubernetesInlinedDevfileSeparatedKubeComps := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- apply:
    component: kubernetes-svc
  id: svck8s
- composite:
    commands:
    - build-image
    - deployk8s
    - svck8s
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
    deployment/container-port: 1111
    deployment/storageLimit: 401Mi
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
- attributes:
    deployment/container-port: 1111
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        name: service-sample
      spec:
        ports:
        - port: 1111
          targetPort: 1111
      status:
        loadBalancer: {}
  name: kubernetes-svc
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesInlinedDevfileSeparatedKubeCompsRevHistory := `
commands:
- apply:
    component: image-build
  id: build-image
- apply:
    component: kubernetes-deploy
  id: deployk8s
- apply:
    component: kubernetes-svc
  id: svck8s
- composite:
    commands:
    - build-image
    - deployk8s
    - svck8s
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
    deployment/container-port: 1111
    deployment/storageLimit: 401Mi
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
        name: deploy-sample
      spec:
        revisionHistoryLimit: 5
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
- attributes:
    deployment/container-port: 1111
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        name: service-sample
      spec:
        ports:
        - port: 1111
          targetPort: 1111
      status:
        loadBalancer: {}
  name: kubernetes-svc
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
- kubernetes:
    deployByDefault: false
    uri: ""
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

	noKubernetesCompDevfile := `
commands:
- apply:
    component: image-build
  id: build-image
- composite:
    commands:
    - build-image
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
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	multipleKubernetesCompsDevfile := `
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
    deployment/container-port: 1111
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        name: service-sample
      spec:
        ports:
        - port: 1111
          targetPort: 1111
      status:
        loadBalancer: {}
  name: kubernetes-deploy
- attributes:
    deployment/container-port: 1111
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        name: service-sample2
      spec:
        ports:
        - port: 1111
          targetPort: 1111
      status:
        loadBalancer: {}
  name: kubernetes-deploy2
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	kubernetesCompsWithNoDeployCmdDevfile := `
commands:
- apply:
    component: image-build
  id: build-image
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
    deployment/container-port: 1111
  kubernetes:
    deployByDefault: false
    inlined: |-
      apiVersion: v1
      kind: Service
      metadata:
        creationTimestamp: null
        name: service-sample
      spec:
        ports:
        - port: 1111
          targetPort: 1111
      status:
        loadBalancer: {}
  name: kubernetes-deploy
metadata:
  name: java-springboot
schemaVersion: 2.2.0`

	replica := int32(5)
	replicaUpdated := int32(1)
	revHistoryLimit := int32(0)
	setRevHistoryLimit := int32(5)

	host := "host.example.com"
	implementationSpecific := networkingv1.PathTypeImplementationSpecific

	tests := []struct {
		name          string
		devfileString string
		componentName string
		appName       string
		image         string
		hostname      string
		wantDeploy    appsv1.Deployment
		wantService   corev1.Service
		wantRoute     routev1.Route
		wantIngress   networkingv1.Ingress
		wantErr       bool
	}{
		{
			name:          "Simple devfile from Inline",
			devfileString: kubernetesInlinedDevfile,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			hostname:      host,
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
						"maysun":                       "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: &revHistoryLimit,
					Replicas:             &replica,
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
											corev1.ResourceCPU:    resource.MustParse("2"),
											corev1.ResourceMemory: resource.MustParse("500Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("701m"),
											corev1.ResourceMemory: resource.MustParse("401Mi"),
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
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
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
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/",
											PathType: &implementationSpecific,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "component-sample",
													Port: networkingv1.ServiceBackendPort{
														Number: 5566,
													},
												},
											},
										},
									},
								},
							},
							Host: host,
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
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
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
			name:          "Simple devfile from Inline with deployment and svc from separated kube components",
			devfileString: kubernetesInlinedDevfileSeparatedKubeComps,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			hostname:      host,
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: &revHistoryLimit,
					Replicas:             &replicaUpdated,
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
											corev1.ResourceStorage: resource.MustParse("401Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("700m"),
											corev1.ResourceMemory:  resource.MustParse("400Mi"),
											corev1.ResourceStorage: resource.MustParse("201Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/",
											PathType: &implementationSpecific,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "component-sample",
													Port: networkingv1.ServiceBackendPort{
														Number: 1111,
													},
												},
											},
										},
									},
								},
							},
							Host: host,
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
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					Path: "/",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(1111),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "component-sample",
					},
				},
			},
			wantService: corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
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
			name:          "Simple devfile from Inline with deployment and svc from separated kube components - with RevisionHistoryLimit set",
			devfileString: kubernetesInlinedDevfileSeparatedKubeCompsRevHistory,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			hostname:      host,
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: &setRevHistoryLimit,
					Replicas:             &replicaUpdated,
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
											corev1.ResourceStorage: resource.MustParse("401Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("700m"),
											corev1.ResourceMemory:  resource.MustParse("400Mi"),
											corev1.ResourceStorage: resource.MustParse("201Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/",
											PathType: &implementationSpecific,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "component-sample",
													Port: networkingv1.ServiceBackendPort{
														Number: 1111,
													},
												},
											},
										},
									},
								},
							},
							Host: host,
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
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					Path: "/",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(1111),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "component-sample",
					},
				},
			},
			wantService: corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
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
			name:          "Simple devfile from Inline with only route",
			devfileString: kubernetesInlinedDevfileRoute,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			wantRoute: routev1.Route{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
						"test":                         "test",
					},
				},
				Spec: routev1.RouteSpec{
					Host: "route111222",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(5566),
					},
					To: routev1.RouteTargetReference{
						Kind:   "Service",
						Name:   "component-sample",
						Weight: &weight,
					},
					TLS: &routev1.TLSConfig{
						Termination:                   routev1.TLSTerminationEdge,
						InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with only route, no targetport - should not panic",
			devfileString: kubernetesInlinedDevfileRouteNoTargetPort,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			wantRoute: routev1.Route{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: "route.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
						"test":                         "test",
					},
				},
				Spec: routev1.RouteSpec{
					Host: "route111222",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(5566),
					},
					To: routev1.RouteTargetReference{
						Kind:   "Service",
						Name:   "component-sample",
						Weight: &weight,
					},
					TLS: &routev1.TLSConfig{
						Termination:                   routev1.TLSTerminationEdge,
						InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with only Ingress",
			devfileString: kubernetesInlinedDevfileIngress,
			componentName: "component-sample",
			appName:       "application-sample",
			hostname:      host,
			image:         "image1",
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
						"test":                         "test",
					},
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/rewrite-target": "/",
						"test": "yes",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/testpath",
											PathType: &implementationSpecific,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "test",
													Port: networkingv1.ServiceBackendPort{
														Number: 5566,
													},
												},
											},
										},
									},
								},
							},
							Host: "foo.bar.com",
						},
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with only Svc",
			devfileString: kubernetesInlinedDevfileSvc,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			wantService: corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
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
			appName:       "application-sample",
			image:         "image1",
			hostname:      host,
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: &revHistoryLimit,
					Replicas:             &replicaUpdated,
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
											corev1.ResourceStorage: resource.MustParse("401Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("700m"),
											corev1.ResourceMemory:  resource.MustParse("400Mi"),
											corev1.ResourceStorage: resource.MustParse("201Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/",
											PathType: &implementationSpecific,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "component-sample",
													Port: networkingv1.ServiceBackendPort{
														Number: 1111,
													},
												},
											},
										},
									},
								},
							},
							Host: host,
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
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					Path: "/",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(1111),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "component-sample",
					},
				},
			},
		},
		{
			name:          "Devfile with long component name - route name should be trimmed",
			devfileString: kubernetesInlinedDevfileDeploy,
			componentName: "component-sample-component-sample-component-sample",
			appName:       "application-sample",
			image:         "image1",
			hostname:      host,
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample-component-sample-component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample-component-sample-component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample-component-sample-component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: &revHistoryLimit,
					Replicas:             &replicaUpdated,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/instance": "component-sample-component-sample-component-sample",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/instance": "component-sample-component-sample-component-sample",
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
											corev1.ResourceStorage: resource.MustParse("401Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:     resource.MustParse("700m"),
											corev1.ResourceMemory:  resource.MustParse("400Mi"),
											corev1.ResourceStorage: resource.MustParse("201Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample-component-sample-component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample-component-sample-component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample-component-sample-component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/",
											PathType: &implementationSpecific,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "component-sample-component-sample-component-sample",
													Port: networkingv1.ServiceBackendPort{
														Number: 1111,
													},
												},
											},
										},
									},
								},
							},
							Host: host,
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
					Name: "component-sample-component-sample-component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample-component-sample-component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample-component-sample-component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					Path: "/",
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(1111),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "component-sample-component-sample-component-sample",
					},
				},
			},
		},
		{
			name:          "Simple devfile from Inline with Route Host missing",
			devfileString: kubernetesInlinedDevfileRouteHostMissing,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			hostname:      host,
			wantDeploy: appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
						"maysun":                       "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: &revHistoryLimit,
					Replicas:             &replicaUpdated,
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
			wantIngress: networkingv1.Ingress{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/",
											PathType: &implementationSpecific,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "component-sample",
													Port: networkingv1.ServiceBackendPort{
														Number: 5566,
													},
												},
											},
										},
									},
								},
							},
							Host: host,
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
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
					},
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
			name:          "Simple devfile from Inline with multiple kubernetes components and only one is referenced by deploy command",
			devfileString: multipleKubernetesCompsDevfile,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			wantService: corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
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
			name:          "Simple devfile from Inline with only one kubernetes component but no deploy command",
			devfileString: kubernetesCompsWithNoDeployCmdDevfile,
			componentName: "component-sample",
			appName:       "application-sample",
			image:         "image1",
			wantService: corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "component-sample",
					Labels: map[string]string{
						"app.kubernetes.io/created-by": "application-service",
						"app.kubernetes.io/instance":   "component-sample",
						"app.kubernetes.io/managed-by": "kustomize",
						"app.kubernetes.io/name":       "component-sample",
						"app.kubernetes.io/part-of":    "application-sample",
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
			name:          "No kubernetes components defined.",
			devfileString: noKubernetesCompDevfile,
			componentName: "component-sample",
			image:         "image1",
			wantErr:       true,
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
			deployAssociatedComponents, err := parser.GetDeployComponents(devfileData)
			if err != nil {
				t.Errorf("TestGetResourceFromDevfile() unexpected get deploy components error: %v", err)
			}
			logger := ctrl.Log.WithName("TestGetResourceFromDevfile")

			actualResources, err := GetResourceFromDevfile(logger, devfileData, deployAssociatedComponents, tt.componentName, tt.appName, tt.image, tt.hostname)
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

				if len(actualResources.Ingresses) > 0 {
					assert.Equal(t, tt.wantIngress, actualResources.Ingresses[0], "First Ingress did not match")
				}

				if len(actualResources.Routes) > 0 {
					if tt.name == "Devfile with long component name - route name should be trimmed" {
						if len(actualResources.Routes[0].Name) > 30 {
							t.Errorf("Expected trimmed route name with length < 30, but got %v", len(actualResources.Routes[0].Name))
						}
						if !strings.Contains(actualResources.Routes[0].Name, "component-sample-comp") {
							t.Errorf("Expected route name to contain %v, but got %v", "component-sample-comp", actualResources.Routes[0].Name)
						}
					} else {
						assert.Equal(t, tt.wantRoute, actualResources.Routes[0], "First Route did not match")
					}
				}
			}
		})
	}
}

func TestParseDevfileModel(t *testing.T) {

	testServerURL := "127.0.0.1:9080"

	simpleDevfile := `
metadata:
  attributes:
    appModelRepository.url: https://github.com/testorg/petclinic-app
    gitOpsRepository.url: https://github.com/testorg/petclinic-gitops
  name: petclinic
schemaVersion: 2.2.0`

	testServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(simpleDevfile))
		if err != nil {
			t.Errorf("TestParseDevfileModel() unexpected error while writing data: %v", err)
		}
	}))
	// create a listener with the desired port.
	l, err := net.Listen("tcp", testServerURL)
	if err != nil {
		t.Errorf("TestParseDevfileModel() unexpected error while creating listener: %v", err)
		return
	}

	// NewUnstartedServer creates a listener. Close that listener and replace
	// with the one we created.
	testServer.Listener.Close()
	testServer.Listener = l

	testServer.Start()
	defer testServer.Close()

	localPath := "/tmp/testDir"
	localDevfilePath := path.Join(localPath, "devfile.yaml")
	// prepare for local file
	err = os.MkdirAll(localPath, 0755)
	if err != nil {
		t.Errorf("TestParseDevfileModel() error: failed to create folder: %v, error: %v", localPath, err)
	}
	err = ioutil.WriteFile(localDevfilePath, []byte(simpleDevfile), 0644)
	if err != nil {
		t.Errorf("TestParseDevfileModel() error: fail to write to file: %v", err)
	}

	if err != nil {
		t.Error(err)
	}

	defer os.RemoveAll(localPath)

	tests := []struct {
		name              string
		devfileString     string
		devfileURL        string
		devfilePath       string
		wantDevfile       *v2.DevfileV2
		wantMetadata      devfile.DevfileMetadata
		wantSchemaVersion string
	}{
		{
			name:          "Simple devfile from data",
			devfileString: simpleDevfile,
			wantMetadata: devfile.DevfileMetadata{
				Name:       "petclinic",
				Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
			},
			wantSchemaVersion: string(data.APISchemaVersion220),
		},
		{
			name:       "Simple devfile from URL",
			devfileURL: "http://" + testServerURL,
			wantMetadata: devfile.DevfileMetadata{
				Name:       "petclinic",
				Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
			},
			wantSchemaVersion: string(data.APISchemaVersion220),
		},
		{
			name:        "Simple devfile from PATH",
			devfilePath: localDevfilePath,
			wantMetadata: devfile.DevfileMetadata{
				Name:       "petclinic",
				Attributes: attributes.Attributes{}.PutString("gitOpsRepository.url", "https://github.com/testorg/petclinic-gitops").PutString("appModelRepository.url", "https://github.com/testorg/petclinic-app"),
			},
			wantSchemaVersion: string(data.APISchemaVersion220),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var devfileSrc DevfileSrc
			if tt.devfileString != "" {
				devfileSrc = DevfileSrc{
					Data: tt.devfileString,
				}
			} else if tt.devfileURL != "" {
				devfileSrc = DevfileSrc{
					URL: tt.devfileURL,
				}
			} else if tt.devfilePath != "" {
				devfileSrc = DevfileSrc{
					Path: tt.devfilePath,
				}
			}
			devfile, err := ParseDevfile(devfileSrc)
			if err != nil {
				t.Errorf("TestParseDevfileModel() unexpected error: %v", err)
			} else {
				gotMetadata := devfile.GetMetadata()
				if !reflect.DeepEqual(gotMetadata, tt.wantMetadata) {
					t.Errorf("TestParseDevfileModel() metadata is different")
				}

				gotSchemaVersion := devfile.GetSchemaVersion()
				if gotSchemaVersion != tt.wantSchemaVersion {
					t.Errorf("TestParseDevfileModel() schema version is different")
				}
			}
		})
	}
}
