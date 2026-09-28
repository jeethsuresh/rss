#!/usr/bin/env bash
set -euo pipefail

parse_common_args() {
  PROJECT_NAME="${PROJECT_NAME:-rss-reader}"
  COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yaml}"
  HOST_PORT="${HOST_PORT:-8787}"
  DETACH=1
  REMOVE_VOLUMES=0
  REMOVE_ORPHANS=1

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --project-name)
        PROJECT_NAME="$2"
        shift 2
        ;;
      --compose-file)
        COMPOSE_FILE="$2"
        shift 2
        ;;
      --host-port|--port)
        HOST_PORT="$2"
        shift 2
        ;;
      --detach)
        DETACH=1
        shift
        ;;
      --no-detach)
        DETACH=0
        shift
        ;;
      --volumes)
        REMOVE_VOLUMES=1
        shift
        ;;
      --no-remove-orphans)
        REMOVE_ORPHANS=0
        shift
        ;;
      --skip-install)
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        echo "Unknown option: $1" >&2
        usage >&2
        exit 2
        ;;
    esac
  done

  export PROJECT_NAME COMPOSE_FILE HOST_PORT DETACH REMOVE_VOLUMES REMOVE_ORPHANS
}

export_compose_runtime_env() {
  export COMPOSE_PROJECT_NAME="${PROJECT_NAME}"
  export RSS_SERVER_PORT="${HOST_PORT}"
  export RSS_SERVER_IMAGE="${PROJECT_NAME}-app:${DEPLOY_IMAGE_TAG:-latest}"
}
