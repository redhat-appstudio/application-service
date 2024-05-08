# Serviceability

The serviceability document aims to help the local application-service developer and the Site Reliability Engineer (SRE) to access and service the application-service component. This document will help you understand how to access and understand the application-service logs, how to debug an application-service problem and provides a quick summary on the various questions that you might have regarding application-service. 

## Accessing the Logs

### Deployed Locally
View the application-service controller logs in the terminal window where the executable manager is running. Example, `./bin/manager` will output the controller logs in the terminal.

### Deployed on a Local Cluster
View the application-service controller logs by tailing the manager container log of the controller manager pod. The pod resides in the application-service namespace. Example,

```
oc logs -f application-service-application-service-controller-manager -c manager -n application-service
```

### Deployed on a Managed Cluster

To access the application-service controller logs on either the AppStudio Staging or Production clusters, you would need access to CloudWatch service. To learn more about how to request access to CloudWatch and view the application-service logs, refer to the HAS Access Control [documentation](https://docs.google.com/document/d/1cK4XGKpXBEYOKfIqSiHuuCfsfHjElxhG9lrlEozzgVE/edit#heading=h.yxk6h5uvh57d). 

## Understanding the Logs
Each application-service controller logs their reconcile logic to the manager. The log message format is generally of format 

```
{"level":"info","ts":"2023-08-31T19:59:21.144Z","msg":"Finished reconcile loop for user-tenant/devfile-sample-go-basic-development-binding-hr9nm","controller":"snapshotenvironmentbinding","controllerGroup":"appstudio.redhat.com","controllerKind":"SnapshotEnvironmentBinding","SnapshotEnvironmentBinding":{"name":"devfile-sample-go-basic-development-binding-hr9nm","namespace":"user-tenant"},"namespace":"user-tenant","name":"devfile-sample-go-basic-development-binding-hr9nm","reconcileID":"d5c8545b-957b-4f1a-b177-84a5d5f0d26c"}
```

To understand the AppStudio controller logging convention, refer to the Appstudio [ADR](https://github.com/redhat-appstudio/book/blob/main/ADR/0006-log-conventions.md)

## Debugging

- Insert break points at the controller functions to debug unit tests or to debug a local controller deployment, refer to the next section on how to set up a debugger
- When debugging unit and integration tests, remember that the mock clients used are hosting dummy data and mocking the API call and returns
- For debugging controller logs either from a local cluster or a managed cluster, gather the log and search for:
  - the namespace the user belongs to. For example, if the user is on the project/namespace, search for `user-tenant`
  - the resource name that the controller is reconciling. For example, if the `SnapshotEnvironmentBinding` resource is being reconciled, then search for `"name":"devfile-sample-go-basic-development-binding-hr9nm"` belonging to `"controllerKind":"SnapshotEnvironmentBinding"`
  - the log message. For example, `"msg":"Finished reconcile loop for user-tenant/devfile-sample-go-basic-development-binding-hr9nm"`. You may search for the string `Finished reconcile loop for` in the application-service repository to track down the code logic that is emitting the log. Remember to look out for resource name concatenation and/or error wrapping that may not turn up in your code search. It is advised to exclude such strings from the code search for debugging purposes

### How to debug on RHTAP or How to set up a debugger on VS Code

For more information, on how to debug on RHTAP Staging or how to set up a debugger on VS Code for local deployment of the application-service controller, please refer to the [Debugging](https://docs.google.com/document/d/1dneldJepfnJ6LnESSYMIhKqmFgjMtf_om_Eud5NMDtU/edit#heading=h.lz54tm3le87l) section of the Education Module document.

## Common Problems
- When deploying HAS locally or on a local cluster, a Github Personal Access Token is required as the application-service controller requires the token for pushing the resources to the GitOps repository. Please refer to the [instructions](../docs/build-test-and-deploy.md#setting-the-github-token-environment-variable) in the deploy section for more information
- When creating a `Component` from the `ComponentDetectionQuery`, remember to replace the generic application name `insert-application-name`, if the information is being used from a `ComponentDetectionQuery` status

## FAQs
Q. Where can I view the application-service API types?

A. The application-service API types and their corresponding webhooks are defined in the [redhat-appstudio/application-api](https://github.com/redhat-appstudio/application-api) repository. You can also find the API reference information in the [Book of AppStudio](https://redhat-appstudio.github.io/architecture/ref/index.html) website.

Q. Where are the application-service controller logic located?

A. Most of the application-service business logic like the reconcile functions are located in the [controllers](https://github.com/konflux-ci/application-service/tree/main/controllers) pkg.

Q. What is the application-service release process?

A. Since application-service is part of the AppStudio project, code commits to the application-service repository are automatically added to the [infra-deployments](https://github.com/redhat-appstudio/infra-deployments/tree/main/components/has) repository.

Q. Where can I learn more about the devfile project?

A. The devfile project is hosted on the [devfile/api](https://github.com/devfile/api) repository and more information about the project as well as the spec can be found on [devfile.io](https://devfile.io/).

Q. How does `ComponentDetectionQuery` detect the component framework and runtime?

A. `ComponentDetectionQuery` uses the go module [devfile/alizer](https://github.com/devfile/alizer) for component detection. For more information about the Alizer project, please head over to the Alizer repository.

Q. How do I debug zombie processes?

A. For more information on how zombie processes affect application-service controllers, please refer to the Troubleshooting [guide](https://docs.google.com/document/d/1yCFkFslhbdd8M_RarRhZcgx6gm9nr2JwDObxNtl4H-U/edit#heading=h.4brqv3sh6lq9).

Q. How do I debug Rate Limiting?

A. The GitHub Personal Access Tokens used by application-service controllers may be rate limited. For more information on how the GitHub PAT rate limiting affects application-service controllers, please refer to the Troubleshooting [guide](https://docs.google.com/document/d/1yCFkFslhbdd8M_RarRhZcgx6gm9nr2JwDObxNtl4H-U/edit#heading=h.3xnfno3qm3if).
