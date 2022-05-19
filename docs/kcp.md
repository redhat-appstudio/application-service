# Running HAS in KCP

## Prereqs

- Access to a hosted KCP environment
- An active KCP workspace, with an OpenShift-based `WorkloadCluster` inside it.

## Before Deployment

Before you deploy HAS on KCP, make sure you create the `has-github-token` secret in the namespace you will be deploying HAS within:

```bash
kubectl create secret generic has-github-token --from-literal=token=$TOKEN -n application-service-system
```

If `kubectl` complains that `application-service-system` does not exist, create it, and then retry the command.

where `$TOKEN` is the GitHub token to be used with HAS, as described [here](https://github.com/redhat-appstudio/application-service#creating-a-github-secret-for-has).

## Deploying the Operator

To deploy HAS on KCP, just run:

```bash
make deploy-kcp
```

Next, run `kubectl get deploy -n application-service-system` to validate that HAS was successfully deployed and synced to the workload cluster.

By default, Webhooks will **not** be installed with HAS. If you wish to install Webhooks on top of HAS, [please see this doc](./kcp-webhooks.md).

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