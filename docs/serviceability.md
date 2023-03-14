# Serviceability

## Logs

### Deployed Locally
View the application-service controller logs by running the manager executable. Example,

```
make install
make build
./bin/manager
```

### Deployed on a Cluster
View the application-service controller logs by tailing the manager container log of the controller manager pod. Example,

```
oc logs -f application-service-application-service-controller-manager -c manager
```

### Understanding the Logs
Each application-service controller logs their reconcile logic to the manager. The log message format is generally of format 

```
2023-03-14T08:39:25.009Z	INFO	controllers.Component	updating devfile component name kubernetes-deploy ...	{"appstudio-component": "HAS", "Component": "mynamespace/pipeline-devfile-example-lcec", "clusterName": ""}
```

The above log indicates that the message is logged by the `Component` controller for the resource `pipeline-devfile-example-lcec` in the namespace `mynamespace`.

## Debugging

- Insert break points at the controller functions to debug unit tests
- When debugging unit and integration tests, remember that the mock clients used are hosting dummy data and mocking the api call and returns 

## Common Problems
- A Github Personal Access token is required as the application-service controller requires the token for pushing the resources to the GitOps repository
- When creating a `Component`, remember to replace the generic application name `insert-application-name`, if the information is being used from a `ComponentDetectionQuery` status

## FAQs
Q. Where can I view the application-service api types?

A. The application-service api types and their corresponding webhooks are defined in the [redhat-appstudio/application-api](https://github.com/redhat-appstudio/application-api) repository.

Q. Where are the application-service controller logic located?

A. Most of the application-service business logic like the reconcile functions are located in the [controllers](https://github.com/redhat-appstudio/application-service/tree/main/controllers) pkg.

Q. What is the application-service release process?

A. The release process for application-service is yet to be determined and defined.

Q. Where can I learn more about the devfile project?

A. The devfile project is hosted on the [devfile/api](https://github.com/devfile/api) repository and more information about the project as well as the spec can be found on [devfile.io](https://devfile.io/).

Q. How does `ComponentDetectionQuery` detect the component framework and runtime?

A. `ComponentDetectionQuery` uses the go module [redhat-developer/alizer](https://github.com/redhat-developer/alizer) for component detection. For more information about the Alizer project, please head over to the Alizer repository.