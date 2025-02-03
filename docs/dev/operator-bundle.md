# Operator build and deployment


## Building the operator bundle

For development and testing purposes it may be beneficial to build the operator bundle.

The template for the CSV is located [here](config/manifests/bases/nvidia-maintenance-operator.clusterserviceversion.yaml).

Build the bundle:

**Note**:
- `VERSION` should be a valid semantic version
- `DEFAULT_CHANNEL` should be in the following format: `vMAJOR.MINOR`, without the patch version
- `CHANNELS` should include the `DEFAULT_CHANNEL` value and `stable` seperated by a comma
- `IMG` should use SHA256

Here how to obtain the digest:

```bash
skopeo inspect docker://ghcr.io/mellanox/maintenance-operator:v0.2.0 | jq .Digest
"sha256:47d87129d967a5f9d947915b414931e6ba7d42be45186123b531d760b93c6306"
```

```bash
DEFAULT_CHANNEL=v0.2 CHANNELS=stable,v0.2 VERSION=0.2.0 IMG=ghcr.io/mellanox/maintenance-operator@sha256:47d87129d967a5f9d947915b414931e6ba7d42be45186123b531d760b93c6306 make bundle
```

Build the bundle image:

```bash
VERSION=0.2.0 IMAGE_TAG_BASE=ghcr.io/mellanox/maintenance-operator make bundle-build
```

Push the bundle image:

```bash
VERSION=0.2.0 IMAGE_TAG_BASE=ghcr.io/mellanox/maintenance-operator make bundle-push
```

## Deploying the operator

The operator is recommended to be deployed to its own namespace (nvidia-maintenance-operator). Create the namespace.

```bash
cat <<EOF | kubectl create -f -
apiVersion: v1
kind: Namespace
metadata:
  name: nvidia-maintenance-operator
  labels:
    name: nvidia-maintenance-operator
EOF
```

### Download operator-sdk

If needed, download `operator-sdk`:

- Run `make operator-sdk` to download `operator-sdk` to `./bin` directory.
- Or, download manually by following these [instructions](https://sdk.operatorframework.io/docs/installation/#install-from-github-release).

### Deploy the operator using the operator-sdk


```bash
operator-sdk run bundle --namespace nvidia-maintenance-operator ghcr.io/mellanox/maintenance-operator-bundle:v0.2.0
```

If needed, kubeconfig file path can be specified:

```bash
operator-sdk run bundle --kubeconfig /path/to/configfile --namespace nvidia-maintenance-operator ghcr.io/mellanox/maintenance-operator-bundle:v0.2.0
```

Now you should see the `nvidia-maintenance-operator` deployment running in the
`nvidia-maintenance-operator` namespace.

**NOTE**

To remove the operator when installed via `operator-sdk run`, use:

```bash
operator-sdk cleanup --namespace nvidia-maintenance-operator nvidia-maintenance-operator
```

### Add Environment Variables to Operator Deployment in OpenShift

It is possible to add environment variables to operator deployment in OpenShift
using the deployed operator's `Subscription`.

Get the `Subscription` name:

```
kubectl get subscriptions.operators.coreos.com -n nvidia-maintenance-operator
NAME                                     PACKAGE                       SOURCE                                CHANNEL
nvidia-maintenance-operator-v0-1-0-sub   nvidia-maintenance-operator   nvidia-maintenance-operator-catalog   v0.2.0
```

Edit the `Subscription`, and add a section `spec.config.env` with needed vars and values.
For example:

```
spec:
  config:
    env:
      - name: ENABLE_WEBHOOKS
        value: "true"
```
