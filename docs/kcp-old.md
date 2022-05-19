# Running HAS in KCP

## Running KCP

1) Git clone https://github.com/kcp-dev/kcp and cd into `kcp`

2) In a terminal window, run `go run ./cmd/kcp start`

## Running the Operator

Before running the following commands, ensure that you have the following environment variables exported:

1. `KUBECONFIG=<path-to-kcp>/.kcp/data/admin.kubeconfig` Set to the path of KCP's kubeconfig

2. `GITHUB_AUTH_TOKEN=<github-token>` Set to a GitHub Personal Access Token with `repo` and `delete_repo` permissions.

3. (Optional) `GITHUB_ORG=<github-org>` Set to a GitHub org (or account) if you do not have write access to `redhat-appstudio-appdata`.

KCP has the rudimentary Kubernetes resources, hence as a result admission & validating webhooks wont work on KCP. There is currently an [issue](https://github.com/kcp-dev/kcp/issues/143) on KCP to discuss when the feature would be installed on KCP. To disable webhooks and the certificate manager to run the operator, complete the following steps:

1. Search for `# Comment for KCP` in files `config/crd/kustomization.yaml`, `config/default/kustomization.yaml` and comment out the webhooks and cert-manager sections as instructed

2. Run `make bundle` and it should update the bundles without the webhook and cert-manager
   
3. export `ENABLE_WEBHOOKS=false` to disable all the webhook controller logic

Once the above prerequisites have been met, run the following commands:

1. `make build` to build the HAS operator binary

2. `make install-kcp` to install the HAS CRDs onto the KCP instance

3. `./bin/manager` to run the operator


## Testing HAS

1) Open a new terminal window

2) Run `export KUBECONFIG=<path-to-kcp>/.kcp/data/admin.kubeconfig` to set your Kubeconfig to KCP

3) `kubectl create ns default` to create a namespace to use

4) Run `kubectl apply -f samples/application/hasapp.yaml` to create a simple Application resource

5) Run `kubectl get hasapp application-sample -o yaml`, and verify you see the following:

```
apiVersion: appstudio.redhat.com/v1alpha1
kind: Application
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"appstudio.redhat.com/v1alpha1","kind":"Application","metadata":{"annotations":{},"name":"application-sample","namespace":"default"},"spec":{"description":"application definition for petclinic-app","displayName":"petclinic"}}
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
  name: application-sample
  namespace: default
  resourceVersion: "167"
  uid: 95e1d4f4-5876-4065-beef-b77d869fc00b
spec:
  description: application definition for petclinic-app
  displayName: petclinic
status:
  conditions:
  - lastTransitionTime: "2021-10-29T20:22:16Z"
    message: Application has been successfully created
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