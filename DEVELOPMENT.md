# Lastapp maintainer guide

Current implementation, 17 November 2026. Start with [AGENTS.md](AGENTS.md).
This guide maps the running framework to its authoritative contracts. The
[concept](CONCEPT.md) and historical plans explain intent, not additional shipped
capabilities.

## Architecture and decisions

```text
Go service
├── Management API + embedded React console + management MCP
├── Shared identity, authorization and entity operations
├── Durable database provisioning / entity publication worker
├── Owned Space workspaces + durable release deployment worker
├── Immutable filesystem builds served on isolated App hosts
└── App-bound data API
    ├── Central MariaDB registry: identities, inventory, roles, definitions,
    │   publication jobs, sessions and agent delegations/receipts
    └── Space-bound MariaDB databases: generated entity tables, record shares,
        local write gates and business-data audit

Tenant
├── Members and teams
└── Space: one database binding + shared entities / rules / ACLs
    └── App: approved client origin + inherited/narrowed access
        └── Micro-app registrations
```

One service currently serves multiple Tenants, Spaces and Apps. An App is the
intended logical deployment/scaling unit, but shared/dedicated container placement
and replicas are not scheduled by this release. A compute preference does not
create a container. Scaling is bounded by databases and runtime capacity; there
is no claim of infinite throughput.

Spaces can reuse a shared database within their Tenant or have an exclusive
dedicated binding, subject to plan policy. Managed provisioning and existing
database registration both work. Space database bindings and resource ancestry
are immutable through ordinary updates. Registering an existing database does
not map its existing tables into entities: authors still describe entities and
Lastapp owns the generated storage namespace.

Global users are not copied per Tenant. Membership connects a user to a Tenant;
teams, fixed control roles, custom scoped business roles and assignments remain
in the registry. One role assignment selects one scope, not arbitrary scope
combinations. A team-scoped Reviewer assignment and a team receiving a Space
role are distinct relationships. See [FOUNDATION.md](FOUNDATION.md).

Space definitions are the reusable backend contract. Apps may inherit or narrow
it through approved ceilings and restrictions. Custom App roles add narrowing
requirements; they cannot create authority beyond the Space. Administrative
authority is separate from business-record access, although entity publication
is powerful enough to deliberately change ACLs. Record shares live with the data
and reference global identities/permissions. See [ENTITY_RUNTIME.md](ENTITY_RUNTIME.md).

Clients contain presentation, actions, navigation and ordinary business workflows.
Server rules protect shared invariants on every write, including bulk operations.
There is no general backend workflow engine or arbitrary backend code runtime.
Client checks improve UX; they never replace server authorization or validation.

## Implemented versus planned

| Implemented | Still planned / unavailable |
| --- | --- |
| Platform and Tenant consoles; local/OIDC accounts; provisioning and inventory | Hosted source library, remote builds, object storage and browser Node runner |
| Shared roles, memberships and entity/record/field ACLs | App-private entity authoring, cross-Space relationships, arbitrary joins |
| Entity designer, draft/test/publish, safe storage evolution | Destructive schema conversions, general backfills, nested JSON queries, full-text/vector search |
| App origin approval, PKCE browser sessions, data SDK and isolated static example | Shared micro-app UI/design package; finer isolation between micro-apps |
| Owned workspaces, local starter/SDK, immutable static releases and explicit production deployment | Git integration, automatic merging, destructive rollback |
| Existing-user delegated OAuth management MCP | Robot accounts and business-record tools in MCP |
| Managed/existing MariaDB storage through the pinned driver | Additional database engines, container scheduling, connectors, billing and production operations |

Generated browser-compatible React/CSS can compile locally and publish through
App releases. Micro-app/shared dependency versions are pinned in an immutable
manifest. See [WORKSPACES.md](WORKSPACES.md) for the full remote-MCP → local
build/test → isolated preview → explicit production flow. App hosts isolate the
console and other Apps; modules compiled into the same App share its authority.
No runner makes every Node/native npm package compatible with the browser.

