# GitOps Generation

The generation of GitOps resources is handled by the go module [redhat-developer/gitops-generator](https://github.com/redhat-developer/gitops-generator). Please refer to the module's README for more information on the usage.

### Business Logic

The application-service `Component` controller parses the devfile's `deploy` command, and gets the corresponding `kubernetes` component outerloop resources. The controller overwrites the first resource of each Deployment, Service, Route type with the `Component` configuration and calls the Gitops generation library.

If for any reason, the controller is unable to find the devfile `kubernetes` component outerloop information, barring an error condition, the GitOps generation library will generate the Deployment, Service and Route resources from the `Component` configuration information.

### Development

When working on a story that requires contribution to [redhat-developer/gitops-generator](https://github.com/redhat-developer/gitops-generator)

- replace the gitops-generator go module's path in your local `go.mod` with the local path of a gitops-generator repository to ensure application-service is working as expected
  - Example, replace the gitops generator module with the local path in go.mod: 
    ```go
    replace github.com/redhat-developer/gitops-generator => <path-to-local-gitops-generator-repo>
    ```

- have your changes reviewed and merged directly to the [redhat-developer/gitops-generator](https://github.com/redhat-developer/gitops-generator) repository
- after merging the gitops-generator changes, run `go list -m -json github.com/redhat-developer/gitops-generator@latest` to determine the latest version of the gitops-generator library module
- remove the local reference of the gitops-generator module and update to the version from the previous step in your `go.mod` file
