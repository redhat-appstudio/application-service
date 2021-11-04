# Hybrid Application Service (HAS)
An Kubernetes operator to create and manage applications and control the lifecycle of applications.


## Building & Testing
This operator provides a `Makefile` to run all the usual development tasks. If you simply run `make` without any arguments, you'll get a list of available "targets".

To build the operator binary run:

```
make build
```

To test the code:

```
make test
```

To build the docker image of the operator one can run:

```
make docker-build
```

This will make a docker image called `controller:latest` which might or might not be what you want. To override the name of the image build, specify it in the `IMG` environment variable, e.g.:

```
IMG=quay.io/user/hasoperator:next make docker-build
```

To push the image to an image repository one can use:

```
make docker-push
```

The image being pushed can again be modified using the environment variable:
```
IMG=quay.io/user/hasoperator:next make docker-push
```


Useful links:
* [HAS Project information page](https://docs.google.com/document/d/1axzNOhRBSkly3M2Y32Pxr1MBpBif2ljb-ufj0_aEt74/edit?usp=sharing)
