# Lastapp verification

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