External integrations will need authorized server-side connection handling;
connector implementation and identity/integration vendor selection remain open.
Do not add external tokens to generated clients or assume a Nango/Auth0 integration
exists. The current login and management OAuth server are implemented in Lastapp.

## Contract and implementation map

| Change area | Authoritative source / reuse point |
| --- | --- |
| Package layout and dependency direction | `cmd/lastapp` → `internal/control` (wiring: routes, sessions/CSRF, MCP adapters, `/data/v1`, workspaces/releases, workers) → domain packages `internal/entities` → `internal/inventory` → `internal/membership` → `internal/roles` → `internal/databases` → `internal/op` → `internal/access` → `internal/registry`. Below control sit `internal/httpapi` (shared HTTP plumbing: `Decode`/`Send`/`Reply`/`SendRef`, the single error writer `WriteError` over `op.Classify`, the console-session wrapper `Session`, page shims, `Observe`/request loggers/`Diagnostic`, `Limiter`, `TrustedProxies.ClientIP`, `AllowedOrigin`/`CookieName`), `internal/sealer` (`Sealer`: owner-bound AES-GCM envelopes from `encryption_key`; format pinned by a fixed vector) and `internal/config`; `internal/entity`, `data`, `authz`, `artifact` are leaves. A lower package never imports a higher one; only `cmd/` and `internal/control` import `internal/control` or `internal/httpapi`; shared queries move down into `registry` (or the owning domain package, e.g. `inventory.AppRow`) rather than behind interfaces. `TestPackageLayering` (`internal/layering_test.go`, `go list`) enforces this table and names the offending import; a new internal package must be added to it. `TestNoHTTPLoopback` fails if an operation package imports `net/http` or `internal/control` |
| Management routes and console-session wrapper | Routes in `internal/control/http.go`; the wrapper (deadline, authentication, limits, initial-password gate, CSRF, error writing) is `httpapi.Session`; domain handlers in `internal/control` |
| Operations shared by console and MCP | `internal/op`: `Actor`, `Runner.Mutate` (lock, live authorization, production guard, receipt, audit), one error `Classify`, `PageRequest`/`Page`, request deadlines; live checks in `internal/access` (`Checker`: `Authorize`, `Scopes`, `CheckAgent`, production/workspace guards; `Principal`, `AgentCall`, agent grants). Domain services `inventory.Inventory` (`internal/inventory`), `membership.Members` (`internal/membership`, also local accounts and the password policy), `roles.Authority` (`internal/roles`, with security audit snapshots), `databases.Storage` (`internal/databases`) and `entities.Authoring` (`internal/entities`); console handlers for them stay in same-named files of `internal/control`. Workspace, release, deployment, starter and audit operations are `Server` methods in `internal/control` (`development_workspaces.go`, `releases.go`, `artifacts.go`, `starters.go`, `http.go`) because they share snapshot and hosting internals. Handlers decode → call the operation → send; MCP adapters call the same operation directly |
| Fixed role IDs/actions/scope categories | `internal/authz/catalog.go`; `internal/access/auth.go`, `internal/roles` resolve current authority |
| Password policy and public settings | `internal/membership/accounts.go` (served by `internal/control/passwords.go`) → `GET /v1/config`; shared frontend controls |
| Entity document, types, expressions, ACL semantics and limits | `internal/entity`; `GET /v1/entity-capabilities` |
| Generated JSON Schema / TypeScript | `internal/entity/contracts.go`; entity/App contract endpoints |
| SQL query/storage compilation and pools | `internal/data`; runtime orchestration in `internal/control/entity_data.go` |
| Publication and database provisioning | `internal/entities`, `internal/databases`; publisher and workers in `internal/control` (`entity_publisher.go`, `worker.go`) |
| App origin approval, launch/session authentication | `internal/entities` (App access, `Authoring.ActorContext`), `internal/control/app_sessions.go`, `entity_data.go`; `packages/client/src/index.ts` |
| MCP schemas, tool catalog and typed adapters | `agent_tools.go`; schemas derive from shared Go input types |
| Agent permissions, limits and live checks | `internal/control/agents.go` → `GET /v1/agent-access/config`; grant checks in `internal/access/delegation.go` |
| OAuth consent, code/refresh flow and client registration | `agent_oauth.go`, `agent_clients.go` |
| Agent context and canonical runtime documentation | `agent_context.go` → `lastapp_context`, `lastapp_describe`, `lastapp://docs/*` |
| Static releases, coherent security activation and status recovery | `releases.go`, `deployment_worker.go`; `GET /v1/apps/{app}/deployment` |
| Registry schema | Ordered `internal/control/migrations/*.sql`; generated `design/control_plane.sql` |
| Typed registry reads | `internal/registry`: `ScanAll`/`ScanOne` (and `op.PageScan`) scan hand-written SQL into record structs whose column list is a const beside each type and whose `ScanFields()` returns the scan destinations; `JSONValue` (`*_json` columns), `Bit` (TINYINT 0/1) and pointers (NULL) keep the JSON identical to the former `Row` maps. Plans, Tenant policy, quotas, record locks and the shared `APIError` live there too; `op.Classify` maps MySQL errors. `QueryRow`/`QueryRows` remain only for dynamic business records and their record ACLs; `TestRegistryRecordsMatchUntypedRows` and `TestRegistryRecordKeysAreSorted` guard the shapes |
| Console composition and policy consumers | `web/admin/src`; shared shell/forms/collections/selectors/themes; pages under `tenant/`, `platform/`, `entity-designer/` |
| Console data layer | Reads: `query.ts` (`useApiQuery`, TanStack Query cache, `refetchInterval` polling); mutations: `use-action.ts` (`useAction`, invalidates reads) |
| Console conventions | `useAction` is the only screen mutation path; confirmations use `confirmAction` (`confirm.ts`, `confirm-dialog.tsx`), never `window.confirm`; every console URL is built and parsed in `console-url.ts`; render and lazy-load failures stay in the shell via `error-boundary.tsx` and `retryable-lazy.ts`; choices and numbers use `select.tsx` / `number-input.tsx` (native `<select>`, date and number inputs are not allowed). No lint suppression file, `//nolint` or inline ESLint disable; the loose `Item` row type in `api-types.ts` is `Record<string, unknown>`, narrowed at use sites (`text`, `num`, `isItem`, `items` or a named shape) |

