#!/bin/bash

set -e
set -u
set -o pipefail

readonly ROOT_DIR="$(cd "$(dirname "${0}")/.." && pwd)"
readonly ARTIFACTS_DIR="${ROOT_DIR}/build"

# shellcheck source=SCRIPTDIR/.util/print.sh
source "${ROOT_DIR}/scripts/.util/print.sh"

function main {
  local version

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --version|-v)
        version="${2}"
        shift 2
        ;;

      --help|-h)
        shift 1
        usage
        exit 0
        ;;

      "")
        # skip if the argument is empty
        shift 1
        ;;

      *)
        util::print::error "unknown argument \"${1}\""
    esac
  done

  if [[ -z "${version:-}" ]]; then
    usage
    echo
    util::print::error "--version is required"
  fi

  build::jam "${version}"
}

function usage() {
  cat <<-USAGE
package.sh --version <version>
Packages the jam source code into jam binaries.
OPTIONS
  --help               -h            prints the command usage
  --version <version>  -v <version>  specifies the version number to use when packaging jam
USAGE
}

function build::jam(){
  local version
  version="${1}"

  mkdir -p "${ARTIFACTS_DIR}"

  pushd "${ROOT_DIR}" > /dev/null || return
    for os in darwin linux; do
      util::print::info "Building jam on ${os}"
      GOOS="${os}" GOARCH="amd64" go build -ldflags "-X github.com/paketo-buildpacks/jam/commands.jamVersion=${version}" -o "${ARTIFACTS_DIR}/jam-${os}"  main.go
      chmod +x "${ARTIFACTS_DIR}/jam-${os}"
    done
    util::print::info "Building jam on windows"
    GOOS="windows" go build -ldflags "-X github.com/paketo-buildpacks/jam/commands.jamVersion=${version}" -o "${ARTIFACTS_DIR}/jam-windows.exe" main.go
  popd > /dev/null || return
}

main "${@:-}"
