#!/bin/bash

set -e
set -u
set -o pipefail

readonly ROOT_DIR="$(cd "$(dirname "${0}")/.." && pwd)"
readonly ARTIFACTS_DIR="${ROOT_DIR}/build"

function main() {
  mkdir -p "${ARTIFACTS_DIR}"

  pushd "${ROOT_DIR}" > /dev/null || return
    for os in darwin linux; do
      echo "* building jam on ${os}"
      GOOS="${os}" GOARCH="amd64" go build -o "${ARTIFACTS_DIR}/jam-${os}" main.go
      chmod +x "${ARTIFACTS_DIR}/jam-${os}"
    done
    echo "* building jam on windows"
    GOOS="windows" go build -o "${ARTIFACTS_DIR}/jam-windows.exe" main.go
  popd > /dev/null || return
}

main "${@:-}"
