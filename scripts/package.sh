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
  local platforms=()

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --version|-v)
        version="${2}"
        shift 2
        ;;

      --platform|-p)
        platforms+=("${2}")
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

  if [[ ${#platforms[@]} -eq 0 ]]; then
    platforms=("darwin/amd64" "linux/amd64" "linux/s390x" "windows/amd64" "darwin/arm64" "linux/arm64" "windows/arm64")
  fi

  validate::platforms "${platforms[@]}"
  build::jam "${version}" "${platforms[@]}"
}

function usage() {
  cat <<-USAGE
package.sh --version <version> [--platform <os>/<arch> ...]
Packages the jam source code into jam binaries.
OPTIONS
  --help               -h             prints the command usage
  --version <version>  -v <version>   specifies the version number to use when packaging jam
  --platform <os>/<arch>  -p <os>/<arch>  build only this GOOS/GOARCH (e.g. linux/amd64). May be repeated.
                      If omitted, builds all supported combinations
                      linux:  amd64, arm64, s390x,
                      darwin: amd64, arm64,
                      windows: amd64, arm64.
USAGE
}

function build::jam(){
  local version="${1}"
  shift
  local platforms=("$@")

  mkdir -p "${ARTIFACTS_DIR}"

  local platform os arch output

  pushd "${ROOT_DIR}" > /dev/null || return
    for platform in "${platforms[@]}"; do
      os="${platform%%/*}"
      arch="${platform##*/}"
      util::print::info "Building jam on ${os} for ${arch}"

      output="${ARTIFACTS_DIR}/jam-${os}"
      if [[ "${arch}" != "amd64" ]]; then
        output="${output}-${arch}"
      fi
      if [[ "${os}" == "windows" ]]; then
        output="${output}.exe"
      fi

      GOOS="${os}" \
      GOARCH="${arch}" \
      CGO_ENABLED=0 \
        go build \
          -ldflags "-X github.com/paketo-buildpacks/jam/v2/commands.jamVersion=${version}" \
          -o "${output}" \
          main.go

      chmod +x "${output}"
    done
  popd > /dev/null || return
}

function validate::platforms() {
  local platform os arch

  for platform in "$@"; do
    if [[ "${platform}" != *"/"* ]] || [[ "${platform}" == *"/"*"/"* ]]; then
      util::print::error "invalid platform \"${platform}\" (expected OS/ARCH with a single slash, e.g. linux/amd64)"
    fi

    os="${platform%%/*}"
    arch="${platform##*/}"
    if [[ -z "${os}" || -z "${arch}" ]]; then
      util::print::error "invalid platform \"${platform}\" (OS and ARCH must be non-empty)"
    fi

    case "${os}" in
      darwin|linux|windows) ;;
      *)
        util::print::error "invalid OS \"${os}\" in \"${platform}\" (supported: darwin, linux, windows)"
        ;;
    esac

    case "${arch}" in
      amd64|arm64|s390x) ;;
      *)
        util::print::error "invalid ARCH \"${arch}\" in \"${platform}\" (supported: amd64, arm64)"
        ;;
    esac
  done
}

main "${@:-}"
