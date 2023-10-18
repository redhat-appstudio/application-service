# Using Private Git Repositories with HAS

Please follow the following instructions to install SPI and use with application-service.

Note: SPI _cannot_ be used on a Red Hat OpenShift Local (formerly, CRC) cluster.
## Configuring SPI

In order to use HAS resources (e.g. `Application`, `Component`, `ComponentDetectionQuery`) with private git repositories, SPI must be installed on the same cluster as HAS (TODO review HAS install instructions):

1) Clone the [SPI operator repo](https://github.com/redhat-appstudio/service-provider-integration-operator) and run the [make command](https://github.com/redhat-appstudio/service-provider-integration-operator/blob/main/docs/DEVELOP.md#running-in-cluster) corresponding to your target cluster type e.g. `make deploy_openshift`


2) Set up SPI
   1) Get SPI oauth route URL from `spi-system` namespace `oc get routes -n spi-system`
   2) Create oauth app in GitHub (`Settings` -> `Developer Settings` -> `OAuth Apps`)
      - Use the SPI oauth url as the Application Callback URL.
      - Homepage URL does not matter
      - Record the Client ID and Client Secret values 
   3) To set up a Github Oauth app with SPI, modify the overlay in your cloned SPI repo that corresponds with the cluster type e.g. in config/overlays/openshift_vault/config.yaml, replace the `clientId` and `clientSecret` with the values from the oauth app you created in step 2.  Run ` kustomize build config/overlays/openshift_vault | kubectl apply -f -` to update the `shared-configuration-file` secret



## Creating a Token

1) In Github, generate a new classic token with User and Repo scope and note down the token value.
2) To create a token secret to use with HAS, draft a `SPIAccessTokenBinding` resource with the following contents:
   ```yaml
    apiVersion: appstudio.redhat.com/v1beta1
    kind: SPIAccessTokenBinding
    metadata:
      name: test-access-token-binding
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
   
3) Create the resource in the namespace you will be creating HAS resources in.  Upon successful creation, the CR will be in `AwaitingTokenData` phase status and a corresponding `SPIAccessToken` CR will be created in the same namespace.

3) Upload the token: 
   1) Set the TARGET_NAMESPACE to where your CRs instances are.  Run `UPLOAD_URL=$(kubectl get spiaccesstokenbinding/test-access-token-binding -n $TARGET_NAMESPACE -o  json | jq -r .status.uploadUrl)`
   2) Inject the token with the curl command, where TOKEN is the console admin token and GITHUB_TOKEN is the token created in main step 1 above
   
      `curl -v -H 'Content-Type: application/json' -H "Authorization: bearer "$TOKEN -d "{ \"access_token\": \"$GITHUB_TOKEN\" }" $UPLOAD_URL`
   3)  The state of the `SPIAccessTokenBinding` CR should change to `Injected` and the state of the `SPIAccessToken` should be `Ready`
   4)  This will also create a K8s secret corresponding to the name of the secret that was specified in the `SPIAccessTokenBinding` created in main step 2 above, for example `token-secret`. Use the secret in HAS CRs for private repositories.
 
   
## Using Private Git Repositories

Now, with the token secret created for the git repository, when creating HAS resources (`Components`, `ComponentDetectionQueries`) that need to access that private Git repository, just pass in the token secret to the resource:

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
  secret: token-secret
```

**ComponentDetectionQuery**

```yaml
apiVersion: appstudio.redhat.com/v1alpha1
kind: ComponentDetectionQuery
metadata:
  name: componentdetectionquery-sample
spec:
  git:
    url: https://github.com/johnmcollier/multi-component-private.git
  secret: token-secret
```