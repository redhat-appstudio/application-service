# Running HAS in KCP

## Running KCP

1) Git clone https://github.com/kcp-dev/kcp and cd into `kcp`

2) In a terminal window, run `go run ./cmd/kcp start`

## Installing the Operator

1) Open a terminal window in the root of this repository

2) Run `export KUBECONFIG=<path-to-kcp>/.kcp/data/admin.kubeconfig` to set your Kubeconfig to KCP

3) Run `make build` to build the HAS operator binary

4) Run `make install` to install the HAS CRDs onto the KCP instance

5) Run `./bin/manager` to run the operator


## Testing HAS

1) Open a new terminal window

2) Run `export KUBECONFIG=<path-to-kcp>/.kcp/data/admin.kubeconfig` to set your Kubeconfig to KCP

3) `kubectl create ns default` to create a namespace to use

4) Run `kubectl apply -f samples/hasapplication/hasapp.yaml` to create a simple HASApplication resource

5) Run `kubectl get hasapp hasapplication-sample -o yaml`, and verify you see the following:

```

```