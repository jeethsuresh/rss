#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/common.sh
source "${SCRIPT_DIR}/scripts/lib/common.sh"

usage() {
  printf 'Usage: %s [--project-name NAME] [--compose-file FILE] [--host-port PORT]\n' "$(basename "$0")"
}

parse_common_args "$@"
export_compose_runtime_env

docker compose -f "${COMPOSE_FILE}" -p "${PROJECT_NAME}" build app
