# Same-host WordPress provisioning

Approved scope: self-hosted, explicitly enabled provisioning restricted to configured host-operator tenant owners. Dedicated same-host River worker owns Docker and MariaDB provisioning access; API retains its privileges. Create isolated per-site database/user, pinned WordPress/WP-CLI Compose project and persistent volume, proxy labels, installation and existing Fleet Agent enrollment. Persist encrypted credentials and progress; preserve resources on failures and resume without resetting installed sites. Complete only after installation, enrollment and HTTPS verification.

## Execution

- [x] Add additive provisioning persistence and isolation policies through database specialist.
- [x] Implement authenticated create/status/retry/credential interfaces, durable worker and enrollment adapter through backend specialist; final verification pending.
- [x] Regenerate OpenAPI consumers using repository tooling.
- [x] Add dashboard form, progress, failure guidance, credential retrieval and retry through frontend specialist; final verification pending.
- [x] Add opt-in worker infrastructure and fail-loud local verification preflight through devops specialist. Full Docker/ACME execution awaits pinned images.
- [ ] Security review committed implementation for privileges, credentials, tenancy, concurrency and enrollment; address findings.
- [ ] Run available local tests and integration verification; report blocked prerequisites explicitly.

## Constraints

No push, PR, public DNS changes, remote database operations, deployment or release. Private write-surface specification missing; standalone provisioning scope only. User authorizes proceeding despite missing generators, but generated output must never be hand-edited. Existing unrelated package manifest/lock edits are excluded.

Persistence: additive migration creation/reapplication passed isolated PostgreSQL smoke; no sqlc input queries added. Atlas-compatible hash reconstruction matched original manifest before update. Infrastructure boundary and preflight regression suites passed locally. Repository-path tenant/RLS and runtime verification remain pending.

Security review performed on committed range; acceptance blocked pending resource-ownership and interrupted-worker corrections. Go provisioning unit checks and worker compilation passed. Dashboard targeted suite running. Docker runtime fixture added; local ACME infrastructure images now available. Full enrollment and final integration acceptance remain outstanding.
