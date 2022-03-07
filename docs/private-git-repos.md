# Using Private Git Repositories with HAS

## Configuring SPI

In order to use HAS resources (e.g. `Applications`, `Components`, `ComponentDetectionQuery`) with private git repositories, SPI must be installed on the same cluster as HAS:

1) Set up `infra-deployments`. Minimally:

   a) Install RBAC for App Studio: `kustomize build openshift-gitops/cluster-rbac | oc apply -f -`

   b) Install Build component`oc apply -n openshift-gitops argo-cd-apps/base/build.yaml`

   c) Install SPI component`oc apply -n openshift-gitops argo-cd-apps/base/spi.yaml`

2) Set up SPI

    a) Get SPI oauth route URL from `spi-system` namespace

    b) Create oauth app in GitHub (`Settings` -> `Developer Settings` -> `OAuth Apps`)

      - Use the SPI oauth url as the Application Callback URL.
      - Homepage URL does not matter
      - Record the Client ID and Client Secret values 
  
   c) Open `components/spi/config.yaml` from `infra-deployments` in an editor and change the following values:
     - `sharedSecret` -> Can be a random string, doesn't matter what you put
     - `clientId` -> Client ID value from previous step
     - `clientSecret` -> Client secret value from previous step
     - `baseUrl` -> SPI oauth URL

        **For example**:
        ```yaml
        sharedSecret: fsdfsdfsdfdsf
        serviceProviders:
        - type: GitHub
          clientId: fake-client-id
          clientSecret: fake-client-secret
        baseUrl: https://spi-oauth-route-spi-system.apps.mycluster.com
        ```

## Creating a Token

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

2) Create the resource in the namespace you will be creating HAS resources in

3) Run `oc get spiaccesstokenbinding test-access-token-binding -o jsonpath="oAuthUrl: {.status.oAuthUrl}"` to get the oauth url
  
    - Alternatively, you can just get the oauth url by running `oc get spiaccesstokenbinding -o yaml` and viewing the status directly.

4) Access the oauth url in your browser. Log in if needed.

5) Run `oc get secrets` in the namespace you created the `SPIAccessTokenBinding` in to verify that the secret was created successfully.

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