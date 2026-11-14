# Lastapp verification

## Follow-up workspace/release review fixes — 14 November 2026

All four findings from the next review are fixed:

- Undeployed release inspection uses the shared workspace ownership/delegation
  guard. Another human manager or another grant for the same human cannot read
  private snapshots through the API/MCP. Only successful deployments expose a
  snapshot as authorized production history, which remains inspectable after
  workspace archive without opening its private backing Space.
- Preview activation validates the settled Space snapshot and identity mappings
  inside the same locked mutation transaction that selects the client release.
  The regression observes the actual preview request waiting on the Tenant lock,
  commits a concurrent role change and verifies rejection without client changes.
- Startup rejects console hostnames equal to or below the App base hostname,
  including case/trailing-dot/port variations and UUID-shaped console names.
  Startup and static routing use one shared hostname namespace definition.
- Workspace App creation/restoration share one quota check. Development uses
  its metadata cap; production creation/restoration and deployment retain the
  subscription quota. The console explains this distinction without copying
  backend limits. Current workspace/MCP guides and runtime agent context agree.

Executed checks:

- Full backend race/security suite using both local fixture configurations:
  `go test -race -count=1 ./...` **PASS**, control package **160.511 seconds**.
- Focused new regressions with race detection: **PASS**, **10.005 seconds**.
  Coverage includes private API/MCP reads, completed versus queued/failed release
  history, archived workspaces, real concurrent preview mutation, separate quota
  limits, restoration at the workspace cap and deployment quota rejection.
- Full Chrome final rerun: **28/28 PASS (1.9 minutes)**. The initial run passed
  27/28; its expired-session
  test cleared cookies while initial collection requests were still starting.
  The UI correctly returned to login before the test clicked Sign out. The
  isolated unchanged test then passed. Its setup now waits for the collection
  and opens the menu before expiry, and asserts the actual logout 401 response;
  that check passes in the complete final rerun.
- Console TypeScript/Vite build, `go vet ./...`, canonical schema parity,
  `git diff --check`, **42 pinned driver files** and **23 current-guide local
  links**: **PASS**. Desktop/mobile workspace-create screenshots were inspected;
  the new browser flow verifies successful development App creation at full
  production capacity with unchanged production inventory and no mobile overflow.
- Local service rebuilt/restarted, registry remains at migration 12; `/readyz`
  returns **ready**. Existing local data and keys are preserved. No commit/push.

Local MariaDB and installed Chrome were used; no remote SaaS deployment or native
Codex/Claude connection was exercised. Logs are retained in ignored
`.local/review-pass2/`, including the initial browser failure and its reruns.

## Workspace/release review fixes — 14 November 2026

All five reported findings are fixed. Release storage publications now stop at
`prepared`; role states, entity versions/ACL projections, App policies and client
selection commit together. Physical version checks deny old runtime snapshots
across activation. Failure rolls back live registry authority, clears pending
versions and restores only verified compatible gates. Gate opening is idempotent
across a process loss before registry commit.

Published ACL references to subsequently disabled users/teams copy into a
workspace without authorizing those recipients; ordinary authoring still rejects
new inactive recipients. App host routing canonicalizes default ports and keeps
the static hostname namespace separate from management for alternate port forms.
The shared App deployment-status operation restores UI/MCP context after a lost
response or reload; transient UI polling errors retry automatically.

Executed against the final implementation:

- Complete backend race/security suite with both local fixture configurations:
  `go test -race -count=1 ./...` **PASS**; control package **164.072 seconds**.
  This includes role-reactivation/removal ordering, inactive user/team baselines,
  lost-build failure, activation-transaction failure, old API restoration,
  interrupted gate opening/restart, status authorization and cross-agent denial.
- Full Chrome suite: **27 passed (1.8 minutes)**. A real local React build is
  uploaded, previewed against isolated data and deployed to production. Status
  survives reload; an injected 503 is retried without user action; failed jobs
  remain visible after reload. Desktop/mobile screenshots were inspected and
  the mobile viewport has no horizontal overflow.
- Console TypeScript/Vite build, `go vet ./...`, canonical schema parity,
  `git diff --check` and all **42 pinned driver files**: **PASS**.
- Local service rebuilt/restarted with registry migration 12; `/readyz`: **ready**.
  Existing local data and keys preserved. Changes remain uncommitted.

The initial full backend pass also succeeded before the additional restart
regression identified MariaDB's zero-changed-row retry behavior; the idempotence
fix and the final full rerun above supersede that receipt. No alternate MariaDB
version, remote SaaS deployment or native remote-agent session was tested.
Compatible DDL/draft metadata can remain after deployment failure, and changed
entity APIs can pause during preparation; no destructive rollback is provided.


## Agent onboarding documentation and final review — 14 November 2026

AGENTS.md now directs new agents to current architecture, source contracts, API
boundaries, security rules and verification commands. CLAUDE.md imports those
same instructions. DEVELOPMENT.md maps the implementation and separates shipped
features from future client hosting, design packages, runners and connectors.
MCP.md documents connection/consent, all management tools, context discovery,
entity authoring, delegation ceilings, replay/revisions and worker guardrails.
README links to these guides; foundation/entity guides and historical-plan status
notes were reconciled with the implemented runtime and MCP.

The canonical SQL document was missing the MCP additions. It now derives from
all executable migrations. The existing schema utility supports offline `--check`
and `--write`; CI checks parity alongside driver integrity. Its default database
validation behavior remains intact.

Executed checks:

- **120 local Markdown link targets valid**; the MCP guide matches all **14** tool
  names from the Go catalog; CLAUDE.md imports AGENTS.md.
- Offline schema parity and generation: **PASS**. An isolated temporary fixture
  confirms stale SQL is rejected, only Up migrations are generated, regenerated
  SQL passes parity and mutually exclusive modes are rejected without database
  access.
- Default schema validator plus the real local foundation SQL/API test: **PASS**,
  control package **2.611 seconds**.
- Final MCP review rerun before these documentation-only/runtime-neutral changes:
  `make mcp-test` **PASS with race detection**, **48.934 seconds**. No new actionable
  findings in that pass. The preceding full backend/browser results remain below.
