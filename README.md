# Lastapp — provisioning and authorization foundation

A Go API and React/Tailwind admin console with real MariaDB provisioning, shared identities/teams, fixed administrative roles and custom Space/App business roles. See [FOUNDATION.md](FOUNDATION.md) for the implemented role model and APIs. Business-object ACLs and the entity runtime are the next stage.

**Open locally:** http://127.0.0.1:8787/console/

Choose **Platform operator**, password **`lastapp-local`**. The local issuer also offers **Tenant administrator**, **Joe (Marketing Reviewer)** and **Outside user**, with the same development password. Signing in does not grant platform administration. The **Northstar demo** contains a CRM Space, Marketing/Engineering teams and two Apps; Joe has a role within Marketing and receives a Space business role through that team.

## Run

Requirements: Go 1.25+, working Node/npm, and MariaDB server/client/install tools. Verified on this Mac with Go 1.27.4, Node 24.20.0 and MariaDB 12.3.3.

```sh
git clone https://github.com/aantich/lastapp.git
cd lastapp
make dev
```

`make dev` builds the frontend and embeds it in the Go binary, applies migrations, seeds development identities/plans, and starts three detached local processes:

| Service | Address |
| --- | --- |
| Admin UI and API | `127.0.0.1:8787` |
| Development OIDC issuer | `127.0.0.1:8788` |
| Private MariaDB | `127.0.0.1:33317` |

Data, keys, credentials, process IDs and logs are in ignored `.local/`. The script uses its own MariaDB data directory, refuses unrelated processes on its ports, and rebuilds/restarts the app and issuer while preserving data/keys. This undeployed foundation initializes `lastapp_foundation`; the earlier `lastapp_control` registry remains intact, with its previous configuration saved privately. There is no legacy-grant migration. To start already-built binaries, use `python3 scripts/dev.py --skip-build`.

When the Node on PATH fails, the script falls back to `$HOME/.nvm/versions/node/v24.20.0/bin`; it does not change global Node or Homebrew. `make` also accepts `NODE_BIN=/path/to/node/bin`.

```sh
make stop                  # stop these local services; preserve data
make test                  # Go unit tests and compilation; DB tests skip by default
make integration           # actual MariaDB + Chrome tests; run make dev first
```

Do not delete `.local/config.json` while retaining its database: its encryption key is needed to read stored connection credentials. The development identity provider is explicitly loopback-only; production uses an external trusted OIDC issuer.

## Review the UI

1. **Tenants → Create Tenant:** select a plan and an existing signed-in identity as the initial Tenant administrator.
2. Open that Tenant and **Create Space**, explicitly selecting its initial manager. Shared layout reuses the Tenant's default database; dedicated layout creates an exclusive database. Status updates while provisioning runs.
3. **Apps → Create App**, select its initial manager, then **Micro-apps → Register micro-app**. These are registry records in this phase, not deployed business runtimes.
4. **Databases → Add database:** request managed creation or register an existing database on a configured target with scoped credentials. Existing registration validates connectivity/version/CRUD grants and does not write to business tables. Failed resources can be retried; registered credentials can be replaced through the API.
5. **Members / Access control:** manage Tenant membership; use Tenant, Teams, Spaces and Apps tabs for scoped role assignments. Tenant/team roles are fixed. Space/App business roles are custom and cannot grant administrative actions. Team membership is separate from a user's role within that team; protected manager roles accept users only. The last active Tenant administrator is protected.
6. Select **Platform overview → Access control** to manage platform superadmin grants. **Plans**, **Users**, **Database targets** and **Activity** provide system management. The last active superadmin is protected.

The `Starter` plan allows one shared managed database, three Spaces and ten Apps. `Development` permits both layouts/methods, dedicated-compute preferences, 100 Spaces, 500 Apps and 100 database resources. Plans apply to future allocations; they do not migrate existing resources.

## Implementation

