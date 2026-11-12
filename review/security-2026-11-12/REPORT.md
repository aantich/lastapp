# Lastapp security and foundation review

Remediated locally: see [the fix and verification record](FIXES.md). The findings and captured vulnerable-behavior probes below are historical evidence against the initial commit. The runner now executes secure regression tests.
12 November 2026. Reviewed commit `6daf3afa0d9b09c0b0cbf8027d53f3d08116e832` on `main`.

**Keep the architecture, but fix the foundation before production use.** Ten targeted probes reproduced ten defects. Three are high priority: OIDC identity collisions can bind a login to another user, retirement bypasses physical database quotas, and production accepts unencrypted registry connections. The other findings affect permission correctness, availability, auditing and lifecycle management. They can be addressed within the existing Go and MariaDB design.

This review changed no application source or running demo registry. Concurrent frontend styling work appeared during the review and was preserved; frontend source references below are pinned to the initial commit, and the reproduced backend files still match it. Tests used a disposable registry on the private development MariaDB and removed their own databases and accounts. Review artifacts are local and have not been committed or published.

## Findings by priority

P1 means fix before production deployment. P2 means correct before relying on the affected behavior as a stable platform contract.

| ID | Priority | Finding | Evidence |
| --- | --- | --- | --- |
| S1 | P1 | Distinct OIDC subjects can receive the same internal identity and privileges | Valid signed token with a trailing-space subject received the original user's superadmin session |
| S2 | P1 | Retiring a database releases quota while retaining its physical database and account | A Tenant with `max_databases=1` created two physical databases by retiring the first |
| S3 | P1 | Production registry transport does not enforce TLS | `development=false` accepted TCP with an empty session TLS cipher |
| S4 | P2 | Creating a disabled role silently creates an active role | `POST /roles` with `state=disabled` persisted `active` |
| S5 | P2 | Runtime privilege validation misses global GRANT OPTION | Strict validation accepted `USAGE ON *.* ... WITH GRANT OPTION` plus schema CRUD |
| S6 | P2 | Default collection reads and authorization metadata are unbounded | 205 duplicate grants were accepted; default read returned 206 rows including the administrator grant |
| S7 | P2 | Security audit events lose information needed to reconstruct changes | Grant deletion removed the original row and wrote empty audit details |
| S8 | P2 | Plan reads serialize allocations across unrelated Tenants | One Tenant's policy transaction blocked another Tenant sharing its plan |
| S9 | P2 | Plan downgrade breaks editing of grandfathered dedicated Apps | A rename with unchanged compute mode failed; the same rename without that field succeeded |
| S10 | P2 | Tenant suspension is applied inconsistently to new registrations | A superadmin could register a micro-app in a suspended Tenant while new App creation was rejected |

## S1 OIDC identity comparison can merge accounts

The unique identity columns use `VARCHAR ... COLLATE utf8mb4_bin`. Despite its name, that collation treats trailing spaces as padding. The login callback uses that comparison for both its identity upsert and subsequent user lookup.

The reproduction bootstrapped `review-victim` as an administrator, then completed a real callback with a correctly signed token for the distinct subject `review-victim `. Signature, issuer, audience, expiry and nonce checks passed. The callback issued a session for the original administrator ID, and `/v1/me` returned `platform_admin=true`.

**Impact:** account confusion or takeover when a configured provider can issue distinct subjects that collide under this comparison. Exploitation depends on the provider's subject-generation rules; this is not a demonstrated way to choose arbitrary Google or Microsoft subjects. The application itself fails to preserve exact identity semantics.

**Fix:** use exact identity storage and comparison, such as `VARBINARY` or an appropriate `NO PAD` binary collation for both issuer and subject. Do not trim or normalize identity values. Add a new migration, retaining existing user IDs, and turn the reproduction into a regression test asserting separate identities.