- `git diff --check` and pinned driver verification: **PASS**, **42/42 unchanged**.

These checks use the existing local MariaDB; no new engine/version matrix or
benchmark ran. The CI parity step is locally verified, not a claim of a completed
remote workflow run.

## Second MCP review fixes — 14 November 2026

All six findings from the follow-up review are fixed:

- Tenant targets require a valid bound Tenant ID before entering the shared management API. An empty Tenant target cannot be interpreted as platform scope and bypass `platform.manage`. Target normalization uses a private copy, preserving request-key inputs.
- Each MCP POST accepts one bounded JSON-RPC object. Legacy batches, including requests without a protocol header, are rejected before SDK dispatch; one admitted request cannot fan out into many tool executions or responses.
- App entity inventories apply authorization, search and paging before response serialization. The shared paging helper validates selectors and reports completeness; hidden entities and fields remain filtered by the App ceiling.
- Entity checks use the shared validator's diagnostics-only response. Large valid definitions can complete checks without embedding generated schema and TypeScript. Results retain candidate identity and synthetic-case diagnostics; agents can inspect generated contracts separately with the existing get tool.
- Queued database and entity-publication jobs retain the delegating user, grant and OAuth client in terminal audit events, including work rejected after revocation. Audit metadata is separate from execution authority and local to each job.
- Creating a Space that reuses an existing shared database requires resource-management authority. Actual allocation still passes the shared provisioning permission, target, mode and quota checks.

Permanent regressions cover the platform escalation through a real MCP request, legacy batch admission, oversized requests, large entity checks, authorized App inventory pages, shared database reuse versus allocation, and successful/revoked worker audit attribution. The Chrome paging test also reuses the searchable fixture helper rather than assuming its administrator is on the first user page.

Validation on the existing local MariaDB and pinned driver:

- Full backend integration/race/security suite: **PASS**, control package **108.213 seconds**, including live provisioning/publication and all new regressions. Only the explicitly opt-in fixture-preparation helper skips locally.
- Complete installed-Chrome suite after the fixture correction: **25/25 PASS (1.4 minutes)**.
- `go test ./...`, `go vet ./...`, TypeScript/Vite production build and `git diff --check`: **PASS**.
- Driver verifier: **42/42 pinned files unchanged**. Registry schema remains **8**.
- Backend and embedded console rebuilt and restarted; `/readyz` returns `ready`. Existing local data and keys are preserved.

Earlier attempts are retained in ignored `.local/mcp-review2/`: a host-interrupted MCP run was followed by a successful focused rerun of both affected cases (**9.331 seconds**). The first complete browser run passed 24/25 and exposed the fixture's first-page assumption; the corrected full browser rerun passed all 25. The backend pass is in `fixes-integration.log` and the final browser pass in `fixes-browser-rerun.log`.

## MCP review fixes — 14 November 2026

All seven findings from the MCP implementation review are fixed:

- Managed allocation and retry share the same grant target/mode/quota checks. Claiming an existing resource consumes a slot; retrying the same resource does not consume another. Requeued work records the current agent/user and stops after revocation. Explicit human retries clear the previous agent binding.
- Access-token validation loads its exact grant revision and permissions in one database statement. Subsequent checks reject stale snapshots; an old access token cannot acquire an expanded grant without refresh/reconnection.
- Anonymous registration/metadata setup, token endpoints and MCP token validation have bounded admission budgets. Bootstrap and validation rate maps are separate from ordinary authenticated request limits. Bearer admission runs before database lookups, and anonymous bootstrap cannot occupy authenticated MCP execution slots.
- The in-process response adapter retains writer and JSON decoding errors. Oversized reads report `response_limit`; oversized responses after a committed mutation return the compact resource receipt and explicit response error. Replaying the same request key remains safe. Entity inspection supports separate draft/published definitions, individual generated sections and explicitly marked single-field contracts. App inspection continues to omit hidden fields and policy expressions; App inventory uses concise metadata.
- The existing list tool supports typed user/team recipient discovery through the shared authorized endpoint. Space/App context includes both recipient inventories with selector-preserving continuation guidance.
- Unapproved registrations have a separate bounded quota and expire after ten minutes. Invalid authorization attempts cannot renew their lifetime. Approved clients do not consume anonymous quota; cleanup preserves live grants and in-progress consent/code exchanges. An index supports pending-client checks (registry schema 8).
- Clipboard and revoke failures leave the loaded connection list and its actions usable. Failed loads are tracked separately from action errors.

Validation on the existing local MariaDB and pinned driver:

- Full backend `make integration` race/security run: **PASS**, control package **85.380 seconds**, including the new regressions and real provisioning/publication checks. Only the explicitly opt-in fixture-preparation helper skips locally.
- Complete installed-Chrome suite: **25/25 PASS (1.3 minutes)**, including denied clipboard access and a rejected stale revoke followed by refresh/recovery. Desktop connection and mobile edit screenshots inspected.
- `go test ./...`, `go vet ./...`, TypeScript/Vite production build and `git diff --check`: **PASS**.
- Driver verifier: **42/42 pinned files unchanged**. No database-version matrix or benchmark run.
- Backend and embedded console rebuilt; `/readyz` returns `ready` at the existing local service. Existing local data and keys are preserved.

## Single-source password policy — current, 12 November 2026

DRY is recorded as the main foundational engineering principle in `AGENTS.md`, README and the foundation contract. Password limits are defined once in `internal/control/passwords.go`. Validation, error text and public `GET /v1/config.password_policy` derive from that source. The running server publishes 8–128 characters with `Cache-Control: no-store`; the embedded console/index is also served without caching.

All initial-password fields share one field definition. All new-password inputs share one policy-driven control, including first-login replacement and ordinary account settings. There are no frontend password-limit constants or hardcoded fallback policy. Login/current-password fields remain independent of new-password requirements. Password generation respects the server bounds. Unicode code-point validation matches Go rather than native HTML UTF-16 limits. Public settings refresh after sign-in and on focus/page restoration, updating open forms without clearing drafts. Missing initial settings show an explicit retry instead of guessing policy.

