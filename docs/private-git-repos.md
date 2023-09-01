# Using Private Git Repositories with HAS

## Configuring SPI

In order to use HAS resources (e.g. `Applications`, `Components`, `ComponentDetectionQuery`) with private git repositories, SPI must be installed on the same cluster as HAS (TODO review HAS install instructions):

1) Clone the [SPI operator repo](https://github.com/redhat-appstudio/service-provider-integration-operator) and run the [make command ](https://github.com/redhat-appstudio/service-provider-integration-operator/blob/main/docs/DEVELOP.md#running-in-cluster) corresponding to your target cluster type e.g. `make deploy_openshift`


2) Set up SPI

    a) Get SPI oauth route URL from `spi-system` namespace `oc get routes -n spi-system`

    b) Create oauth app in GitHub (`Settings` -> `Developer Settings` -> `OAuth Apps`)

      - Use the SPI oauth url as the Application Callback URL.
      - Homepage URL does not matter
      - Record the Client ID and Client Secret values 

    c) To set up a Github Oauth app with SPI, modify the overlay in your cloned SPI repo that corresponds with the cluster type e.g. in config/overlays/openshift_vault/config.yaml, replace the `clientId` and `clientSecret` with the values from the oauth app you created in step 2.  Run ` kustomize build config/overlays/openshift_vault | kubectl apply -f -` to update the `shared-configuration-file` secret



## Creating a Token

1) In github, generate a new classic token with User and Repo scope


To create a token to use with HAS:

1) Create an `SPIAccessTokenBinding` resource with the following contents:
   ```yaml
    apiVersion: appstudio.redhat.com/v1beta1
    kind: SPIAccessTokenBinding
    metadata:
      name: test-access-token-binding
      namespace: default
    spec:
      permissions:
        required:
         - type: rw
           area: repository
      repoUrl: https://github.com/johnmcollier/private-devfile-repo
      secret:
        name: token-secret
        type: kubernetes.io/basic-auth
   ```
   
2) Create the resource in the namespace you will be creating HAS resources in.  Upon successful creation, the CR will be in `AwaitingTokenData` phase status and a corresponding SPIAccessToken CR will be created in the same namespace.

3) Upload the token: 
   1) Set the TARGET_NAMESPACE to where your CRs instances are.  Run `UPLOAD_URL=$(kubectl get spiaccesstokenbinding/test-access-token-binding -n $TARGET_NAMESPACE -o  json | jq -r .status.uploadUrl)`
   2) Inject the token where TOKEN is the console admin secret and GITHUB_TOKEN is the token created in step 1)
   
      `curl -v -H 'Content-Type: application/json' -H "Authorization: bearer "$TOKEN -d "{ \"access_token\": \"$GITHUB_TOKEN\" }" $UPLOAD_URL`
   3)  The state of the SPIAccessTokenBinding should change to `Injected` and the state of the SPIAccessToken should be `Ready`
   4)  This will also create a K8s secret corresponding to the name of the secret that was specified in the SPIAccessTokenBinding created in step 2.
 
   
## Using Private Git Repositories

Now, with the token secret created for the git repository, when creating HAS resources (`Components`, `ComponentDetectionQueries`) that need to access that Git repository, just pass in the token secret to the resource:

**Component**

```yaml
apiVersion: appstudio.redhat.com/v1alpha1
kind: Component
metadata:
  name: component-sample
spec:
  componentName: backend
  application: application-sample
  replicas: 1
  source:
    git:
      url: https://github.com/johnmcollier/devfile-private.git
  secret: token-multi-secret
```

**ComponentDetectionQuery**

```yaml
apiVersion: appstudio.redhat.com/v1alpha1
kind: ComponentDetectionQuery
metadata:
  name: componentdetectionquery-sample
spec:
  isMultiComponent: true
  git:
    url: https://github.com/johnmcollier/multi-component-private.git
  secret: token-multi-secret
```