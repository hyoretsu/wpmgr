# Same-host WordPress provisioning

Approved scope: self-hosted, explicitly enabled provisioning restricted to configured host-operator tenant owners. Dedicated same-host River worker owns Docker and MariaDB provisioning access; API retains its privileges. Create isolated per-site database/user, pinned WordPress/WP-CLI Compose project and persistent volume, proxy labels, installation and existing Fleet Agent enrollment. Persist encrypted credentials and progress; preserve resources on failures and resume without resetting installed sites. Complete only after installation, enrollment and HTTPS verification.

## Execution

- [x] Add additive provisioning persistence and isolation policies through database specialist.
- [ ] Implement authenticated create/status/retry/credential interfaces, durable worker and enrollment adapter through backend specialist.
- [ ] Regenerate OpenAPI consumers using repository tooling.
- [ ] Add dashboard form, progress, actionable errors and retry through frontend specialist.
- [x] Add opt-in worker infrastructure and fail-loud local verification preflight through devops specialist. Full Docker/ACME execution awaits pinned images.
- [ ] Security review committed implementation for privileges, credentials, tenancy, concurrency and enrollment; address findings.
- [ ] Run available local tests and integration verification; report blocked prerequisites explicitly.

## Constraints

No push, PR, public DNS changes, remote database operations, deployment or release. Private write-surface specification missing; standalone provisioning scope only. User authorizes proceeding despite missing generators, but generated output must never be hand-edited. Existing unrelated package manifest/lock edits are excluded.

Persistence: additive migration creation/reapplication passed isolated PostgreSQL smoke; no sqlc input queries added. Atlas-compatible hash reconstruction matched original manifest before update. Infrastructure boundary and preflight regression suites passed locally. Repository-path tenant/RLS and runtime verification remain pending.