New browser regressions supply a different policy (9–12 characters), verify all creation/change forms and generation, update an open form to 8–64 without losing its draft, verify missing-policy retry and complete real eight-character local onboarding after a stricter policy was loaded on the login page. The latter captures `artifacts/password-policy-onboarding.png`. Backend tests verify the unauthenticated public contract, no-store header and Unicode boundaries against the authoritative validator.

Full backend race suite with existing local integration/security configs: **PASS**, control package **47.932 seconds**; only the opt-in CI fixture-preparation helper skips locally. Complete installed-Chrome suite: **19/19 PASS in 1.1 minutes**, including all three new password-policy regressions and real eight-character account/password lifecycle checks. `go vet ./...`, TypeScript/Vite production build, `git diff --check` and driver verification (**42/42 unchanged**) pass. API and embedded console were rebuilt and restarted while preserving local data and credentials. Detailed logs are in ignored `.local/password-policy-dry/`.

## Earlier password length adjustment — 12 November 2026

Local account creation/bootstrap and password changes now accept **8–128 characters**. The API counts Unicode code points. Shared browser form settings show the requirement before submission and apply it to initial account passwords and replacement passwords; external database credentials keep their separate behavior. Existing passwords and hashing/session safeguards are preserved.

Focused verification: password-boundary cases (including Unicode), local bootstrap/login guards and the Tenant/account/password lifecycle all pass with `-race` on the existing local MariaDB (**25.989 seconds**). The integration fixtures use exactly eight-character initial and replacement passwords. `go vet ./...`, the TypeScript/Vite production build and `git diff --check` pass. Both targeted Chrome scenarios pass (**2/2 in 9.3 seconds**), covering Tenant/new-administrator creation, global account creation, login and initial-password replacement with eight-character passwords. The updated API and embedded console are running.

## Console review corrections — 12 November 2026

All five findings from the UI-simplification code review are fixed in the running console:

- Existing platform/Tenant/team roles can be revoked while the account, membership or team is disabled. New assignments still require active recipients and last-administrator guards remain enforced. Chrome checks revoke platform authority from a disabled account, Tenant authority from a disabled membership and team roles across disabled team membership/team states without restoring access.
- Platform **Users → Add to Tenant** selects an existing local or SSO identity and creates ordinary membership through the existing API. No automatic administrator grant, credential creation or email-based OIDC linking occurs. Tenant administrators still cannot read the global identity directory.
- Space/App Access collections use the parent refresh counter for manual/background updates. Open role editors keep their drafts and captured revisions; a concurrent update remains a 412 instead of being overwritten.
- Tenant administrator and user-recipient searches include email; team-recipient queries use their own columns. Database tests check the user/team query variants and scoped authority restrictions; Chrome selects a role recipient by email.
- App compute preferences are displayed and edited under Advanced using the creation form's shared field definition. Chrome verifies retaining an existing dedicated preference after plan support is removed, changing to shared and preventing a new dedicated selection.

Validation on the existing local MariaDB and pinned driver:

- Full `go test -race -count=1 -v ./...` with integration/security configs: **PASS**, control package **63.007 seconds**. Only the opt-in CI fixture-preparation helper skips locally.
- `go vet ./...`, TypeScript/Vite production build and `git diff --check`: **PASS**.
- Complete installed-Chrome suite: **16/16 PASS in 1.1 minutes**, including all three new review-regression scenarios. The focused regressions also passed independently (**3/3 in 19.0 seconds**). The ordinary-membership form was visually inspected in `artifacts/review-add-to-tenant.png`.
- Driver verifier: **42/42 pinned files unchanged**. Registry schema and database bindings are preserved. The rebuilt API/embedded console is running and ready at `http://127.0.0.1:8787/console/`.

## Console simplification — 12 November 2026

The console now uses task-based navigation and shared shell/forms/collections/pickers. Platform navigation is Tenants, Users, Activity and Settings; Tenant navigation starts at Apps. Space/App detail screens open on their actual collections. Members and team roles appear together; database servers/plans and technical form overrides are secondary settings. Browser URLs preserve direct links, reloads and Back/Forward. The account menu groups appearance, password and sign-out actions.

Tenant administrator management is one atomic platform operation: **Tenants → Open → Administrators → Add administrator**, using either an existing identity or new email/password credentials. Tests verify account/membership/grant rollback, idempotence, membership recovery, credential preservation, secret omission, privilege separation and scoped recipient lookup. Name-only Space creation uses eligible managed targets, shared-resource reuse and effective plan defaults; an unconfigured target fails cleanly without partial allocation. Registry schema is unchanged.

Validation uses the existing local MariaDB and pinned driver:

- Full `go test -race -count=1 -v ./...` with the local integration/security configs: **PASS**, control package **48.272 seconds**. Only the opt-in CI fixture-preparation helper skips locally.
- `go vet ./...`, TypeScript/Vite production build and `git diff --check`: **PASS**.
- Driver verifier: **42/42 pinned files unchanged**. No driver changes, alternate server-version testing or benchmarks.
- Complete installed-Chrome suite against the final embedded console: **13/13 PASS in 46.7 seconds**. It covers new/existing administrator creation, required initial-password rotation, name-only Space/App/micro-app creation, shared database provisioning and existing registration with an unchanged business-table marker, inline Tenant/team role management, stale revisions, delayed old-Tenant requests, direct URLs/reloads/Back/Forward, server search/paging, picker Load more, Vite authentication/writes, App-only visibility, session recovery, themes, mobile layout/focus and disabled browser storage.
- Final desktop, administrator, team, dark-form and mobile screenshots were inspected in `artifacts/simple-*.png`. Chrome verification caught a hidden desktop sidebar control; the final build corrects it. Selector failures during the test rewrite were corrected to target the custom picker listbox rather than hidden native select options. The onboarding/mobile scenario also passed after adding assertions that wait for the responsive sidebar transition to settle (**1/1 in 6.3 seconds**); the final mobile capture has a hidden sidebar and full-width content.
- Rebuilt API and embedded console restarted successfully; final readiness: **ready** at `http://127.0.0.1:8787/console/`. Existing local data, keys and database bindings are preserved.

