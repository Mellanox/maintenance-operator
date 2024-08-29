#!/bin/bash

#  2024 NVIDIA CORPORATION & AFFILIATES
#
#  Licensed under the Apache License, Version 2.0 (the License);
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an AS IS BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

set -o nounset
set -o pipefail
set -o errexit

if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

CLUSTER_NAME=${CLUSTER_NAME:-"mn-op"}
MINIKUBE_BIN=${MINIKUBE_BIN:-"unknown"}
HELM_BIN=${HELM_BIN:-"unknown"}

CERT_MANAGER_VERSION=${CERT_MANAGER_VERSION:-"v1.15.3"}
CERT_MANAGER_NAMESPACE=${CERT_MANAGER_NAMESPACE:-"cert-manager"}

function helm() {
    ${HELM_BIN} $@
}

function minikube() {
    ${MINIKUBE_BIN} $@
}

# Check for mandatory vars
if [[ "${MINIKUBE_BIN}" == "unknown" ]]; then
  echo "MINIKUBE_BIN not provided. Aborting." >&2
  exit 1
fi

if [[ "${HELM_BIN}" == "unknown" ]]; then
  echo "HELM_BIN not provided. Aborting." >&2
  exit 1
fi

# set minikube profile
minikube profile ${CLUSTER_NAME}
# install helm
echo "Installing cert-manager ${CERT_MANAGER_VERSION}"
helm repo add jetstack https://charts.jetstack.io --force-update
helm install \
  cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version ${CERT_MANAGER_VERSION} \
  --set crds.enabled=true \
  --set prometheus.enabled=false
