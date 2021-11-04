# Running HAS in KCP

## Running KCP

1) Git clone https://github.com/kcp-dev/kcp and cd into `kcp`

2) In a terminal window, run `go run ./cmd/kcp start`

## Installing the Operator

1) Open a terminal window in the root of this repository

2) Run `export KUBECONFIG=<path-to-kcp>/.kcp/data/admin.kubeconfig` to set your Kubeconfig to KCP

3) Run `make build` to build the HAS operator binary

4) Run `make install` to install the HAS CRDs onto the KCP instance

5) Run `./bin/manager` to run the operator


## Testing HAS

1) Open a new terminal window

2) Run `export KUBECONFIG=<path-to-kcp>/.kcp/data/admin.kubeconfig` to set your Kubeconfig to KCP

3) `kubectl create ns default` to create a namespace to use

4) Run `kubectl apply -f samples/hasapplication/hasapp.yaml` to create a simple HASApplication resource

5) Run `kubectl get hasapp hasapplication-sample -o yaml`, and verify you see the following:

```
apiVersion: appstudio.redhat.com/v1alpha1
kind: HASApplication
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"appstudio.redhat.com/v1alpha1","kind":"HASApplication","metadata":{"annotations":{},"name":"hasapplication-sample","namespace":"default"},"spec":{"description":"application definition for petclinic-app","displayName":"petclinic"}}
  clusterName: admin
  creationTimestamp: "2021-10-29T20:22:16Z"
  generation: 1
  managedFields:
  - apiVersion: appstudio.redhat.com/v1alpha1
    fieldsType: FieldsV1
    fieldsV1:
      f:metadata:
        f:annotations:
          .: {}
          f:kubectl.kubernetes.io/last-applied-configuration: {}
      f:spec:
        .: {}
        f:description: {}
        f:displayName: {}
    manager: kubectl-client-side-apply
    operation: Update
    time: "2021-10-29T20:22:16Z"
  - apiVersion: appstudio.redhat.com/v1alpha1
    fieldsType: FieldsV1
    fieldsV1:
      f:status:
        .: {}
        f:conditions: {}
        f:devfile: {}
    manager: manager
    operation: Update
    subresource: status
    time: "2021-10-29T20:22:16Z"
  name: hasapplication-sample
  namespace: default
  resourceVersion: "167"
  uid: 95e1d4f4-5876-4065-beef-b77d869fc00b
spec:
  description: application definition for petclinic-app
  displayName: petclinic
status:
  conditions:
  - lastTransitionTime: "2021-10-29T20:22:16Z"
    message: HASApplication has been successfully created
    reason: OK
    status: "True"
    type: Created
  devfile: |
    metadata:
      attributes:
        appModelRepository.url: https://github.com/redhat-appstudio-appdata/petclinic-choose-responsibility
        gitOpsRepository.url: https://github.com/redhat-appstudio-appdata/petclinic-choose-responsibility
      description: application definition for petclinic-app
      name: petclinic
    schemaVersion: 2.2.0
```