## Tenant workspace review fixes — current, 12 November 2026

All four findings from the Tenant-workspace review are fixed in the running API and console:

- Global email/password account creation requires platform administration. Tenant membership creation rejects attempts by Tenant administrators to claim a new email or replace an existing password. Platform **Users → Create account** provides the authorized creation flow without granting automatic roles/memberships; Tenant **People** adds existing accounts by email or user ID. Authority is checked again inside the mutation transaction. Verified self-service invitations remain future work.
- Optional `trusted_proxies` identifies exact ingress IPs/CIDRs. Login resolves `X-Forwarded-For` from the direct peer towards the first untrusted hop. Direct header spoofing, injected chain prefixes, malformed/oversized chains and IPv4/IPv6 normalization are covered. Distinct proxy clients have independent budgets, while attacks against one email retain a shared limit.
- **Home → View all Apps** opens a scoped, paginated/searchable directory. A real Chrome scenario creates 101 Apps for an App-only manager and a private sibling App: the manager has no parent-Space access, reaches the oldest App through paging/search, opens its micro-app controls, and cannot discover the sibling.
- Session expiry/revocation clears the workspace and returns to login; signing out an expired session also succeeds locally. Wrong current-password credentials stay on the form. A real old-session request held across password rotation returns 401 after revocation but cannot close the newly authenticated session.

Final checks on the existing private MariaDB 12.3.3 and pinned driver:

- Full `go test -race -count=1 -v ./...` with integration/security configs: **PASS**, control package **41.186 seconds**. Only the opt-in CI fixture-preparation helper skips locally. The first run exposed a timing assumption in the new rate-budget test under the race detector; its final burst assertions use the fast invalid-length path after rate charging, avoiding password-hash timing dependence.
- Complete installed-Chrome suite: **13/13 PASS in 55.0 seconds**. The expanded account/password-rotation scenario subsequently passed again (**1/1 in 4.1 seconds**).
- `go vet ./...`, TypeScript/Vite production build and `git diff --check`: **PASS**.
- Driver verifier: **42/42 pinned files unchanged**. No driver changes, database-version matrix or benchmark run.
- Registry schema is unchanged by these four fixes. Existing data, keys and database bindings are preserved.

## Earlier Tenant workspace and local accounts implementation, 12 November 2026

The running local console has distinct Platform and Tenant workspaces. The implemented contract and review steps are in [FOUNDATION.md](FOUNDATION.md) and [README.md](README.md).

Final validation uses the existing private MariaDB 12.3.3 and the pinned Go driver; no database-version matrix or performance benchmark was run:

- Full `go test -race -count=1 -v ./...` with integration/security configs: **PASS**, control package **37.493 seconds**. This includes existing provisioning/authorization/security checks plus local bootstrap without OIDC, email/password login, initial-password restrictions, session rotation/revocation, existing-account reuse without credential replacement, rollback, secret omission, origin/throttle/hash-concurrency guards and strict platform/Tenant administrative separation. Only the explicitly enabled CI fixture-preparation helper skips locally.
- `go vet ./...`, `git diff --check` and TypeScript/Vite production build: **PASS**.
- Complete installed-Chrome suite: **10/10 PASS in 42.1 seconds**. It provisions a local administrator through the UI, changes the initial password, builds Space → App → micro-app, verifies real managed provisioning and existing registration without business-table modification, exercises teams/roles, stale editor protection, server paging/search, Tenant switching, Vite login, themes, mobile navigation and disabled browser storage.
- The Tenant-switch regression deliberately delays a successful old-Tenant form request, switches Tenants, then verifies that its completed response cannot open an old-Tenant dialog in the new workspace.
- Canonical 18-table DDL parity and the executable foundation SQL/API validator: **PASS** on a disposable registry on the same server.
- Driver snapshot: **42/42 pinned files unchanged**.
- Rebuilt API/embedded console running; readiness: **ready**.

Chrome verification caught and corrected an accessible password-field label and Space/App metadata refreshes remounting access editors. Final screenshot review also caught long Tenant names overflowing the sidebar selector; the selector is now constrained to the sidebar. Both affected registration/access and server-paging Chrome scenarios passed again (**2/2 in 19.3 seconds**) with selector-boundary assertions after the layout correction. Final desktop, dark-theme, Space/App access and mobile screenshots were inspected in `artifacts/tenant-*.png`; mobile tables scroll inside their containers.

Existing local data, encryption keys and database bindings are preserved. Local credentials add one global table and a session authentication-method column. Email recovery delivery, MFA, business entities/ACL execution, integrations and App execution remain outside this stage; the UI exposes working functionality only.

## Review fixes — 12 November 2026

Both review findings are fixed in the running local API and embedded console:

- Tenant request budgets are charged only after active membership or platform authority is established. Per-user throttling remains first. The regression covers a 120-request outsider burst, nonexistent/arbitrary Tenant IDs, disabled membership, legitimate member traffic and platform access without membership.
- Team editors submit their captured revision even when background queries refresh current metadata. Editing is disabled until the selected team is loaded. A real Chrome/API regression opens a draft, concurrently renames and disables the team, triggers a background refresh, and verifies that the stale save returns 412 without undoing the disable. Reopening and saving against the new revision succeeds.

The new tests failed against the original implementation before the fixes. Final validation uses the same existing private MariaDB instance and pinned driver:

- Full `go test -race -count=1 -v ./...` with local integration/security configs: **PASS**, control package **14.810 seconds**; only the explicit CI fixture-preparation helper skips locally.
- `go vet ./...` and `git diff --check`: **PASS**.
- TypeScript/Vite production build: **PASS**.
- Complete installed-Chrome suite: **10/10 PASS in 36.3 seconds**.
- Rebuilt API and embedded console restarted successfully; local readiness reports **ready**.

## Earlier identity roles and scopes foundation

