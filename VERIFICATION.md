# Lastapp verification

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

## Current identity roles and scopes foundation

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
