# Same-host WordPress provisioning

Provisioning is an explicit self-hosted opt-in. Set
`WPMGR_PROVISIONING_ENABLED=true` and configure the host operator organization's
UUID in `WPMGR_PROVISIONING_OPERATOR_TENANT_ID`. Only that organization's signed-in
owners can create, inspect, retry, or retrieve credentials. Hosted installations
cannot enable provisioning. This host setting enables a capability; it does not
select how every site is added. Provisioning remains optional for each new site.
One organization can manage existing WordPress sites alongside sites provisioned
on this host, in the same dashboard. Existing sites are never automatically
migrated into Docker or converted into provisioned sites.

## Host prerequisites

Provide an existing local Docker daemon with Compose, MariaDB, a Docker-label-aware
Caddy reverse proxy, and its existing external proxy network. Configure public DNS
for each requested domain to reach that proxy and ensure certificate issuance can
succeed. Provisioning does not create the shared proxy, MariaDB server, or DNS
records. WordPress and the worker publish no host ports; the proxy reaches
WordPress over the configured Docker network.

Preload approved, pinned WordPress and WP-CLI images into the host daemon. Specify
explicit version tags or digests, never `latest`. Runtime startup uses
`--pull never`. The approved Fleet Agent package and control-plane URLs must use
HTTPS and be reachable from the WordPress environment.

MariaDB must support database comments and
`information_schema.SCHEMATA.SCHEMA_COMMENT`. Provisioned databases carry their
provisioning record UUID as their ownership comment. Use a dedicated provisioner
account able to create databases and users, grant site-database privileges, read
schema metadata, and `SELECT` from `mysql.user`. Grant authority must cover the
privileges assigned to each new site database. Keep these credentials exclusive
to the worker; WordPress receives its own per-site database user and password.
Configure the MariaDB address reachable from both worker and site containers;
TLS defaults to enabled in the Compose overlay.

## Configuration and local command examples

Use `infra/provisioning/operator.env.example` for nonsecret Compose settings and
`infra/provisioning/worker.env.example` for the separate worker secret file.
Copy and edit them outside the repository, for example under `/etc/wpmgr/`.
The worker file holds the existing control-plane application-role database
password, the same age identity used by the control plane, and the MariaDB
provisioner credentials. Restrict that file to mode `0600`. Never put provisioner
credentials in the shared repository-root `.env`, which other services load.

Pre-create the absolute state directory with mode `0700`; the host and worker must
use the identical path. Per-request state contains a private Compose document and
secret files. Secret files use mode `0600` and WordPress/WP-CLI ownership UID/GID
`33:33`. Preserve the state directory and the control-plane age identity across
worker restarts and backups.

The overlay gives only `provisioning-worker` the Docker socket and state mount.
Socket access grants host-level authority. The worker drops capabilities except
`CHOWN`, needed for the secret-file ownership above. The API does not receive
these mounts or the provisioner secret file.

Illustrative local commands from the repository root, after configuring the
existing base installation and both files. These examples are not an execution
record; the build may fetch dependencies unless already cached.

```sh
install -d -m 0700 /var/lib/wpmgr/provisioning
chmod 600 /etc/wpmgr/provisioning-worker.env

docker compose --env-file /etc/wpmgr/operator.env \
  -f infra/docker-compose.yml -f infra/docker-compose.provisioning.yml \
  --profile provisioning build api provisioning-worker

docker compose --env-file /etc/wpmgr/operator.env \
  -f infra/docker-compose.yml -f infra/docker-compose.provisioning.yml \
  --profile provisioning up -d api provisioning-worker
```

Set `WPMGR_PROVISIONING_WORKER_ENV_FILE` to the worker secret file and
`WPMGR_PROVISIONING_STATE_DIR` to the prepared path. API and worker must use the
same operator settings. A request saves its runtime configuration; retry uses
that saved configuration rather than adopting subsequent environment edits.

## Adding a site

Start with **Add site** and choose the path for that site:

- **Connect existing** connects a site already running on its current
  host through Fleet Agent. It does not create or move hosting resources.
- **Create new WordPress** provisions a new installation on this Docker host.
  This choice is available only when provisioning is enabled and the signed-in
  user is an owner of the configured host-operator organization.

Both paths add sites to the same dashboard. Provisioning progress and recovery
controls apply only to sites created through the provisioning path; existing
connections retain their normal connection flow.

## Operation and recovery

For **Create new WordPress**, submit a bare domain, optional site title,
administrator username, and administrator email through the provisioning form.
The title defaults to the domain. Passwords are generated and stored
encrypted. The request creates a durable job and site record. Status shows
progress through database, volume, WordPress, installation, agent enrollment,
and HTTPS verification. Completion requires installation, enrollment, and a
verified HTTPS response from the provisioned installation.

The authenticated API exposes creation at `POST /api/v1/provisioning`, status at
`GET /api/v1/sites/{siteId}/provisioning`, retry at
`POST /api/v1/sites/{siteId}/provisioning/retry`, and administrator credentials at
`GET /api/v1/sites/{siteId}/provisioning/credentials`. Retrieve credentials through
the dashboard's explicit credential action and keep them in an appropriate
password store. Status responses do not disclose passwords.

Failure retains the database/user, Docker volume/container, state files, and
stored credentials. Diagnose the reported stage and error code, correct the
host prerequisite, then retry the existing request. Retry checks resource
ownership and preserves an installed site's content and credentials. Do not
remove database comments, reset passwords, delete the volume, or create a new
request as a substitute for recovery. A collision needs manual operator
inspection; do not overwrite an unrelated resource. If abandoning a request,
retain resources until their ownership and backup needs have been reviewed;
provisioning supplies no automatic rollback or cleanup.

For an isolated certificate-issuance fixture, inspect and run
`scripts/provisioning-acme-local_test.sh` only against the local Docker socket
with its required images already cached. It uses an internal test network,
local ACME CA, and separate proxy-label namespace, without public DNS changes or
published host listeners. This fixture does not verify a site's public DNS or
production certificate configuration.