12 November 2026, macOS/arm64, using the existing private MariaDB 12.3.3, the pinned `aantich/go-mariadb` driver, Go 1.27.4, NVM Node 24.20.0 and installed Chrome. The API and console are running against `lastapp_foundation`; earlier local databases remain intact.

Final results:

- `go test -race -count=1 -v ./...` with both local integration/security configs: **PASS**. The control package completed in **14.491 seconds**, including the new foundation, live provisioning and all 15 security regression subtests. The CI-only fixture preparation helper is not invoked in this local run.
- `go vet ./...`: **PASS**.
- TypeScript/Vite production build: **PASS**.
- Complete Chrome suite: **9/9 PASS in 33.7 seconds**. This covers the updated scoped-role UI, real provisioning/registration, fixed catalogs, team membership disable/enable, custom Space/App roles, team recipients, pagination/search, Vite login and all theme/accessibility scenarios.
- `python3 scripts/verify_driver.py`: **PASS**, 42 pinned files unchanged.
- `python3 design/validate_schema.py`: **PASS**. Canonical DDL matches executable bootstrap SQL, then the current Go foundation SQL/API checks pass on a disposable registry on the same local server.
- Local readiness: **ready**. The Northstar demo’s CRM database is **ready**, with its two registered Apps. Space/App access screenshots were inspected for layout and shared styling.

Foundation assertions cover true team scopes; fixed administrative catalogs; business/control separation; manager assignment authority; cross-Tenant/Space/App rejection; team-recipient inheritance; user/membership/team/role disable and re-enable; removal/rejoin semantics; stale membership revisions; paged caller roles; SQL domain/null-scope constraints; and concurrent last-Tenant-admin removal.

Browser verification caught a create-team form hidden before a scope was selected; that flow was corrected and passes. Running the final checks during development startup exposed a brief audit-write privilege grant in the startup script. Bootstrap now grants schema-level SELECT/INSERT only and individual table UPDATE/DELETE permissions, excluding audit history. The final Go run after corrected startup confirms that runtime credentials cannot modify/delete audit rows.

The implemented API/scope is [FOUNDATION.md](FOUNDATION.md). Business-entity ACL evaluation, authorized SQL compilation, App credentials and the App SDK remain deferred. These results do not claim production load or deployment validation.

## Earlier provisioning stage verification

12 November 2026, local macOS/arm64. Actual execution against the private MariaDB instance; no mock database.

## Environment

- Go 1.27.4; module baseline Go 1.25.
- Node 24.20.0 from the working NVM installation, React 19, Vite 7, Tailwind 4.
- MariaDB 12.3.3, through the dedicated `aantich/go-mariadb` driver snapshot.
- Installed Google Chrome, driven by Playwright.

Final result: **all Go tests passed with the race detector; `go vet ./...` passed; all 3 Chrome scenarios passed** (final browser run: 19.5 seconds). Frontend production build passed.

## Executed checks

`go test ./...` and `go vet ./...` cover compilation and static checks. `go test -race -v -count=1 ./internal/control` with `LASTAPP_TEST_CONFIG` enables the real MariaDB integration scenario in addition to unit tests.

The integration scenario checks:

- Encrypted credential round trips, tampering and wrong owner/Tenant rejection; malformed request validation.
- OIDC-backed identities cannot create Tenants without platform grants.
- Last direct Tenant administrator protection; last platform administrator protection is also exercised through the browser API.
- Concurrent shared Space creation produces one shared physical database; duplicate Space creation does not allocate an orphan resource.
- Managed provisioning reaches ready; verification retry preserves credentials and database ownership.
- Dedicated database bindings cannot be reused by a second Space.
- App-scoped collection reads and writes, scoped pagination, sibling-write denials and stale revisions.
- Group membership, inherited grants and immediate revocation when disabling a group.
- Tenant custom roles cannot include platform permissions.
- Existing database registration preserves a business-table marker and validates schema CRUD grants.
- Bad registered credentials fail cleanly and replacement credentials recover the same resource.
- Unknown database ownership and pre-existing account collisions are refused; unrelated account passwords remain valid.
- Killing the worker leader's pinned registry connection during target work recovers to ready safely.
- Plan layout restrictions, pending-resource quotas, cross-Tenant references and suspended Tenant allocation rules.
- API/audit responses omit connection secrets. The registry runtime account is denied UPDATE/DELETE on audit history at the database level.

`npm run test:e2e` in `web/admin` uses real OIDC redirects and Chrome to check:

1. Tenant → shared Space/ready database → App → micro-app, using console forms.
2. Missing CSRF rejection, stale revisions, last-superadmin protection and unauthorized outside-user access.
3. Existing database registration with unchanged business data; membership, custom role, group and grant management; platform access, plans, targets and activity screens; desktop and mobile screenshots.

Screenshots are saved locally in `artifacts/`: `admin-overview.png`, `admin-microapps.png`, `admin-databases.png`, `admin-access.png`, `admin-mobile.png`. Playwright retains traces and screenshots for failed runs in ignored local folders. Browser scenarios assert no uncaught page errors.

The earlier standalone DDL validation report, [design/VALIDATION.md](design/VALIDATION.md), records 65 structural checks. It is separate from the Go/API/browser tests.

## Boundaries

These checks verify the provisioning stage, not production load, multi-region operation, every external OIDC provider, or business-entity APIs. MariaDB 10.11 remains a compatibility-matrix task; the executed server version was 12.3.3. Automatic key rotation, large-directory console paging, production packaging/deployment and App scheduling are not claimed as implemented.

Test-owned database resources, including intentional failures, are retained locally for inspection. Tests create isolated IDs/names and do not drop existing databases or user accounts. `make stop` stops services without deleting data.

## Admin design and themes — 12 November 2026

The Paces Neo-inspired redesign passes the frontend TypeScript/Vite production
build and Go compilation through `make dev`. The final complete Chrome run
passes **7/7 scenarios in 35.0 seconds**, using the existing local API, OIDC
issuer and MariaDB. The original three provisioning/access scenarios still pass.
The four additional scenarios verify:

