#!/usr/bin/env bash
set -ex -o pipefail -o errtrace -o functrace

PROJECT_ROOT="$(readlink -e $(dirname "${BASH_SOURCE[0]}")/../)"

pushd ${PROJECT_ROOT}

GOBIN=${PROJECT_ROOT}/bin go install \
   sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0

GOBIN=${PROJECT_ROOT}/bin go install \
  k8s.io/code-generator/cmd/defaulter-gen@v0.21.8 \
  k8s.io/code-generator/cmd/openapi-gen@v0.21.8

bin/controller-gen \
  object:headerFile="./hack/boilerplate.go.txt" \
  paths="api/..."

bin/defaulter-gen \
  --go-header-file "./hack/boilerplate.go.txt" \
  --input-dirs "api/v1beta1" \
  --output-base "." \

bin/openapi-gen \
  --output-file-base "zz_generated.openapi" \
  --go-header-file "./hack/boilerplate.go.txt" \
  --input-dirs "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1" \
  --output-base "api" \
  --output-package "v1beta1"

popd