To add a management operation: write a typed method in its domain package that
takes an `op.Actor` and runs writes through `Runner.Mutate` (for example
`inventory.CreateTenant`); add a thin decode → call → send handler in
`internal/control`; expose it to MCP by an adapter in `agent_tools.go` that calls
the same method; regenerate the goldens (below) for the new tool schema/response;
and add a regression test for its authorization boundary.

Do not add a second policy language, ORM, query compiler, permission cache or MCP
service to make a local feature easier. Extend the existing typed contract and
shared operations. Keep the Go evaluator and parameterized SQL semantics aligned,
including exact decimals, nulls, permission paths and old/new update checks.

## API boundaries

| Surface | Authentication and purpose | Contract entry points |
| --- | --- | --- |
| `/auth/*`, `/v1/*`, `/admin/v1/*` | Console session; exact Origin and CSRF for writes; administrative/authoring operations | `http.go`, [FOUNDATION.md](FOUNDATION.md), [entity guide](ENTITY_RUNTIME.md) |
| `/data/v1/*` | App-bound bearer, approved origin and current user authority; business-record APIs | SDK in `packages/client/src/index.ts`; dispatch in `entity_data.go` |
| `/mcp` | OAuth bearer bound to this resource and explicit user delegation; management only | [MCP.md](MCP.md), generated tool schemas/catalog and runtime guides |
| `/.well-known/oauth-*`, `/oauth/*` | Discovery, public-client setup and interactive delegated authorization | `agent_oauth.go`, `agent_clients.go`; no authority from registration alone |

