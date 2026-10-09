#!/usr/bin/env bash
set -euo pipefail
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
cat > "$fixture/docker" <<'MOCK'
#!/usr/bin/env bash
case "$1" in
info|network) exit 0 ;;
image) [[ "${MOCK_MISSING_IMAGE:-false}" == false ]] ;;
compose) printf '%s\n' '{"services":{"api":{},"provisioning-worker":{"cap_drop":["ALL"],"cap_add":["CHOWN"],"environment":{"WPMGR_PROVISIONING_STATE_DIR":"/tmp/provisioning-test"},"volumes":[{"target":"/var/run/docker.sock"},{"type":"bind","source":"/tmp/provisioning-test","target":"/tmp/provisioning-test"}]}}}' ;;
*) exit 9 ;;
esac
MOCK
chmod +x "$fixture/docker"
export PATH="$fixture:$PATH" DOCKER_HOST=unix:///var/run/docker.sock
export WPMGR_PROVISIONING_WORDPRESS_IMAGE=wordpress:6.8.2-php8.3-apache
export WPMGR_PROVISIONING_WPCLI_IMAGE=wordpress:cli-2.12.0-php8.3
export WPMGR_TEST_MARIADB_IMAGE=mariadb:11.4.8
export WPMGR_TEST_CADDY_IMAGE=caddy-test:2.10.0
export WPMGR_TEST_ACME_IMAGE=pebble-test:2.8.0
export WPMGR_PROVISIONING_PROXY_NETWORK=isolated-test
bash "$script_dir/provisioning-integration-preflight.sh"
if MOCK_MISSING_IMAGE=true bash "$script_dir/provisioning-integration-preflight.sh" >"$fixture/out" 2>&1; then
  echo 'FAIL: missing image passed' >&2; exit 1
fi
cat "$fixture/out"
if DOCKER_HOST=tcp://remote.example:2376 bash "$script_dir/provisioning-integration-preflight.sh" >"$fixture/out" 2>&1; then
  echo 'FAIL: remote daemon passed' >&2; exit 1
fi
cat "$fixture/out"
if WPMGR_PROVISIONING_WORDPRESS_IMAGE=wordpress:latest bash "$script_dir/provisioning-integration-preflight.sh" >"$fixture/out" 2>&1; then
  echo 'FAIL: floating image passed' >&2; exit 1
fi
cat "$fixture/out"
echo 'provisioning integration preflight tests: PASS'
