# Same-host WordPress provisioning

Approved scope: self-hosted, explicitly enabled provisioning restricted to configured host-operator tenant owners. Dedicated same-host River worker owns Docker and MariaDB provisioning access; API retains its privileges. Create isolated per-site database/user, pinned WordPress/WP-CLI Compose project and persistent volume, proxy labels, installation and existing Fleet Agent enrollment. Persist encrypted credentials and progress; preserve resources on failures and resume without resetting installed sites. Complete only after installation, enrollment and HTTPS verification.

## Execution

- [x] Add additive provisioning persistence and isolation policies through database specialist.
- [x] Implement authenticated create/status/retry/credential interfaces, durable worker and enrollment adapter through backend specialist; final verification pending.
- [x] Regenerate OpenAPI consumers using repository tooling.
- [x] Add dashboard form, progress, failure guidance, credential retrieval and retry through frontend specialist; final verification pending.
- [x] Add opt-in worker infrastructure and fail-loud local verification preflight through devops specialist. Docker runtime verification passed; local ACME verification passed against isolated test infrastructure.
- [x] Security review committed implementation for privileges, credentials, tenancy, concurrency and enrollment; address findings.
- [ ] Run available local tests and integration verification; report blocked prerequisites explicitly.

## Constraints

No push, PR, public DNS changes, remote database operations, deployment or release. Private write-surface specification missing; standalone provisioning scope only. User authorizes proceeding despite missing generators, but generated output must never be hand-edited. Existing unrelated package manifest/lock edits are excluded.

Persistence: additive migration creation/reapplication passed isolated PostgreSQL smoke; no sqlc input queries added. Atlas-compatible hash reconstruction matched original manifest before update. Infrastructure boundary and preflight regression suites passed locally. Repository-path tenant/RLS integration passed through the application role; real Docker runtime retry, restart, installation title/password persistence, and database isolation passed.

Security review performed on committed ranges. Resource ownership and interrupted-worker corrections landed; final stopped-helper removal uses inspected immutable container identity and independent review accepted the correction with real Docker replacement proofs. API provisioning tests, worker compilation, vet, API build, and production-role provisioning integration passed. Dashboard targeted tests and scoped lint passed after frontend corrections; full typecheck still pending after bounded timeouts. Boundary and preflight suites passed. Real Docker integration passed with stubbed enrollment and local TLS fixture. Local ACME proof passed: missing DNS rejects issuance, corrected local mapping yields trusted HTTPS with unpublished backend ports. Actual Fleet enrollment fixture is committed; execution and full dashboard typecheck remain pending.
