# Security remediation — 12 November 2026

All ten reproduced findings from [the original review](REPORT.md) are fixed locally. The existing registry is migrated to version 2, and the rebuilt backend and console are running at <http://127.0.0.1:8787/console/>. Existing user IDs, tenant data and the concurrent UI redesign were preserved. Changes have not been committed or pushed.

| Finding | Implemented correction |
| --- | --- |
| S1: OIDC account collision | Migration 2 changes issuer/subject to `utf8mb4_nopad_bin`. Signed tokens with distinct subjects receive separate identities. Upgrade tests preserve existing user IDs and grants. Identifiers are not trimmed or normalized. |
| S2: Retirement releases physical quota | All retained database-resource records count toward database quota, including retired resources. Retirement keeps data and accounts and does not make room for another database. The console explains this. Physical reclamation remains an explicit operator responsibility. |
| S3: Production registry transport | Production TCP requires verified TLS, hostname validation, no plaintext fallback and TLS 1.2 or newer. Unix sockets remain supported. Production OIDC discovery, authorization, token and JWKS endpoints require HTTPS; HTTP requests and redirect downgrades are rejected. |
| S4: Disabled roles become active | Creation validates and persists the requested role state. Invalid state and permission-domain values are rejected. |
| S5: Global GRANT OPTION | Every grant is checked for grant authority before global USAGE is skipped. Both managed and registered runtime validation reject global GRANT OPTION; legitimate schema CRUD still passes. |
| S6: Unbounded collections and metadata | Collection reads default to 100 rows, capped at 200, with stable ordering, server search and pagination metadata. The console, Tenant selector and entity pickers page/search on the server. Group previews contain at most ten members, fetched together, with a separate paged member API. Equivalent grants are rejected by the API and a database unique constraint; migration coalesces old duplicates while retaining audit evidence. Scope IDs are deduplicated and capabilities use one grant query. Administrative metadata, request rates and pending allocations have finite ceilings. Provisioning rotates between Tenants; expired sessions are removed in bounded batches. |
| S7: Incomplete security audit | Grants retain principal, scope and permission/role details after revocation. Roles record permission sets and state before and after changes. Membership, group, group-member and global user-state changes also retain their relevant old/new values. Credentials are excluded. Audit history is accessible through bounded JSON pages and search, with append-only runtime privileges preserved. |
| S8: Shared plan lock | Allocation retains the Tenant lock but reads the shared plan without `FOR UPDATE`. Unrelated Tenants no longer serialize on their common plan. |
| S9: Grandfathered App edits fail | Dedicated-compute entitlement is checked when the preference changes to dedicated. An unchanged preference survives downgrade and ordinary metadata edits. Generic console edits send changed fields. |
| S10: Inconsistent suspension | New micro-app registrations perform the same active-Tenant allocation check as Apps, Spaces and databases, including for platform administrators. |

The dependency patch upgrades `go-jose/v4` to 4.1.4. Redacted diagnostics now include SQL numbers/states and network/TLS categories with request or resource IDs, without printing raw server messages or provisioning SQL. The development UI uses one explicit loopback origin; login returns to port 5173 and authenticated writes pass through its proxy. Production origin checks stay strict.

The control-plane action catalog now fails closed for unknown domains, including for superadmins. Space/App ownership is resolved on the server. Production admin cookies use the `__Host-` prefix, and `app_origin` rejects the admin hostname even on a different port. [The provisioning plan](../../PROVISIONING_PLAN.md) records the separate business-policy boundary, isolated generated-code host, Space-owned schema contract and canonical target responsibility. These safeguards do not implement the later business runtime, entity compiler, private target placement or App credential broker.

Verification completed against the existing development installation:

- Full Go/API tests with the race detector: passed. The secure suite includes all ten findings plus upgrade preservation, metadata/member bounds, role audit changes, session expiry cleanup and worker fairness.
- `go vet ./...`: passed.
- Frontend TypeScript/Vite production build: passed.
- Chrome/Playwright: **9/9 passed**, including real provisioning, registration, permission checks, the existing appearance/mobile scenarios, paging/search beyond 100 records, and login plus writes through the Vite proxy.
- `govulncheck` 1.9.0: **no vulnerabilities found**.
- All **42** pinned MariaDB driver snapshot hashes: verified.
- `git diff --check`: passed.

A GitHub Actions workflow now defines backend/race/static checks, secure regressions, the dependency scan, driver integrity and browser tests using the same MariaDB release line as the local installation. It is configured locally; no remote CI result is claimed before these changes are published.

Re-run with `make integration`, or run just the isolated security regressions with `python3 review/security-2026-11-12/run_probes.py`. The original `.txt` vulnerable-behavior probes and captured results remain historical evidence. The runner now executes assertions of corrected behavior. Disposable security registries and their owned resources are removed; the existing integration/browser fixtures retain their established inspection behavior.

Operational changes: deploy migration 2 before serving the new binary; take the usual registry backup first. Existing production browser sessions need a fresh sign-in because the secure cookie names changed. Per-process rate limits are fairness safeguards, not a distributed billing quota. Older audit entries remain historical and cannot acquire details that were never recorded.
