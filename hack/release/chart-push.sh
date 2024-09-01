#!/bin/bash
set -ex

# github repo owner: e.g mellanox
GITHUB_REPO_OWNER=${GITHUB_REPO_OWNER:-}
# github api token with package:write permissions
GITHUB_TOKEN=${GITHUB_TOKEN:-}
# github tag e.g v1.2.3
GITHUB_TAG=${GITHUB_TAG:-}

BASE=${PWD}
HELM_CMD="${BASE}/bin/helm"
HELM_CHART=${BASE}/deployment/maintenance-operator-chart
HELM_CHART_VERSION=${GITHUB_TAG#"v"}
HELM_CHART_TARBALL="maintenance-operator-chart-${HELM_CHART_VERSION}.tgz"

if [ -z "$GITHUB_REPO_OWNER" ]; then
    echo "ERROR: GITHUB_REPO_OWNER must be provided as env var"
    exit 1
fi

if [ -z "$GITHUB_TOKEN" ]; then
    echo "ERROR: GITHUB_TOKEN must be provided as env var"
    exit 1
fi

if [ -z "$GITHUB_TAG" ]; then
    echo "ERROR: GITHUB_TAG must be provided as env var"
    exit 1
fi

$HELM_CMD package ${HELM_CHART}
$HELM_CMD registry login ghcr.io -u ${GITHUB_REPO_OWNER} -p ${GITHUB_TOKEN}
$HELM_CMD push ${HELM_CHART_TARBALL} oci://ghcr.io/${GITHUB_REPO_OWNER}
