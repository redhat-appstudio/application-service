# Testing HAS

## Unit Tests

There are unit tests written for the operator, using the Go `testing` library. They cover any packages under the `pkg/` folder. To run these tests, just run

```
make test
```

## Controller Tests

There are tests written for each controller (e.g. HASApplication) in the operator, using the `ginkgo` BDD testing library. To run these tests (along with any unit tests), just run

```
make test
```

### Running Controller Tests in VS Code

Running the controller's tests (tests under the `controller/` directory) in VS Code require a bit of leg work before working.

First, make sure you have run `make test` on your system at least once, as this ensures certain binaries like `kube-apiserver` and `etcd` are downloaded on to your system. On Linux, this is `$XDG_DATA_HOME/io.kubebuilder.envtest`; on Windows, `%LocalAppData\io.kubebuilder.envtest`; and on OSX, `~/Library/Application Support/io.kubebuilder.envtest`

cd to that folder 
Then, add the following environment variable to your shell configuration:

```
export KUBEBUILDER_ASSETS=<path>/io.kubebuilder.envtest/k8s/<version>
```

e.g.:
```
export KUBEBUILDER_ASSETS=/Users/john/Library/Application Support/io.kubebuilder.envtest/k8s/1.22.1-darwin-amd64
```

From here, you should be good to run the controller's unit tests in VS Code. Verify this by opening `hasapplication_controller_test.go` in VS Code and running `run package tests`.

## Integration Tests

TBD