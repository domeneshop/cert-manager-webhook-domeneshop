# cert-manager-webhook-domeneshop

This is a DNS01 webhook implementation for [cert-manager](https://github.com/jetstack/cert-manager),
allowing usage of the [Domeneshop API](https://api.domeneshop.no/docs/) to issue certificates for
wildcard domains or other names that are not publicly accessible. 

# Usage

## Requirements

- Working [cert-manager](https://github.com/jetstack/cert-manager) deployed in your Kubernetes cluster
- An API key for the [Domeneshop API](https://api.domeneshop.no/docs/)
- A domain configured to use DNS service with Domeneshop

## Installing

1. Create a Kubernetes namespace for the webhook to live in

    ```
    kubectl create ns webhook-domeneshop
    ```

2. Install the Helm chart

    ```
    helm install webhook --set groupName='api.domeneshop.no' --namespace=webhook-domeneshop deploy/domeneshop-webhook
    ```

3. Ensure the pod is running

    ```shell
    % kubectl get pods -n webhook-domeneshop
    NAME                                                       READY   STATUS    RESTARTS   AGE
    webhook-cert-manager-webhook-domeneshop-7745d84f75-qrlsk   1/1     Running   0          108s
    ```

## Issuer and secrets

In order to issue certificates using the webhook, create a new Issuer resource with cert-manager.

Ensure the email address is set to a valid address, and that the `groupName` matches the name passed in step #2 above.

```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: domeneshop-dns01
spec:
  acme:
    email: example@example.com
    server: https://acme-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: letsencrypt-account-key
    solvers:
    - dns01:
        webhook:
          groupName: api.domeneshop.no
          solverName: domeneshop
          config:
            APITokenSecretRef:
              key: APIToken
              name: domeneshop-credentials
            APISecretSecretRef:
              key: APISecret
              name: domeneshop-credentials
```

Finally, create the corresponding secret containing your [Domeneshop API credentials](https://api.domeneshop.no/docs/#section/Authentication):

```
kubectl create secret generic domeneshop-credentials \
    --namespace webhook-domeneshop \
    --from-literal=APIToken=<token> \
    --from-literal=APISecret=<secret>
```

**NOTE:** If your cluster is RBAC-enabled and you want to use a `ClusterIssuer` instead, you may have to uncomment the bottom two resources in `deploy/domeneshop-webhook/templates/rbac.yaml` before installing the Helm chart, in order for the webhook to read the credentials secrets in the `cert-manager` namespace.

## Issue a certificate

You should now be ready to issue certificate using DNS01 challenges through the Domeneshop API!

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: test-certificate
spec:
  dnsNames:
  - www.example.com
  issuerRef:
    name: domeneshop-dns01
    kind: Issuer
  secretName: test-certificate-tls
```

Eventually, the certificate should be issued using the webhook:

```shell
% kubectl get certificate
NAME                                                  READY   SECRET                                                AGE
test-certificate                                      True    test-certificate-tls                                  3m36s
```

For troubleshooting, try using `kubectl describe` on the resources related to the issuance (e.g. `certificates.acme.cert-manager.io`, `challenges.acme.cert-manager.io`, `orders.acme.cert-manager.io`). Refer to the [cert-manager documentation](https://cert-manager.io/docs/) for more information.

# Running tests

1. Download required testing binaries:

    ```shell
    make test/kubebuilder
    ```

2. Edit `testdata/domeneshop-webhook/secret.yml` with a valid API token and secret.

3. Run the tests (replace `example.com.` with the FQDN for a domain on your account):

    ```
    TEST_ZONE_NAME=example.com. go test -v .
    ```

    **NOTE:** The tests will create and validate TXT records on your domain.