Console, App and MCP credentials are separate; none is a general-purpose token
for the other surfaces. MCP invokes the same management operations in process,
not an unauthenticated HTTP endpoint or a synthetic superadmin session. Every
management domain exposes typed operations that take an explicit `Actor`; console
handlers decode, call and send, and MCP tools call the same operations. The former
in-process HTTP adapter (`executeManagement`) is deleted and `TestNoHTTPLoopback`
keeps it from returning.
Golden files in `internal/control/testdata` pin the MCP tool schemas and the
response shapes of representative endpoints (`internal/control/golden_test.go`):
inventory/MCP management (`response_shapes`), workspaces/builds/releases/
deployments/starters and the MCP workspace/release tools (`delivery_shapes`),
`/data/v1` (`data_shapes`), console sessions, password/OIDC login, CSRF and agent
OAuth including cookie names/attributes (`session_shapes`), and normalised
`audit_events` attribution for human, agent and worker steps (`audit_attribution`).
Refactors must leave them unchanged. Regenerate them only for an intended contract
change with `go test ./internal/control -run Golden -update-golden` (all but the
tool schema golden need `LASTAPP_TEST_CONFIG`) and review the diff.

Important management groups are Tenant/Space/App/micro-app inventory, scoped
`roles`, `role-assignments`, `recipients`, memberships/teams, database resources,
Space `entities`, entity validation/test/publications, and App `data-access` and
`entity-contracts`. See the actual route registrations rather than guessing paths
from names. Role/permission projections are not an independent ACL editing API.

Management inventory collections use page/limit/search metadata. The console
entity designer loads its complete Space inventory within the entity-count cap;
MCP entity inventories are explicitly paged. Entity data reads use the SDK's
structured filter/sort/cursor contract. These are different interfaces.
Consume `pagination.has_more` or `next_cursor` as appropriate. Do not assume the
first page is a complete inventory. Use current revisions on guarded writes;
stale revisions must fail rather than silently overwrite another author's work.

## Local workflow and verification

Run from the repository root. [README.md](README.md) covers prerequisites and
development identities. `make dev` builds the embedded console/binaries and
starts the owned local MariaDB, API and development issuer while preserving data.
The console is at `http://127.0.0.1:8787/console/`; MCP is `/mcp` on the same origin.
If Go is run directly in a restricted environment, set
`GOCACHE="$PWD/.local/go-cache"`. Make targets already do this. Make targets use
`node`/`npm` from PATH; pass `NODE_BIN=/path/to/node/bin` to prefer another Node
rather than replacing the global toolchain.

```sh
make test                       # unit/build checks; DB tests may skip
make lint                       # golangci-lint v2.15.0 + console npm run lint/typecheck
make fmt                        # gofumpt/gci via golangci-lint fmt
GOCACHE="$PWD/.local/go-cache" go vet ./...
make mcp-test                   # TestAgent* only: MCP/OAuth/agent security + agent worker
make integration                # full backend race/security + browser suite
make e2e                        # browser suite only, on a disposable stack
make client                     # SDK + example client.js; needs web/admin npm ci
python3 design/validate_schema.py --check  # offline DDL parity only
python3 scripts/verify_driver.py           # pinned driver integrity
git diff --check
```

CI (`.github/workflows/checks.yml`, job `test`) runs the same lint, type, schema,
driver, race-test and browser checks, plus `govulncheck`.

Integration/database tests, `TestNoHTTPLoopback` and the goldens currently live in
`internal/control`, so `make integration` and `make mcp-test` target that package.
The agreed direction is that each layer gets its own tests and its own fixtures;
do not add a shared test-environment package.