- Sidebar collapse/expand; actual light/dark surface changes; preset persistence
  across reloads, tables and edit forms; reset; Escape and focus restoration.
- System color-mode changes, preference synchronization between tabs and
  recovery from invalid stored values.
- Mobile workspace selection and navigation, theme controls, focus containment,
  Enter-key navigation search, and no horizontal page overflow at 390px.
- Console navigation and theme switching when browser storage throws.

The authenticated appearance scenario also checks for browser console and
uncaught page errors. The initial signed-out `/v1/me` probe intentionally returns
401 and is outside its console-error assertion. Earlier checks caught and led
to fixes for the radio hit area and Enter-key search reopening; screenshot and
color assertions now account for finite CSS transitions.

Desktop Neo, Studio dark forms and the mobile Evergreen dark appearance drawer
were visually inspected. Additional local captures are in
`artifacts/admin-neo-overview.png`, `artifacts/admin-studio-dark-form.png` and
`artifacts/admin-mobile-appearance.png`. Design observations, raw local reference
locations and theme extension instructions are recorded in
[the design reference](design/references/paces-neo.md).

This design pass changes the frontend and its documentation/tests. It does not
rerun or relabel the earlier Go race/integration results above.


## Security corrections — 12 November 2026

All ten findings in the initial security review are corrected; see [the remediation and verification record](review/security-2026-11-12/FIXES.md). The existing registry is on migration 2. The full Go/API suite passes with `-race`; `go vet` passes; the TypeScript/Vite build passes; and the final Chrome run passes **9/9 scenarios in 34.2 seconds**, including server paging/search and Vite login plus authenticated writes. `govulncheck` reports no vulnerabilities after the go-jose patch, and all 42 driver snapshot hashes match.

This supersedes the earlier limitations about unpaged collection loading and the latest-150-only audit view. Security regression fixtures are disposable and clean up only their own verified resources; the original integration/browser inspection fixtures retain their existing behavior. The new CI workflow is configured but has not been run remotely. Business-data policy enforcement and generated-app execution remain outside the provisioning phase; their required boundaries are recorded in the provisioning plan.

## Business entities, ACLs and rules — 12 November 2026

The entity implementation supersedes the preceding provisioning-only boundary.
It includes the React designer, generated MariaDB storage, durable publication,
secured data APIs, App-bound authentication, TypeScript SDK and optional record
sharing. General generated-App builds/runners remain outside this stage. Usage
and supported changes are in [the entity guide](ENTITY_RUNTIME.md).

Final checks ran against the existing local MariaDB and the rebuilt Go/React
services, using the unchanged pinned driver:

| Check | Result |
| --- | --- |
| `LASTAPP_TEST_CONFIG="$PWD/.local/config.json" LASTAPP_SECURITY_CONFIG="$PWD/.local/config.json" go test -race -count=1 ./...` with local `GOCACHE` | All packages passed; control integration suite 56.094 seconds |
| `npm run test:e2e --prefix web/admin` with the configured Node | 21/21 Chrome scenarios passed in 1.2 minutes |
| `make client` | SDK TypeScript build and static example generation passed |
| `make dev` | React type/build checks, Go binaries and local startup passed; registry version 4 |
| `go vet ./...` | Passed |
| `python3 scripts/verify_driver.py` | All 42 snapshot hashes matched; driver unchanged |
| Normalized design DDL versus all four migration Up sections | Identical |
| Plan/guide JSON examples, local links and `git diff --check` | Passed |
| HTTP `/healthz`, `/console/`, `/v1/config` | All returned 200 after verification |

The real-database entity regression covers owner ACLs, inaccessible-record errors,
authorized revision conflicts, atomic bulk rollback, count/cursor pagination,
wide-projection page limits, exact decimal and null/text SQL/evaluator parity,
synthetic tests without mutations, interrupted activation recovery and rejection
of unsupported storage changes while preserving the active version. Two Apps
reuse the same entity and records; another Space in the same physical database
cannot cross their namespace or reference boundary. Sharing checks include exact
permission delegation, label versus security changes, grantor revocation and
parent-session revocation. Separate regressions check Tenant concurrency limits
and prove a stalled database open does not block a healthy cached resource.

Browser checks retain the complete provisioning, administration, roles, password,
appearance and workspace regressions. New flows author/test/publish an entity on
managed storage, preserve a dirty editor across a competing draft update, and
connect a static App on a separate host through the actual approval/PKCE/token
flow to create and read real records. Registered storage is covered by database
and integration fixtures. The designer's desktop and 390px screenshots were
visually inspected, including responsive layout and the existing shared theme.

Final local logs are `.local/entities-go-final-all.log`,
`.local/entities-browser-final-all.log` and `.local/entities-dev-final.log`.
Screenshots are `web/admin/test-results/entity-designer-desktop.png` and
`web/admin/test-results/entity-designer-mobile.png`. These generated artifacts
remain ignored. Earlier verification runs exposed and corrected ambiguous test
selectors and an expiration-test race where a pending request could return the
UI to login before Refresh was clicked; the final complete run passed.

Existing development data and pre-existing working-tree changes were preserved.
No database-version matrix, benchmark, driver modification or remote deployment
was performed. The local console remains available at
<http://127.0.0.1:8787/console/>.

## Entity designer layout correction — 12 November 2026

The Fields screen now aligns the name input and Add field button, uses a compact
list with shared edit/remove actions, and groups ownership, default fields,
sharing and lifecycle controls into a properly padded settings disclosure.
Desktop ownership controls share a row and stack on mobile. The same row
component serves Access and Rules. Entity dialogs have one padding layer,
consistent form spacing and grouped review actions. All colors continue to use
the shared appearance tokens.

