# Lastapp — shared business backend and agent tooling

A Go API and React/Tailwind console with separate Platform and Tenant workspaces, email/password accounts and optional OIDC, real MariaDB provisioning, shared identities/teams and scoped roles. See [FOUNDATION.md](FOUNDATION.md) for the implemented role model and APIs. The entity designer, generated storage, business-object ACLs/rules, App sessions, TypeScript data SDK and delegated management MCP are implemented.

**Foundational principle: DRY.** Rules and policies have one authoritative source, shared by all consumers. Keep implementations simple and reuse existing contracts and components. See [AGENTS.md](AGENTS.md) for the persistent engineering rules.

**Business entities:** [Usage and implementation guide](ENTITY_RUNTIME.md) · [Design and delivery plan](ENTITY_DESIGN_IMPLEMENTATION_PLAN.md). In a Tenant workspace, open **Space → Entities**. App origins and ceilings are approved under **App → Settings → App data access**.

**Agent MCP:** connect your agent to `http://127.0.0.1:8787/mcp` (or your deployment's HTTPS URL). The client discovers OAuth and opens Lastapp for sign-in and explicit approval; there are no API keys or manual client registrations to create. The URL is also under **My account → Agent access**. Agents manage configuration using the same guarded backend operations as the console. See [MCP.md](MCP.md) for connection, tools, context, delegation, retries and security boundaries.

**For new contributors and agents:** start with [AGENTS.md](AGENTS.md) and [DEVELOPMENT.md](DEVELOPMENT.md). [CLAUDE.md](CLAUDE.md) imports the same instructions. These current guides distinguish implemented behavior from historical plans and deferred product ideas.

**Open locally:** http://127.0.0.1:8787/console/

Use **Continue with SSO** to open development SSO, then choose **Platform operator**, password **`lastapp-local`**. The issuer also offers **Tenant administrator**, **Joe (Marketing Reviewer)** and **Outside user**, with the same development password. Signing in does not grant platform administration. The **Northstar demo** contains a CRM Space, Marketing/Engineering teams and two Apps. The normal login form accepts the email/password accounts created by platform administrators through Tenant provisioning or **Users → Create account**.

## Run

Requirements: Go 1.25+, working Node/npm, and either MariaDB server/client/install tools or Docker. Verified on this Mac with Go 1.27.4, Node 24.20.0 and MariaDB 12.3.3.

Without a local `mariadbd` (or with `LASTAPP_MARIADB=docker`), `make dev` runs MariaDB from [docker-compose.yaml](docker-compose.yaml) instead: the same `mariadb:12.3` image as CI, published only on `127.0.0.1:33317`, data in the `lastapp_mariadb-data` volume, root password generated into `.local/mariadb.env`. Database accounts then use host `%`, because published-port connections reach the container from the Docker gateway rather than `127.0.0.1`. `make stop` stops the container and keeps the volume.

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

Do not delete `.local/config.json` while retaining its database: its encryption key is needed to read stored connection credentials. The development identity provider is explicitly loopback-only. Production can use local accounts, trusted OIDC, or both.

## Review the UI

1. As **Platform operator**, open **Tenants → Create Tenant**. Choose a plan, enter the administrator's name/email/initial password (or generate one), or select an existing account. An existing account's password is never replaced.
2. Sign out, then sign in with that email/password. Replace the initial password to unlock the workspace. Alternatively, use the development **Tenant administrator** SSO account to inspect the Northstar demo.
3. Tenant users land on **Apps**, the complete searchable, paginated directory. The sidebar is **Apps, Spaces, People, Activity, Settings**, filtered by authority. App-only managers need no parent-Space visibility. A single searchable Tenant switcher appears for multiple memberships.
4. **Spaces → Create Space:** enter a name. The current user becomes manager; managed creation uses the effective Tenant/plan database layout and an eligible server, reusing the Tenant's shared database where applicable. **Advanced** exposes slug, manager, layout/server and existing-resource choices. A Space opens on **Apps**, with **Entities**, **Access** and **Settings**; database details are in Settings.
5. An App opens on **Micro-apps**, with **Access** and **Settings**. Create Apps and micro-app registrations by name; technical metadata and compute preferences are under Advanced. The compute preference remains editable under App **Settings → Edit App → Advanced**. These preferences and registrations do not execute Apps.
6. **People → Members:** Tenant administrator checkboxes and Tenant access controls are directly on member rows. Add existing local accounts by email. **Teams** opens a team's members with Administrator/Reviewer checkboxes on the same rows. Space/App **Access → Manage roles** is the secondary custom-role editor. The last active Tenant administrator remains protected. Existing roles can be revoked while the account or access is disabled; new assignments require active recipients.
7. Tenant **Settings** contains general information, plan limits and databases. Existing database registration preserves business tables; connection checks/retries, credential replacement and retirement use the provisioning API. The **account menu** contains **My account**, **Appearance** and **Sign out**.
8. Platform navigation is **Tenants, Users, Activity, Settings**. Open a Tenant for **Overview, Administrators, Settings**. **Administrators → Add administrator** creates or selects an account, activates its membership and grants Tenant administration atomically. Existing passwords are never replaced. **Users** contains platform administrator checkboxes and global account enable/disable; **Settings** contains database servers and plans. **Users → Add to Tenant** adds an existing local or SSO identity as an ordinary member, with no automatic administrator grant. Platform accounts cannot author Tenant Spaces/Apps or hold Tenant/team roles.

Console URLs identify the current Tenant, Space/App and tab. Reloads, direct links and browser Back/Forward preserve that location. Searchable pickers load results and further pages inside one control; list pagination disappears when unnecessary.

Expired or revoked sessions return to login automatically, including when signing out an already-expired session. Incorrect login/current-password responses remain ordinary form errors. Starting a new password session or rotating credentials invalidates older request generations so a delayed old-session response cannot close the new session.

The `Starter` plan allows one shared managed database, three Spaces and ten Apps. `Development` permits both layouts/methods, dedicated-compute preferences, 100 Spaces, 500 Apps and 100 database resources. Plans apply to future allocations; they do not migrate existing resources.

## Implementation

- `internal/control`: explicit SQL/HTTP handlers, registry role resolution, encrypted database credentials, local/OIDC sessions and provisioning. `internal/authz` defines the shared typed scopes and fixed control catalog. No ORM or policy service.
- `cmd/lastapp`: `serve`, `migrate`, `bootstrap`, `seed-development`.
- `cmd/devoidc`: development-only signed OIDC issuer supporting discovery, JWKS, code exchange, PKCE and nonce.
- `web/admin`: React UI, Tailwind and shared form/table components, served from the Go binary at `/console/`.
- `design/control_plane.sql`: canonical registry schema, kept equal to the executable migrations. It includes local accounts, entities/publications, App-bound sessions and agent OAuth/delegation/receipt tables. Regenerate from migrations with `python3 design/validate_schema.py --write`; check parity with `--check`.
- `third_party/go-mariadb`: source snapshot of **[aantich/go-mariadb](https://github.com/aantich/go-mariadb)** from sibling `../go-mariadb`, including its compression implementation and license notices. `SNAPSHOT.json` records hashes. All connectivity uses its `mariadb` package and `database/sql` connector. No MySQL fallback driver is installed. The snapshot avoids silently changing Lastapp when another chat edits the driver; intentionally refresh/pin it when adopting a reviewed driver change.

The hierarchy is **Tenant → Space → App → micro-app installation**. A Space has one fixed database binding. Spaces can share only within the same Tenant, and dedicated bindings are exclusive. All identities, memberships, groups, permission definitions, roles and assignments live in the central registry. Record-specific shares and business audit stay beside the records and reference global identities and permissions.

Session tokens are hashed at rest; writes require CSRF and exact Origin checks. Authorization reads current grants on each request. List queries apply scope predicates before selecting rows. Cross-Tenant references have composite foreign keys as a second guard. Revisions prevent stale overwrites. Client-generated UUIDs make duplicate creation detectable; retries must reuse the original UUID.

Credentials use AES-GCM with random nonces, key IDs and authenticated owner/Tenant context. API responses and audit events omit secrets. Migration credentials, registry runtime credentials and target provisioning credentials are separate; the development registry account can append/read audit events but cannot update/delete them.

A pinned MariaDB advisory lock elects one provisioning worker. It reconciles durable resource rows; a separate target resource lock prevents simultaneous external DDL. Generated names, stored passwords and database ownership comments make retries stable. Unknown databases/accounts are refused. Retiring registry entries never drops physical data. Schema rollback is deliberately not destructive: restore a backup for rollback.

## Production configuration

Copy `config.example.json` to a protected file and replace its placeholders. Use a separately privileged migration DSN with `lastapp migrate -config /path/to/migration.json`, then run `lastapp serve -config /path/to/runtime.json`. Bootstrap an exact issuer/subject identity using `lastapp bootstrap -config ... -issuer ... -subject ...`; email is never a privilege source.

For local authentication without OIDC, leave the OIDC issuer/client fields empty and bootstrap a dedicated platform account with a private initial-password file:

```sh
lastapp bootstrap -config /path/to/runtime.json -name "Platform operator" -email operator@example.com -password-file /private/path/initial-password
```

The file contains only the initial password (8–128 characters); keep it private and remove it after bootstrap. The first login requires a new password. To grant an already-existing local account platform administration without resetting its password, omit `-password-file`. Platform and Tenant administrative accounts remain separate.

Local passwords use random 16-byte salts and PBKDF2-HMAC-SHA256 with 600,000 iterations, matching the [OWASP PBKDF2 work-factor guidance](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html#pbkdf2). Passwords are never encrypted/recovered or included in API/audit output. Login has per-peer/per-email throttling and bounded hashing concurrency. Changing a password revokes old sessions and rotates the current session and CSRF token. OIDC identities are never automatically linked by email. Email recovery delivery and MFA are later work.

Production requires an HTTPS public origin, trusted HTTPS OIDC when enabled, identity-verified database TLS and an operator-defined runtime account host/network policy. Terminate HTTPS at the trusted ingress. `allowed_networks` explicitly permits private database address ranges; public addresses are otherwise permitted. DNS results are checked again at connection time. Register each physical server under one canonical endpoint: equivalent DNS aliases are not automatically deduplicated.

When a reverse proxy forwards client addresses, configure `trusted_proxies` with its exact connecting IPs or CIDRs (for example, `["10.0.0.10/32"]`). The default empty list ignores forwarded headers. Trusted ingress must overwrite `X-Forwarded-For` or append the actual connecting client; Lastapp walks the chain from right to left and stops at the first untrusted hop. Login budgets then distinguish clients behind that proxy while retaining the per-email limit. Untrusted peers cannot spoof a new budget through forwarding headers; malformed/oversized chains fall back to the direct peer. Do not add client networks to the proxy trust list.

The deployment encryption key is a base64 32-byte key. This first version uses one configured key generation (`encryption_key_id`, default `primary-v1`); automatic key rotation/re-encryption is not implemented. Preserve keys with registry backups.

## Scope of this phase

Implemented: centralized identity/inventory, scoped role management, fixed Tenant/team catalogs, custom Space/App business roles and assignments, plan quotas, lifecycle management, real database provisioning/registration, retries/recovery, auditing and the matching admin UI.

Implemented agent stage: OAuth discovery/automatic connection, existing-user scoped delegation, typed management tools, live context/contracts, receipt-safe mutations and console consent/access management. No business-record tools or robot accounts. See [the MCP guide](MCP.md).

Implemented entity stage: Space-owned definitions and publishing, generated storage, entity/record/field ACLs, defaults/calculations/validation/allowed changes, bulk CRUD and filtered/paginated reads, optional record sharing, App-bound authentication and a small TypeScript SDK. See [the entity guide](ENTITY_RUNTIME.md).

Implemented development/deployment stage: owned isolated workspaces, local React/SDK starter, bounded filesystem build uploads, immutable releases/previews and explicit production deployment. Open **Space → Development**, then **App → Releases**. Remote agents connect through MCP and use their own local tools. See [WORKSPACES.md](WORKSPACES.md).

Later: remote builds, browser Node runners, Git/object-storage integrations, App container scheduling, external connectors, billing and production operations. `compute_mode` records a preference; it does not start containers. Artifact references are metadata. Inventory collections use bounded pages (`?page=1&limit=100&q=search`, maximum limit 200) with `pagination: {page, limit, has_more}`. Scope predicates apply before search and pagination. The console pages/searches those lists and selection controls, including audit history and group members. The entity designer uses the complete Space entity inventory within its entity-count limit; MCP entity inventories expose explicit pagination/completeness. Business-record queries use the separate SDK cursor contract, not management page numbers.

See [FOUNDATION.md](FOUNDATION.md), [CONCEPT.md](CONCEPT.md), and [VERIFICATION.md](VERIFICATION.md). The provisioning plan describes the earlier implementation; its Tenant custom-role model is superseded by the foundation.

## Admin themes

The console defaults to a Paces Neo-inspired design: compact Inter typography,
a pale grouped sidebar, a white workspace, blue accents and dashed card borders.
Use **Account menu → Appearance** to choose **Neo**, **Studio** or
**Evergreen**, with **Light**, **Dark** or **System** color mode. Preferences are
saved on this browser and apply across console screens and forms. Mobile has
an offcanvas menu that includes the Tenant workspace selector.

Themes use shared semantic CSS variables exposed through Tailwind 4 utilities;
new components can use `bg-surface`, `text-ink`, `text-primary`, `border-line`
and `rounded-panel` without choosing a fixed palette. See the
[reference and theme extension guide](design/references/paces-neo.md).


Security fixes and operating boundaries are recorded in [the remediation report](review/security-2026-11-12/FIXES.md). Run `make integration` for the API/security/browser suites. The local Vite UI on port 5173 is an explicit development-only origin; login returns there, and writes use the proxy. Production accepts only its configured admin origin and requires verified TLS for TCP registry connections.