`make mcp-test` and the backend half of `make integration` require the existing
local development config/server. `make mcp-test` runs only the `TestAgent*` control
tests with race detection; the general security suite (`LASTAPP_SECURITY_CONFIG`)
runs under `make integration`, whose browser half is `make e2e` (below). `make client`
compiles with the console's TypeScript, so run `make web` (or `make dev`) once first;
`make e2e` runs `make client` automatically through the `pretest:e2e` hook. Database fixtures use disposable registries; worker tests clean
their owned generated resources. Do not point them at production. Plain Go tests
without explicit fixture config are not integration evidence. The browser suite never touches the development stack or its data: `make e2e`
(`scripts/e2e.py`, also used by CI) starts a fresh MariaDB from
the `mariadb-e2e` service of `docker-compose.yaml` (profile `e2e`, tmpfs, port 33318; only that container is removed, never the development volume),
prepares it the way CI does (`scripts/ci_config.py` + `TestPrepareFixture`),
builds the current source and runs a separate API on 8797, development issuer on
8798 and Vite on 5183, then stops them and removes the container. Its disposable
config, secrets, binaries, artifacts and logs are in `.local/e2e/` (recreated each
run). The ports are chosen only in `scripts/e2e.py`; tests read them through
`LASTAPP_E2E` (`web/admin/tests/stack.ts`), so `npm run test:e2e` alone refuses to
run. Pass Playwright arguments with `make e2e ARGS=tests/security.spec.ts`. Docker
is required. Chrome is used locally, Chromium in CI.

For registry changes, add a forward Goose migration (the startup schema check
derives its required version from the newest embedded migration), then regenerate
the SQL document with `python3 design/validate_schema.py --write`. `--check` compares the document
with the migrations without database access. Without a flag the validator does the same
comparison and then runs `TestAuthorizationFoundation` on a disposable registry on
the existing local server (`LASTAPP_TEST_CONFIG`, default `.local/config.json`). It does not test
every later feature; use the relevant entity/MCP/security suites too. Never edit
the generated SQL independently of migrations.

Runtime UPDATE,DELETE grants are derived from the migrations: every new table
receives them unless it is listed in `appendOnlyRegistryTables`
(`internal/control/grants.go`). `lastapp runtime-grants -database NAME -host HOST`
prints those statements without configuration or a database connection;
`scripts/dev.py` and the CI fixture apply the same list.

For security changes, add a regression that fails for the actual boundary being
fixed. Reuse existing test fixtures/helpers and runtime policy catalogs. For UI
changes, verify real flows, desktop/mobile layout, errors and stale editor state.
Capture meaningful results in [VERIFICATION.md](VERIFICATION.md), including skips,
failures and reruns; do not substitute a plan or old receipt for executed checks.

Logging uses `log/slog`: text in development, JSON lines otherwise (stderr;
`make dev` writes `.local/lastapp.log`). The outer handler (`httpapi.Observe`, `internal/httpapi/logging.go`)
is the single source of `X-Request-ID`, puts a request-scoped logger
(`request_id`, `method`) into the context and writes one `http request` line with
the route pattern, status, bytes and duration. It never logs query strings,
bodies, cookies or credential headers. Errors are wrapped with `%w`; `httpapi.Diagnostic`
logs only the type chain, SQL number/state, network operation and context state,
never error messages, and unmapped API errors stay a generic 500.

`main` is protected by the GitHub ruleset `main`: changes arrive only through a
pull request whose required `test` check passed on a branch up to date with
`main`; force-pushes and deletion are blocked.

Keep `.local/`, keys, passwords, build output, traces and screenshots out of git.
No routine change requires resetting local data, refreshing the driver snapshot,
testing other database versions or running driver benchmarks.

## Documentation maintenance

The current reading path is README → this guide → FOUNDATION / ENTITY_RUNTIME /
MCP / WORKSPACES as needed. AGENTS.md is the policy entry point; CLAUDE.md imports it. Update
the relevant current guide and runtime agent documentation when behavior changes.
Schemas, limits, catalogs and generated contracts remain code-derived; prose
explains their purpose instead of maintaining another executable definition.

CONCEPT and the old provisioning/authorization/implementation plans preserve
decisions and proposals. Their status banners link to current behavior. Historical
review/verification records retain their original scope and dates. Do not revive
superseded custom Tenant roles, table-mapping UI, a mandatory workflow DSL or
unimplemented connector/runner capabilities from those documents.
