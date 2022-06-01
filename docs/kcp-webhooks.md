# Webhooks on KCP

By default, the KCP-specific kustomize manifests under `config/kcp` do **not** deploy HAS with webhooks. 

At this point in time, Webhooks on KCP need to be exposed over a Route / Ingress URL. The following instructions outline how to do that.

  - At least, on an OpenShift workload cluster. If you're running HAS on a non-OpenShift workload cluster, these steps will differ.

## Prereqs:

- [OpenShift Acme installed](https://developer.ibm.com/tutorials/secure-red-hat-openshift-routes-with-lets-encrypt) on the workload cluster
  - cert-manager w/ let's encrypt should also work if you're using non-OpenShift clusters

## Steps:

On the KCP side-of-things:

1) Follow the [HAS on KCP](./kcp.md) doc to install HAS on KCP.

Next, switch to the workload cluster:

2) Find the namespace on the workload cluster where KCP synced HAS to. It should look something like `kcp1b06ba2d907942fa826c1c66e7eb185b2708dd4b69770069eaa470b8`

   - Validate that this is the namespace where HAS got synced to, by seeing if there are running pods in the namespace
   - For convenience with the following commands run `export KCP_NAMESPACE=<your-kcp-namespace>`

2) `oc apply -f hack/kcp/webhook-service.yaml -n $KCP_NAMESPACE` to create the Service exposing the webhook service

3) A secret, `webhook-server-cert` should have been created in the same namespace. Run `oc extract secret/webhook-server-cert -n $KCP_NAMESPACE` to retrieve the TLS certificates for the webhook service.

4) Copy the contents of the `tls.crt` file.

5) Open `hack/kcp/webhook-route.yaml` in an editor and make the following changes: 
  - Replace the `<REPLACE_CERT>` with the TLS Certificate retrieved from the previous step. 
  - Replace the `<REPLACE_HOST>` with a valid Route hostname (e.g. webhook.apps.myopenshiftcluster.com). We are not using generated hostnames because the generated hostname is too long (> 63 characters) because of the kcp namespace length.
  
    It should look something like this:
   
    ```
    kind: Route
    apiVersion: route.openshift.io/v1
    metadata:
      name: webhook-route
      annotations:
        kubernetes.io/tls-acme: 'true'
    spec:
      host: $REPLACE_HOST
      to:
        kind: Service
        name: webhook-service
        weight: 100
      port:
        targetPort: 9443
      tls:
        termination: reencrypt
        destinationCACertificate: |
          -----BEGIN CERTIFICATE-----
          FAKECERTIFICATE1
          -----END CERTIFICATE-----
          -----BEGIN CERTIFICATE-----
          FAKECERTIFICATE2
          -----END CERTIFICATE-----
        insecureEdgeTerminationPolicy: None
      wildcardPolicy: None
      ```
    
5) `oc apply -f hack/kcp/webhook-route.yaml -n $KCP_NAMESPACE` to create the route
  
    - *NOTE:* You must make sure the generated url for the Route is less than 63 characters, or else OpenShift Acme's cert generation *will* fail. 
  
    - If the generated route url is over 63 characters, modify it to be shorter.

6) Access the Route URL in your browser and verify that publicly signed certificates are being used.

Next, switch back to KCP and do the following

7) Open `config/kcp-webhook/manifests.yaml` in an editor and change all occurrences of `<REPLACE_HOST>` to the exposed Route URL from the previous step

    - There should be 4 occurrences that need to be changed.

8) Run `kustomize build config/kcp-webhook | kubectl apply -f -` to deploy the webhooks