Source: [identity columns](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/migrations/00001_control.sql#L10), [callback upsert and lookup](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/auth.go#L109). [MariaDB documents utf8mb4_bin as PAD SPACE](https://mariadb.com/docs/server/reference/sql-statements/administrative-sql-statements/show/show-collation); [OIDC requires exact string comparison](https://www.openid.net/specs/openid-connect-core-1_0.html#StringOps).

## S2 Retirement bypasses physical resource quotas

Database quota counts exclude `retired` resources. Retirement changes only registry state: the physical database and account remain. An unbound database created through the database-resource API is eligible for retirement, so a Tenant administrator can repeat create, retire, create without exceeding the logical quota.

The probe created two physical databases and accounts under the Starter plan's one-database limit. The first was retired and the second ready. Active-resource accounting reported one.

**Impact:** uncontrolled accumulation of databases, accounts and provider resource costs. The tenant-facing quota is not a physical-resource limit.

**Fix:** keep retained managed resources in quota accounting until they have actually been reclaimed. Initially, make reclamation an explicit operator operation; physical deletion need not be added to tenant self-service. If active logical quotas are also useful, define them separately from retained-resource/storage limits.

Source: [quota calculation](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/tenants.go#L326), [retirement](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/databases.go#L319).

## S3 Production accepts an unencrypted registry connection

`Open` requires an HTTPS public URL in production but does not require a secure registry DSN. It accepts TCP without TLS, as well as the driver's insecure TLS modes. The target-database connection helper has stronger transport checks than the registry path.

The probe reused the local TCP registry connection with `development=false` and an HTTPS public URL. Startup succeeded and `SHOW SESSION STATUS LIKE 'Ssl_cipher'` returned an empty cipher.

**Impact:** the registry contains every Tenant's identity, policy and provisioning state. A network attacker able to intercept an unencrypted connection could read or modify that traffic. HTTPS at the application ingress does not protect database traffic.

**Fix:** fail startup for production TCP registry connections without certificate and hostname validation. Reject plaintext fallback and `skip-verify`; permit explicitly configured local Unix sockets as a separate transport case. Check production OIDC discovery endpoints and redirects for equivalent transport guarantees rather than relying only on the issuer URL prefix.

Source: [configuration and connector setup](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/core.go#L85), [driver TLS modes](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/third_party/go-mariadb/dsn.go#L275).

## S4 Disabled roles are created active

The role creation DTO accepts a `state` field, but the insert omits it. MariaDB supplies the default `active`. Even `state=disabled` or an invalid state value is silently ignored.

**Impact:** an importer or administration client can assign a role believing it is disabled while its grants are effective. The reproduction verified the persisted active state.

**Fix:** either honor and validate creation state or reject the field and make the creation contract explicitly active-only. Separate creation and update DTOs where their accepted fields differ.

Source: [role input and creation](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/access.go#L237).

## S5 Global GRANT OPTION escapes runtime validation

`checkGrants` skips any statement beginning with `GRANT USAGE ON *.* TO` before checking for `WITH GRANT OPTION`. MariaDB can represent global grant authority on precisely such a statement.

The probe created a database account with global `GRANT OPTION` plus schema CRUD. Even strict validation accepted it. That contradicts the intended prohibition on runtime grant authority.

**Fix:** inspect security-relevant options on every grant before treating USAGE as harmless. Keep strict managed-account validation and registered-account validation explicit. Add tests for global and schema grant options and supported role behavior. Avoid interpreting privilege authority from an unchecked string prefix.

Source: [validation order](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/worker.go#L238). [MariaDB explains USAGE options and GRANT OPTION](https://mariadb.com/docs/server/reference/sql-statements/account-management-sql-statements/grant#the-usage-privilege).

## S6 Reads and authorization metadata have no default bound

Pagination is applied only when a caller supplies `page` or `limit`. Otherwise the server reads the entire collection into memory. Grants, groups, group memberships, roles and micro-app registrations have no corresponding plan limits. Semantically identical grants can be inserted repeatedly. Scope evaluation can then produce repeated SQL predicates, and the capabilities endpoint repeats authorization work for each permission.

The probe inserted 205 identical effective grants. The default collection returned 206 rows, while an explicit limit returned 200. This confirms the missing default bound; destructive load testing was not performed.

**Impact:** authorized users can create disproportionate database, CPU and memory work. The console's repeated full reads amplify the problem as directories grow. The single global provisioning worker also processes requests serially without Tenant fairness, so a large slow queue can delay other Tenants.

**Fix:** make list limits mandatory by default, page the console, bound nested member lists, prevent or coalesce equivalent grants, deduplicate scope IDs and add modest per-principal/Tenant request and allocation limits. Add expired-session cleanup. Compute a request's capabilities from one authorization query where practical. Keep these changes local; a distributed policy cache is unnecessary.

Source: [pagination default](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/pagination.go#L10), [grant insertion](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/access.go#L407), [scope predicates](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/auth.go#L271), [capabilities](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/http.go#L169), [worker queue](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/worker.go#L48).

## S7 Audit events cannot reconstruct permission changes

Role creation and permission updates write empty audit details. Grant creation records the role or permission but omits principal and scope. Grant deletion removes the source row and records empty details. Once a grant is removed or a role changes again, the audit alone cannot establish who had which access.

The probe created and revoked a direct permission grant. The revocation event retained its record ID and actor but `details_json` was `{}`. The original grant row no longer existed.

**Fix:** record safe before/after values for security mutations: principal, scope, role/permission IDs, old/new role permission sets and enabled state. Continue excluding secrets. Add pagination/export for audit history; its API currently exposes only the latest 150 events. Preserve append-only database privileges.

Source: [role audit](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/access.go#L280), [grant creation](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/access.go#L411), [grant deletion](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/access.go#L459), [audit retrieval](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/http.go#L217).

## S8 Shared plan rows serialize unrelated Tenants

`policyFor` joins the Tenant to its plan with `FOR UPDATE`. This locks the shared plan row, not just the Tenant. A transaction for one Tenant therefore blocks another Tenant on the same plan. The probe held the first transaction open and the second policy read timed out after 300 milliseconds.

**Fix:** retain the Tenant row lock used for allocation/quota consistency, but read the plan without an exclusive shared-plan lock. Define the policy version/read point used for new allocations. Also keep `mutate` explicitly limited to control-plane operations: its Tenant-wide write serialization should not become the business-record write path for independently scaling Apps.

Source: [policy read](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/tenants.go#L316), [mutation locking](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/core.go#L367).

## S9 Unchanged compute preference breaks grandfathering

The App editor always sends its current `compute_mode`. The backend rechecks dedicated-compute entitlement whenever that field is present, including when its value is unchanged. After a downgrade, an existing dedicated App can no longer be renamed or managed using that form. The same rename succeeds when sent without the compute field.

**Fix:** validate new entitlements when a value changes or a new resource is allocated. Preserve the promised grandfathering of existing resources. Sending only changed UI fields also reduces accidental policy revalidation.

Source: [compute entitlement check](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/inventory.go#L343), [App edit payload](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/web/admin/src/main.tsx#L1356).

## S10 Suspension has inconsistent creation rules

App creation checks the Tenant's active state through `policyFor`. Micro-app creation checks only its App and Space. The shared authorizer bypasses Tenant checks for platform administrators, allowing a superadmin to create a micro-app under a suspended Tenant while new App creation returns a conflict.

This is an administrative consistency defect, not a demonstrated Tenant-user privilege escalation. It makes the meaning of suspension depend on the endpoint.

**Fix:** apply a common active-Tenant allocation check to every creation path. Distinguish administrative repair of existing records from creation of new resources.

Source: [global authorization bypass](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/auth.go#L238), [App creation](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/inventory.go#L182), [micro-app creation](https://github.com/aantich/lastapp/blob/6daf3afa0d9b09c0b0cbf8027d53f3d08116e832/internal/control/inventory.go#L240).

## Boundaries to resolve before the business runtime

These are future security constraints, separate from the ten reproduced defects.

- **Control-plane authority versus business-data authority.** `authorize` currently lets platform administrators pass any action string before checking its domain or Tenant membership. That is convenient for inventory administration but conflicts with the plan's promise that superadmins do not implicitly read Tenant business data. Space/App lifecycle checks also live in individual handlers, not the shared authorizer. Do not reuse this function unchanged as the business-data policy boundary. Introduce explicit action domains and server-resolved active resource context.
- **Generated code versus the administration session.** Arbitrary micro-apps/npm dependencies must not execute with the admin console's origin and session authority. Otherwise they can obtain the readable CSRF token and call management APIs with the user's privileges. Use a separate admin origin, isolated untrusted app contexts and a broker or narrowly scoped, App-bound credentials. This is a browser trust boundary, not a reason to introduce a general workflow engine.
- **Ownership of shared data definitions.** Multiple Apps sharing one Space database need one authoritative schema/migration and invariant contract. Independent App definitions must not rewrite the same tables or weaken another App's rules. Decide that ownership before adding entity compilation.
- **Physical server identity and placement.** Database identity is currently target ID plus case-folded name. DNS aliases can describe the same physical server, and all active targets are available to eligible Tenants. Keep canonical-target registration an operator responsibility and define explicit entitlements before adding private/premium targets.

## Dependency and delivery checks

The frontend `npm audit --json` returned zero known advisories for the locked dependency tree.

The official Go scanner flagged `github.com/go-jose/go-jose/v4 v4.1.3` under [GO-2026-4945](https://pkg.go.dev/vuln/GO-2026-4945), fixed in v4.1.4. The [upstream advisory](https://github.com/go-jose/go-jose/security/advisories/GHSA-78h2-9frx-2jm8) describes JWE decryption with malformed wrapped keys. Lastapp's inspected OIDC path parses and verifies signed JWS tokens; it does not call the documented JWE decryption path. This is an affected dependency to patch, not a confirmed remotely exploitable Lastapp endpoint. The advisory lacks symbol-level precision, so the scanner's displayed signing traces do not prove that decryption is reachable.

Additional delivery gaps from static inspection:

- No CI workflow enforces these security cases. MariaDB 10.11 support is declared but was not executed; actual testing uses 12.3.3. Add a small Go/API/browser/driver-integrity pipeline and a server-version matrix.
- The supplied Vite development proxy does not solve Origin enforcement. Browser writes from port 5173 are rejected by the backend configured for port 8787, and login redirects back to 8787. Keep production checks strict and define one intentional development-origin/proxy setup.
- Internal errors log only their Go type, despite telling users to inspect a request ID. Add redacted SQL error numbers and useful network/TLS categories so operators can diagnose failures without logging credentials or raw provisioning SQL.

## Validation and remediation order

The isolated probes are [review_probe_test.go.txt](review_probe_test.go.txt). Run them with:

```sh
python3 review/security-2026-11-12/run_probes.py
```

The private development MariaDB and `.local/config.json`, `.local/migrate.json` and `.local/state.json` must exist. The runner overlays an additional test into the real `internal/control` package and reuses its API test helper; it does not edit application files. The preserved `.txt` probes assert the original defective behavior. The runner now executes `internal/control/security_test.go`, where PASS means the corrected behavior is enforced. [Captured results](probe-results.txt) cover all ten cases.

1. Fix S1 with a migration and exact-identity regression tests; fix S2 and S3 before deployment.
2. Fix S4–S7, update the flagged dependency, and put negative security cases into CI.
3. Correct S8–S10 and establish the business-data and browser-origin boundaries before extending the runtime.

The existing design has useful protections worth preserving: parameterized data queries, composite foreign keys for Tenant ownership, current grant lookups, protected built-in roles, administrator-loss guards, random hashed sessions, CSRF/Origin checks, credential encryption bound to owner/Tenant, explicit database ownership markers and stable provisioning retries. The earlier passing suite verified those exercised scenarios; it did not establish the stronger guarantees contradicted by these new cases.
