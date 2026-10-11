#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/common.sh
source "${SCRIPT_DIR}/common.sh"

fail() {
  echo "common_test.sh: $*" >&2
  exit 1
}

COMPOSE_DEFAULT="$(sed -n 's/.*image: \${RSS_SERVER_IMAGE:-\([^}]*\)}.*/\1/p' "${SCRIPT_DIR}/../../docker-compose.yaml" | head -n 1)"
[[ -n "${COMPOSE_DEFAULT}" ]] || fail "could not parse compose image default"
[[ "${COMPOSE_DEFAULT}" == "docker.io/library/rss-reader-app:latest" ]] || fail "compose image must be fully qualified to avoid stale localhost images"

PROJECT_NAME="rss-reader"
HOST_PORT="8787"
unset RSS_SERVER_IMAGE DEPLOY_IMAGE_TAG COMPOSE_PROJECT_NAME RSS_SERVER_PORT
export_compose_runtime_env
[[ "${RSS_SERVER_IMAGE}" == "${COMPOSE_DEFAULT}" ]] || fail "build image is ${RSS_SERVER_IMAGE}, want compose default ${COMPOSE_DEFAULT}"

# Forge tags the built compose image as {compose_slug}-app:{sha}, then runs
# deploy.sh --project-name rss-reader-staging with DEPLOY_IMAGE_TAG set.
PROJECT_NAME="rss-reader-staging"
unset RSS_SERVER_IMAGE
DEPLOY_IMAGE_TAG="abc123def"
export_compose_runtime_env
[[ "${RSS_SERVER_IMAGE}" == "docker.io/library/rss-reader-app:abc123def" ]] || fail "tagged image is ${RSS_SERVER_IMAGE}, want docker.io/library/rss-reader-app:abc123def"

RSS_SERVER_IMAGE="already-set:tag"
unset DEPLOY_IMAGE_TAG
export_compose_runtime_env
[[ "${RSS_SERVER_IMAGE}" == "already-set:tag" ]] || fail "pre-set RSS_SERVER_IMAGE was overwritten to ${RSS_SERVER_IMAGE}"

echo "common_test.sh: ok"
