#!/usr/bin/env bash

set -o errexit
set -o nounset

SCRIPT_NAME="${0}"

function setup_firewall {
  local container_iface="${1}"
  local proxy_port="${2}"
  local metadata_ip="${3}"
  local metadata_port="${4}"
  local force="${5}"

  local drop_args=(
      -I INPUT
      -p tcp
      --dport "${proxy_port}"
      ! -i "${container_iface}"
      -j DROP
  )

  echo "Drop traffic to ${proxy_port} not from container interface ${container_iface}"
  if [ "$force" = "1" ]; then
      iptables "${drop_args[@]}"
  else
      echo -e "iptables ${drop_args[@]}\n"
  fi

  echo "Redirect any metadata requests from containers to the proxy service"
  local proxy_ip=$(ifconfig "${container_iface}" | grep "inet addr" | awk -F: '{print $2}' | awk '{print $1}')

  local forward_args=(
      -t nat
      -I PREROUTING
      -p tcp
      -d "${metadata_ip}" --dport "${metadata_port}"
      -j DNAT
      --to-destination "${proxy_ip}:${proxy_port}"
      -i "${container_iface}"
  )

  if [ "$force" = "1" ]; then
      iptables "${forward_args[@]}"
  else
      echo "iptables ${forward_args[@]}"
      echo -e "\nUse --force to disable dry-run mode."
  fi
}

function error {
  echo "${@:-}" 1>&2
}

function print_help {
  error "${SCRIPT_NAME} [options]"
  error
  error "Options:"
  error "  --container-iface: [required] container bridge network interface (example: docker0)"
  error "  --proxy-port: port on the container interface that the metadata proxy is bound to"
  error "                (default: 18000)"
  error "  --metadata-ip: IP of the EC2 metadata service (default: 169.254.169.254)"
  error "  --metadata-port: Port of the EC2 metadata service (default: 80)"
  error "  --force: Disable the default dry-run mode"
}

function main {
  local container_iface="" # docker0, etc
  local proxy_port="18000"
  local metadata_ip="169.254.169.254"
  local metadata_port="80"
  local force=""

  if [[ $EUID -ne 0 ]]; then
    error "This script must be run as root"
    exit 1
  fi

  while [[ ${#} -gt 0 ]]; do
    case "${1}" in
      --container-iface) container_iface="${2}"; shift;;
      --proxy-port) proxy_port="${2}"; shift;;
      --metadata-ip) metadata_ip="${2}"; shift;;
      --metadata-port) metadata_port="${2}"; shift;;
      --force) force="1";;
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

  if [[ -z "${container_iface}" ]]; then
    error "ERROR: --container-iface is required (example: docker0)"
    print_help
    exit 1
  fi

  if [[ "$force" = "" ]]; then
      echo -e "Dry run:\n"
  fi

  setup_firewall            \
    "${container_iface}"    \
    "${proxy_port}"         \
    "${metadata_ip}"        \
    "${metadata_port}"      \
    "${force}"
}

main "${@:-}"
