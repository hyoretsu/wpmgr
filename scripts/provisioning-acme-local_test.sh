#!/usr/bin/env bash
# Isolated real ACME issuance; cached images only, no public DNS or host listeners.
set -euo pipefail
for tool in docker curl python3 timeout rg seq; do
  command -v "$tool" >/dev/null || { echo "missing required tool: $tool" >&2; exit 1; }
done
endpoint=${DOCKER_HOST:-$(docker context inspect --format '{{.Endpoints.docker.Host}}')}
[[ "$endpoint" == unix://* ]] || { echo 'local Docker socket required' >&2; exit 1; }
pebble_image=ghcr.io/letsencrypt/pebble:2.10.1
proxy_image=ghcr.io/hyoretsu/caddy-docker-cloudflare-l4-proxy@sha256:3c2f227d4152c6ac24a2fdc635f1dc5564c1690b612cea2ece92d1977fdfab36
for image in "$pebble_image" "$proxy_image"; do timeout 15 docker image inspect "$image" >/dev/null; done
fixture=$(mktemp -d /tmp/wpmgr-acme.XXXXXX)
id=wpmgr-acme-${fixture##*.}
network=$id
prefix=$id
domain=${id,,}.invalid
cleanup() {
  for name in "$id-proxy" "$id-backend" "$id-ca" "$id-extract"; do
    docker logs "$name" >"$fixture/$name.log" 2>&1 || true
    docker rm -f "$name" >/dev/null 2>&1 || true
  done
  docker network rm "$network" >/dev/null 2>&1 || true
  echo "ACME evidence: $fixture"
}
trap cleanup EXIT
docker network create --internal "$network" >/dev/null
docker create --name "$id-extract" "$pebble_image" >/dev/null
docker cp "$id-extract:/test" "$fixture/test"
docker rm "$id-extract" >/dev/null
python3 - "$fixture/test/config/pebble-config.json" <<'PY'
import json,sys
p=sys.argv[1]
s=json.load(open(p)); s['pebble']['httpPort']=80; s['pebble']['tlsPort']=443
json.dump(s,open(p,'w'))
PY
cat > "$fixture/Caddyfile" <<EOF
{
  cert_issuer acme {
    dir https://pebble:14000/dir
    trusted_roots /fixture/test/certs/pebble.minica.pem
    disable_tlsalpn_challenge
  }
  auto_https disable_redirects
}
EOF
printf 'isolated provisioning backend\n' > "$fixture/index.html"
# Separate label namespace prevents the host's existing proxy consuming test routes.
docker run -d --pull never --name "$id-backend" --network "$network" \
  -v "$fixture:/fixture:ro" --label "$prefix=$domain" \
  --label "$prefix.reverse_proxy={{upstreams 8080}}" --label "${prefix}_ingress_network=$network" \
  "$proxy_image" caddy file-server --root /fixture --listen :8080 >/dev/null
docker run -d --pull never --name "$id-ca" --network "$network" --network-alias pebble \
  -e PEBBLE_VA_NOSLEEP=1 -v "$fixture/test:/test:ro" "$pebble_image" >/dev/null
docker run -d --pull never --name "$id-proxy" --network "$network" \
  -v /var/run/docker.sock:/var/run/docker.sock:ro -v "$fixture:/fixture:ro" \
  "$proxy_image" caddy docker-proxy --label-prefix "$prefix" \
  --ingress-networks "$network" --caddyfile-path /fixture/Caddyfile >/dev/null
proxy_ip=$(docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$id-proxy")
failed=false
for attempt in $(seq 1 40); do
  docker logs "$id-proxy" > "$fixture/missing-dns.log" 2>&1
  if rg -q 'no such host|NXDOMAIN|lookup .*invalid' "$fixture/missing-dns.log"; then failed=true; break; fi
  sleep 1
done
[[ "$failed" == true ]] || { echo 'FAIL: missing DNS never rejected validation' >&2; exit 1; }
echo 'missing DNS ACME validation: rejected'
docker rm -f "$id-ca" >/dev/null
docker run -d --pull never --name "$id-ca" --network "$network" --network-alias pebble \
  --add-host "$domain:$proxy_ip" -e PEBBLE_VA_NOSLEEP=1 \
  -v "$fixture/test:/test:ro" "$pebble_image" >/dev/null
# Export the issuance root (distinct from the ACME directory TLS root).
ca_ip=$(docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$id-ca")
for attempt in $(seq 1 20); do
  if curl --noproxy '*' --fail --silent --max-time 2 --insecure "https://$ca_ip:15000/roots/0" > "$fixture/issuance-root.pem"; then break; fi
  sleep 1
done
[[ -s "$fixture/issuance-root.pem" ]] || { echo 'FAIL: issuance root unavailable' >&2; exit 1; }
# Restart proxy after expected rejection.
docker rm -f "$id-proxy" >/dev/null
docker run -d --pull never --name "$id-proxy" --network "$network" --ip "$proxy_ip" \
  -v /var/run/docker.sock:/var/run/docker.sock:ro -v "$fixture:/fixture:ro" \
  "$proxy_image" caddy docker-proxy --label-prefix "$prefix" \
  --ingress-networks "$network" --caddyfile-path /fixture/Caddyfile >/dev/null
issued=false
for attempt in $(seq 1 90); do
  if curl --noproxy '*' --fail --silent --show-error --max-time 2 \
    --resolve "$domain:443:$proxy_ip" --cacert "$fixture/issuance-root.pem" \
    "https://$domain/" > "$fixture/https-body" 2> "$fixture/https-error"; then issued=true; break; fi
  sleep 1
done
[[ "$issued" == true ]] || { cat "$fixture/https-error" >&2; echo 'FAIL: trusted HTTPS unavailable' >&2; exit 1; }
rg -q 'isolated provisioning backend' "$fixture/https-body"
[[ $(docker inspect --format '{{json .HostConfig.PortBindings}}' "$id-backend") == '{}' || $(docker inspect --format '{{json .HostConfig.PortBindings}}' "$id-backend") == null ]]
echo 'ACME local integration: PASS (issued trusted HTTPS; unpublished backend)'