`make dev` passed the TypeScript/Vite build and rebuilt the running local console.
The complete browser run passed its 20 existing non-entity scenarios; the new
entity layout check initially used an exact label selector that did not match
the select's implicit label. After switching to its accessible combobox role,
the focused entity scenario passed, including real create/test/publication,
desktop input/button and ownership alignment, 390px overflow, mobile dialog
bounds and focus restoration. The final visual run also captures Studio dark
and wide desktop layouts; its final run passed in 8.2 seconds. All four final
captures were visually inspected. Logs are `.local/entity-layout-browser.log`,
`.local/entity-layout-focused.log` and `.local/entity-layout-visual-final.log`.
The screenshot helper resets scroll before full-page captures so fixed headers
are not captured at their previous scrolled offset.

Screenshots are in `web/admin/test-results/entity-designer-{desktop,dark,mobile}.png`
and `web/admin/test-results/entity-editor-mobile.png`. This correction changes
frontend layout/components and their browser verification; backend business
logic and the pinned MariaDB driver are unchanged.

## Complete screen and modal layout review — 12 November 2026

Reviewed the current console screen families and their create/edit/disclosure
variants through real local sessions. The final capture matrix contains **136
states at desktop and mobile sizes (272 captures)**, with no document horizontal
overflow, off-screen dialogs or extra Advanced-section indentation in its final
checks. Captures/contact sheets were visually inspected, with full-size checks
of the dense forms, settings, credentials, onboarding and expression dialogs.

| Area | Reviewed surfaces |
| --- | --- |
| Authentication/account | Sign-in, first password, ordinary password change, SSO account, account before Tenant assignment, App connection approval/error |
| Platform | Tenants, Tenant overview/administrators/settings, users, activity, database servers, plans; their create/edit/account/administrator dialogs and Advanced sections |
| Tenant workspace | Apps, Spaces, Space/App tabs and settings, micro-apps, people, teams and team members, activity, Tenant settings; create/edit/member dialogs |
| Access | Space/App assignments, custom role lists and create/edit dialogs, user/team access assignment, inherited and explicit App ceilings/restrictions |
| Databases | Managed creation, existing registration, resource list, replacement runtime credentials and schema credentials |
| Entity authoring | Library, fields/settings, all eleven field types and relevant options, access subjects/operations/conditions, validation/allowed-change dialogs, all four Test operations and publication review |
| Shared surfaces | Search/paging tables, empty/loading states, appearance controls, mobile navigation and the isolated static data example |

Corrections use shared components and CSS tokens:

- One form spacing/padding/control-height contract; Advanced fields no longer
  acquire a second inset. Single-field forms and initial passwords use the
  available width. Account selection and credential pairs have coherent rows.
- Table action headings align with their actions; long actions wrap. Table text
  is more readable and existing scroll containers retain mobile containment.
- Settings have consistent heading/action groups and spacing. App data access
  is a sibling section rather than a panel nested inside another settings card;
  its developer-only local setup instructions remain in documentation.
- Password onboarding uses a centered single-column grid. The account password
  form has an appropriate section heading and no inherited oversized title or
  excess hint margins. Mobile navigation no longer animates the desktop content
  offset across the layout breakpoint.
- The isolated example aligns its entity selector/actions and uses consistent
  field/button spacing. Colors and appearance presets remain shared in the
  console; no authorization rules or backend behavior changed.

`make dev` passed the final TypeScript/Vite and Go build and left the local UI
running. The complete Chrome suite passed **23/23 scenarios in 1.3 minutes**,
including two new regressions for advanced-field alignment, credential pairing,
responsive dialog bounds, centered onboarding and account use without Tenant
assignment. Existing provisioning, permission, password-policy, theme and App
data flows passed. All 42 pinned-driver hashes matched; `git diff --check` passed.

Local evidence: `.local/ui-audit/final-console/manifest.json` (164 captures),
`.local/ui-audit/final-authoring/manifest.json` (108 captures),
`.local/ui-review-build.log`, `.local/ui-review-browser-final.log` and
`.local/ui-review-example-final.log`. The first audit iteration exposed a bad
audit endpoint and transient measurements during responsive animation; these
were investigated and corrected before the final matrix. It also exposed an
audit navigation race during password replacement; the final audit waits for
the actual authenticated screen, and the permanent regression verifies that
flow. These audit issues are not counted as application failures or passing
checks. Screenshot artifacts remain ignored; prior data and uncommitted work
were preserved. No commit, push or remote deployment was performed.

## Delegated management MCP — 14 November 2026

Implemented the management-only MCP endpoint in the existing Go service using
the official Go SDK and Streamable HTTP. Its 14 typed tools reuse the existing
management handlers, authorization, mutations and workers. Context includes
paginated inventories, full authorized entity definitions, published App
contracts, revisions, current grant limits and explicit unavailable features.
Canonical guides are shared between the describe tool and MCP resources.
Micro-app registration does not yet build or host source; business-record
access and robot identities are outside this release.

OAuth authorization-code/S256 PKCE consent delegates an existing user's current
authority within an explicit Tenant, team, Space, App or platform ceiling.
Audience-bound, hashed credentials, rotating refresh tokens, grant revision
checks and revocation apply to requests and queued work. Provisioning has
approved-target and allocation limits. Mutation receipts make unchanged retries
safe, and shared audit entries retain the human and agent identity. Agents
cannot expand grants, retrieve credentials or execute arbitrary SQL/code.
Native clients require registration; public HTTPS Client ID Metadata Documents
are also supported. The URL-only connection follow-up below adds automatic
dynamic client registration to this initial implementation.

The console now provides consent plus My account → Agent access, with editing,
expiry and revocation; Tenant administrators can inspect and revoke Tenant
connections in Settings. Permissions, presets, eligibility and limits come from
the backend contract. Desktop consent and mobile consent/edit screens were
visually inspected; browser checks cover control sizes, overflow, modal bounds,
SSO return, approval and denial.

Final validation passed:

- `make integration`: the full backend race/integration suite passed in
  63.240 seconds, followed by **25/25 browser scenarios in 1.4 minutes**.
- `go test ./...`, `go vet ./...` and `git diff --check` passed. All 42 pinned
  MariaDB-driver file hashes matched; the driver was not modified.
