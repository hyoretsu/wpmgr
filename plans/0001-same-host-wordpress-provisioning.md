# Same-host WordPress provisioning

Approved scope: self-hosted, explicitly enabled provisioning restricted to configured host-operator tenant owners. Dedicated same-host River worker owns Docker and MariaDB provisioning access; API retains its privileges. Create isolated per-site database/user, pinned WordPress/WP-CLI Compose project and persistent volume, proxy labels, installation and existing Fleet Agent enrollment. Persist encrypted credentials and progress; preserve resources on failures and resume without resetting installed sites. Complete only after installation, enrollment and HTTPS verification.

## Execution

- [x] Add additive provisioning persistence and isolation policies through database specialist.
- [x] Implement authenticated create/status/retry/credential interfaces, durable worker and enrollment adapter through backend specialist.
- [x] Regenerate OpenAPI consumers using repository tooling.
- [x] Add dashboard form, progress, failure guidance, credential retrieval and retry through frontend specialist.
- [x] Add opt-in worker infrastructure and fail-loud local verification preflight through devops specialist. Docker runtime verification passed; local ACME verification passed against isolated test infrastructure.
- [x] Security review committed implementation for privileges, credentials, tenancy, concurrency and enrollment; address findings.
- [x] Run local tests, affected builds/typecheck/lint, production-role persistence integration, real Docker installation/enrollment/retry/restart, and isolated ACME verification.

## Constraints

No push, PR, public DNS changes, remote database operations, deployment or release. Private write-surface specification missing; standalone provisioning scope only. User authorizes proceeding despite missing generators, but generated output must never be hand-edited. Existing unrelated package manifest/lock edits are excluded.

Persistence: additive migration creation/reapplication passed isolated PostgreSQL smoke; no sqlc input queries added. Atlas-compatible hash reconstruction matched original manifest before update. Infrastructure boundary and preflight regression suites passed locally. Repository-path tenant/RLS integration passed through the application role; real Docker runtime retry, restart, installation title/password persistence, and database isolation passed.

Security review performed on committed ranges. Resource ownership and interrupted-worker corrections landed; final stopped-helper removal uses inspected immutable container identity and independent review accepted the correction with real Docker replacement proofs. API provisioning tests, worker compilation, vet, API build, and production-role provisioning integration passed. Dashboard targeted tests and scoped lint passed after frontend corrections; full dashboard typecheck, scoped lint, and production build passed after restoring local dependency links and allowing bounded checks sufficient time. Boundary and preflight suites passed. Real Docker integration passed with stubbed enrollment and local TLS fixture. Local ACME proof passed: missing DNS rejects issuance, corrected local mapping yields trusted HTTPS with unpublished backend ports. Real Fleet enrollment integration passed against the production site lifecycle and application-role PostgreSQL pool: existing pending site connected, retry/restart preserved credentials and agent settings, enrollment occurred once, database grants remained isolated. The enrollment fixture stages the built local plugin ZIP; HTTPS package download was not exercised. No production deployment or public DNS verification performed.

## Per-site choice follow-up

Host opt-in enables provisioning as an optional capability. The Add site flow offers connecting existing WordPress or creating new WordPress for each site. Both paths share the dashboard; existing sites retain their hosting and connection behavior. Operator documentation and dashboard choice implemented with the approved labels **Connect existing** and **Create new WordPress**. Review correction preserves pending existing-site creation when switching setup choices. Regression coverage exercises the setup chooser, existing-site enrollment, provisioning availability, and mixed fleets. Final local checks remain pending.