- `internal/control`: explicit SQL/HTTP handlers, registry role resolution, encrypted credentials, OIDC sessions and provisioning. `internal/authz` defines the shared typed scopes and fixed control catalog. No ORM or policy service.
- `cmd/lastapp`: `serve`, `migrate`, `bootstrap`, `seed-development`.
- `cmd/devoidc`: development-only signed OIDC issuer supporting discovery, JWKS, code exchange, PKCE and nonce.
- `web/admin`: React UI, Tailwind and shared form/table components, served from the Go binary at `/console/`.
- `design/control_plane.sql`: 17-table foundation schema. `internal/control/migrations/00001_control.sql` initializes a fresh registry; security constraints are consolidated there.
- `third_party/go-mariadb`: source snapshot of **[aantich/go-mariadb](https://github.com/aantich/go-mariadb)** from sibling `../go-mariadb`, including its compression implementation and license notices. `SNAPSHOT.json` records hashes. All connectivity uses its `mariadb` package and `database/sql` connector. No MySQL fallback driver is installed. The snapshot avoids silently changing Lastapp when another chat edits the driver; intentionally refresh/pin it when adopting a reviewed driver change.

The hierarchy is **Tenant → Space → App → micro-app installation**. A Space has one fixed database binding. Spaces can share only within the same Tenant, and dedicated bindings are exclusive. All identities, memberships, groups, permission definitions, roles and grants live in the central registry.

Session tokens are hashed at rest; writes require CSRF and exact Origin checks. Authorization reads current grants on each request. List queries apply scope predicates before selecting rows. Cross-Tenant references have composite foreign keys as a second guard. Revisions prevent stale overwrites. Client-generated UUIDs make duplicate creation detectable; retries must reuse the original UUID.

Credentials use AES-GCM with random nonces, key IDs and authenticated owner/Tenant context. API responses and audit events omit secrets. Migration credentials, registry runtime credentials and target provisioning credentials are separate; the development registry account can append/read audit events but cannot update/delete them.

A pinned MariaDB advisory lock elects one provisioning worker. It reconciles durable resource rows; a separate target resource lock prevents simultaneous external DDL. Generated names, stored passwords and database ownership comments make retries stable. Unknown databases/accounts are refused. Retiring registry entries never drops physical data. Schema rollback is deliberately not destructive: restore a backup for rollback.

## Production configuration

Copy `config.example.json` to a protected file and replace its placeholders. Use a separately privileged migration DSN with `lastapp migrate -config /path/to/migration.json`, then run `lastapp serve -config /path/to/runtime.json`. Bootstrap an exact issuer/subject identity using `lastapp bootstrap -config ... -issuer ... -subject ...`; email is never a privilege source.

Production requires HTTPS public origin, trusted HTTPS OIDC, identity-verified database TLS and an operator-defined runtime account host/network policy. Terminate HTTPS at the trusted ingress. `allowed_networks` explicitly permits private database address ranges; public addresses are otherwise permitted. DNS results are checked again at connection time. Register each physical server under one canonical endpoint: equivalent DNS aliases are not automatically deduplicated.

The deployment encryption key is a base64 32-byte key. This first version uses one configured key generation (`encryption_key_id`, default `primary-v1`); automatic key rotation/re-encryption is not implemented. Preserve keys with registry backups.

## Scope of this phase

Implemented: centralized identity/inventory, scoped role management, fixed Tenant/team catalogs, custom Space/App business roles and assignments, plan quotas, lifecycle management, real database provisioning/registration, retries/recovery, auditing and the matching admin UI.

Later: business entity definitions/API compilation, entity/record/field ACLs, micro-app build/execution, App container scheduling, external connectors, billing and production operations. `compute_mode` records a preference; it does not start containers. Artifact references are metadata. Collection APIs always return bounded pages (`?page=1&limit=100&q=search`, maximum limit 200) with `pagination: {page, limit, has_more}`. Scope predicates apply before search and pagination. The console pages and searches collections and selection lists on the server; audit history and group members are paged too.

See [FOUNDATION.md](FOUNDATION.md), [CONCEPT.md](CONCEPT.md), and [VERIFICATION.md](VERIFICATION.md). The provisioning plan describes the earlier implementation; its Tenant custom-role model is superseded by the foundation.

## Admin themes

The console defaults to a Paces Neo-inspired design: compact Inter typography,
a pale grouped sidebar, a white workspace, blue accents and dashed card borders.
Use the **palette icon** in the topbar to choose **Neo**, **Studio** or
**Evergreen**, with **Light**, **Dark** or **System** color mode. Preferences are
saved on this browser and apply across console screens and forms. Mobile has
an offcanvas menu that includes the Tenant workspace selector.

Themes use shared semantic CSS variables exposed through Tailwind 4 utilities;
new components can use `bg-surface`, `text-ink`, `text-primary`, `border-line`
and `rounded-panel` without choosing a fixed palette. See the
[reference and theme extension guide](design/references/paces-neo.md).


Security fixes and operating boundaries are recorded in [the remediation report](review/security-2026-11-12/FIXES.md). Run `make integration` for the API/security/browser suites. The local Vite UI on port 5173 is an explicit development-only origin; login returns there, and writes use the proxy. Production accepts only its configured admin origin and requires verified TLS for TCP registry connections.