- The official MCP client exercised real OAuth, schemas/tool discovery,
  resource/role/membership management, managed database provisioning and entity
  publication. Security cases include sibling-scope denial, narrowed context,
  App contract filtering, stale edits, idempotency conflicts, invalid PKCE,
  refresh replay, grant changes/revocation, token audience isolation across
  endpoints sharing one registry, unsafe origins and provisioning ceilings.
- A deterministic regression verifies that publication queued before database
  provisioning waits for storage readiness. Recursive expression schemas are
  checked against actual default/calculated-field and App-gate expressions.
- The rebuilt local server reports ready and advertises the exact
  `http://127.0.0.1:8787/mcp` OAuth resource. The registry is at schema version 6.

Final log: `.local/mcp-release-validation.log`. Screenshots remain ignored in
`web/admin/test-results/mcp-{consent-desktop,consent-mobile,edit-mobile,connections-desktop}.png`.
The console remains running at <http://127.0.0.1:8787/console/>. Existing local
data was preserved. No database-version matrix, commit, push or remote deployment
was performed.

### URL-only agent connection follow-up — 14 November 2026

The user-facing flow now starts in the agent client with the MCP URL, followed
by Lastapp sign-in and consent. Static client registration, secrets, tokens and
manual OAuth scope configuration are unnecessary. Discovery supports both Client
ID Metadata Documents and an advertised RFC 7591 registration endpoint. A
registration stores bounded public metadata only; it cannot grant authority.
S256 PKCE, exact registered host/path matching, dynamic native loopback ports
(including Claude's localhost callback), explicit consent and audience checks
still apply. Registration is rate/concurrency limited and serialized at its
registry cap; cleanup retains clients with live grants.

Discovery offers the supported capabilities so users can choose App/Entity
author permissions on the first connection. Consent still initially selects
inspection only, and issued tokens contain only the selected permissions.
Explicitly requested narrower scopes remain narrow. The console shows the
return address and offers Copy MCP URL with a short connection explanation.
Policy limits and scope lists continue to derive from the backend contract.

Native-client smoke checks used the installed Codex CLI
`0.162.0-alpha.17.2` (default CIMD and explicit DCR) and Claude Code `2.1.263`
(default CIMD). Each discovered OAuth using only the server URL. Following each
real client-generated authorization URL returned HTTP 302 to Lastapp consent;
both vendors' real metadata documents and native callback formats were accepted.
These checks deliberately stopped before approval, so they did not authorize a
connection or save access tokens. Headless commands ended at their callback
input prompt. Claude used an isolated disposable profile; normal client
configuration was not changed.

Two new backend integration scenarios exercise automatic discovery/registration,
mandatory PKCE, redirect pinning, explicit approval, token exchange and the
official MCP client, plus rejection of unsafe/oversized registration metadata.
They verify that registration creates no grant, offered capabilities do not
become authority, and cleanup preserves a live connection. The browser consent
scenario now registers its own public client with no configured scope list and
checks the exact approved token scopes. Consent/account screenshots were
visually inspected. The desktop account capture now opens the account at the
desktop viewport, verifies panel/sidebar alignment and disables transient
animations while taking its image.

The full backend race/integration run passed in 68.647 seconds and all **25
browser scenarios passed in 1.4 minutes**. The final two-scenario MCP browser
confirmation, including the settled desktop capture, passed in 4.1 seconds.
`go test ./...`, `go vet ./...`, driver integrity (42 pinned files) and
`git diff --check` passed. The local console was rebuilt and remains running.

Evidence: `.local/mcp-auto-focused.log`, `.local/mcp-auto-final-validation.log`
and `.local/mcp-native-{codex,codex-dcr,claude}.log`. Schema version 7 adds the
public client metadata table. These ignored local logs contain transient OAuth
request state and should not be published. No native-client approval, commit,
push or remote deployment was performed.

## 14 November 2026 — isolated workspaces and static App releases

Implemented the agreed remote-management/local-authoring flow: owned per-agent
Space copies with empty managed databases, typed identity mapping, development
PKCE sessions, authoritative React/SDK starter files and ZIP download, bounded
immutable filesystem uploads, pinned App release manifests and durable explicit
production preparation/activation. The console exposes Space Development and
App Releases. Current contracts and deployment prerequisites are in WORKSPACES.md;
AGENTS.md and CLAUDE.md reference that guide.

Final validation:

- `LASTAPP_TEST_CONFIG=.local/config.json LASTAPP_SECURITY_CONFIG=.local/config.json go test -race -count=1 ./...` passed; control integration/security package: **143.667 seconds**.
- All **26 Chrome scenarios passed in 1.8 minutes**. The new scenario writes the
  returned starter files to a disposable local directory, compiles React using
  the console's identical pinned dependency tree, uploads through the UI,
  authenticates with real App PKCE and performs workspace CRUD. The same compiled
  files then run on the production App host with its production App ID and an
  empty production dataset. Desktop viewport and mobile captures were visually
  inspected; geometry assertions cover sidebar/content alignment and overflow.
- Integration coverage includes production-record preservation during additive
  schema promotion, cyclic-reference baseline copying, immutable release bases,
  workspace rebasing after success, cross-agent denial for the same user,
  development-only authority, grant-revision session revocation, artifact path
  confinement, upload replay and corrupt-build rejection.
- `go vet ./...`, UI TypeScript/Vite build, generated SQL parity, driver integrity
  (**42 unchanged pinned files**) and `git diff --check` passed. Registry migrations
  9–11 are applied locally; the development service was rebuilt and restarted.

Expanded validation found and corrected a missing build-list pagination contract
and a reference-column ALTER operation that combined incompatible foreign-key
and online-column steps. The existing entity browser scenario also read a draft
revision before its asynchronous save had completed; it now waits for the actual
successful save response. Focused reruns and the final complete suites passed.

Evidence stays ignored locally: `.local/workspaces-final-go.log`,
`.local/workspaces-final-browser.log`, `.local/workspace-focused.log` and
`web/admin/test-results/`. These files can contain transient test authorization
state and must not be published. Native-client remote SaaS setup, wildcard TLS,
shared-volume replica operation, production load and object storage were not
exercised by these local checks. No commit, push or remote deployment was made.
