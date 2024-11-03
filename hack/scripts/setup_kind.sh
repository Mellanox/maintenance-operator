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

CLUSTER_NAME="${CLUSTER_NAME:-"mn-op"}"
SCRIPT_DIR="$(dirname "$(realpath "$0")")"
KIND_BIN=${KIND_BIN:-"unknown"}
KUBECTL_BIN=${KUBECTL_BIN:-"kubectl"}
CERT_MANAGER_VERSION=${CERT_MANAGER_VERSION:-"v1.15.3"}
CERT_MANAGER_NAMESPACE=${CERT_MANAGER_NAMESPACE:-"cert-manager"}

function kind() {
    ${KIND_BIN} $@
}

function kubectl() {
    ${KUBECTL_BIN} $@
}

function label_worker_nodes() {
    # label worker nodes as workers. we do this here since doing it in kind config file does not work.
    kubectl label nodes --all node-role.kubernetes.io/worker=
    kubectl label node ${CLUSTER_NAME}-control-plane node-role.kubernetes.io/worker-
}

if [[ "${KIND_BIN}" == "unknown" ]]; then
  echo "KIND_BIN not provided. Aborting." >&2
  exit 1
fi

echo "Creating kind cluster ${CLUSTER_NAME}"
CLUSTER_NAME=${CLUSTER_NAME} envsubst < ${SCRIPT_DIR}/e2e-cluster.yaml.template > ${SCRIPT_DIR}/e2e-cluster.yaml
kind create cluster --config ${SCRIPT_DIR}/e2e-cluster.yaml
rm -f ${SCRIPT_DIR}/e2e-cluster.yaml
label_worker_nodes
