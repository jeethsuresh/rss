#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/common.sh
source "${SCRIPT_DIR}/scripts/lib/common.sh"

usage() {
  printf 'Usage: %s [--project-name NAME] [--compose-file FILE] [--volumes] [--no-remove-orphans]\n' "$(basename "$0")"
}

parse_common_args "$@"

DOWN_ARGS=(down)
if [[ "${REMOVE_VOLUMES}" -eq 1 ]]; then
  DOWN_ARGS+=(-v)
fi
if [[ "${REMOVE_ORPHANS}" -eq 1 ]]; then
  DOWN_ARGS+=(--remove-orphans)
fi

docker compose -f "${COMPOSE_FILE}" -p "${PROJECT_NAME}" "${DOWN_ARGS[@]}"
