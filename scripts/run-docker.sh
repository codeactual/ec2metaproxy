#!/usr/bin/env bash

set -o errexit
set -o nounset

SCRIPT_NAME="${0}"

function run_docker {
  local image="${1}"
  local container_name="${2}"
  local host="${3}"
  local config=`readlink -f ${4}`

  docker run                                                \
    -d                                                      \
    --net=host                                              \
    -v "${host}:${host}"                                    \
    -v "${config}:/config.json"                             \
    --name="${container_name}"                              \
    --restart=always                                        \
    "${image}"                                              \
    /config.json
}

function error {
  echo "${@:-}" 1>&2
}

function print_help {
  error "${SCRIPT_NAME} [options]"
  error
  error "Options:"
  error "  --image: metadata proxy docker image (default: ec2mtaproxy:latest)"
  error "  --container-name: name for the local metadata proxy container (default: ec2metaproxy)"
  error "  --host: daemon socket, must match value in config file (default: /var/run/docker.sock)"
  error "  --config: path to JSON file"
}

function main {
  local image="ec2metaproxy:latest"
  local container_name="ec2metaproxy"
  local host="/var/run/docker.sock"
  local config=""

  while [[ ${#} -gt 0 ]]; do
    case "${1}" in
      --image) image="${2}"; shift;;
      --container-name) container_name="${2}"; shift;;
      --host) host="${2}"; shift;;
      --config) config="${2}"; shift;;
      -h|--help)
        print_help
        exit 0;;
      *)
        if [[ -n "${1}" ]]; then
          error "Unknown option: ${1}"
          print_help
          exit 1
        fi
        ;;
    esac

    shift
  done

  run_docker                \
    "${image}"              \
    "${container_name}"     \
    "${host}"               \
    "${config}"
}

main "${@:-}"
