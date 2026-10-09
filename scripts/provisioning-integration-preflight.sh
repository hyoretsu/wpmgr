#!/usr/bin/env bash
# Read-only, no-pull preflight for same-host provisioning integration fixtures.
set -euo pipefail
for tool in docker python3 timeout; do
  command -v "$tool" >/dev/null || { echo "missing required tool: $tool" >&2; exit 1; }
done
endpoint=${DOCKER_HOST:-$(docker context inspect --format '{{.Endpoints.docker.Host}}')}
case "$endpoint" in unix://*) ;; *) echo 'integration requires local Unix Docker socket' >&2; exit 1 ;; esac
timeout 15 docker info >/dev/null
for key in WPMGR_PROVISIONING_WORDPRESS_IMAGE WPMGR_PROVISIONING_WPCLI_IMAGE \
  WPMGR_TEST_MARIADB_IMAGE WPMGR_TEST_CADDY_IMAGE WPMGR_TEST_ACME_IMAGE; do
  value=${!key:-}
  if [[ -z "$value" || "$value" == *:latest || ( "$value" != *:* && "$value" != *@sha256:* ) ]]; then
    echo "missing exact-version image setting: $key" >&2
    exit 1
  fi
  if ! timeout 15 docker image inspect "$value" >/dev/null 2>&1; then
    echo "required local image unavailable: $key (preload separately; no pull attempted)" >&2
    exit 1
  fi
done
: "${WPMGR_PROVISIONING_PROXY_NETWORK:?set existing isolated integration proxy network}"
timeout 15 docker network inspect "$WPMGR_PROVISIONING_PROXY_NETWORK" >/dev/null
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
timeout 15 docker compose -f "$repo_root/infra/docker-compose.yml" \
  -f "$repo_root/infra/docker-compose.provisioning.yml" --profile provisioning \
  config --no-env-resolution --format json | python3 "$repo_root/scripts/check-provisioning-boundary.py"
echo 'provisioning integration preflight: PASS (runtime assertions still required)'
