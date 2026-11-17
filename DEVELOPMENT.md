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
| Package layout and dependency direction | `cmd/lastapp` → `internal/control` (wiring: routes, the single `/data/v1/` mount, the build upload endpoint and App host server, the worker step list, trusted platform bootstrap) → `internal/worker` (the background leader: `worker.Leader` holds one `GET_LOCK` on a pinned registry connection and runs each `Step` in its own loop while it holds it, see [Background worker](#background-worker); the list — `Cleanup`, deployments, workspaces, `Publication`, `databases.Provisioner` — is built once in `Open`; it imports only `access`, `data`, `op` and `registry`, never a transport package) → `internal/agent` (agent delegation: `agent.Server` serves the OAuth authorization server for MCP clients — discovery, registration, consent, code/refresh exchange and revocation — the console agent-access endpoints and the streamable-HTTP management MCP server with its tool catalog and typed adapters; wired in `Open` with the registry DB, `access.Checker`, config, the domain services (`inventory`, `membership`, `roles`, `databases`, `entities`, `delivery`), the shared `Limiter`/trusted proxies and its admission slots; control mounts it with `Routes(mux, console)`, passing the console-session wrapper so consent authenticates console users without importing `session`) → `internal/session` (console sessions: `session.Manager` serves password login, OIDC login/callback with the sealed flow cookie, session issue and cookies, principal resolution, CSRF/Origin checks for console writes, logout and password change; wired in `Open` with the registry DB, config, `Sealer`, the shared `membership.Passwords` pool and the trusted proxies, and it owns the login limiter; control only registers its routes) → `internal/dataapi` (the App data API: `dataapi.Runtime` serves `/data/v1`, App launch codes, PKCE exchange and bearer sessions with their live parent checks; wired in `Open` with the registry DB, `access.Checker`, `entities.Authoring`, `publisher.Publisher`, `Sealer`, config and the shared `Limiter`/trusted proxies) → `internal/delivery` (owned workspaces, snapshots, build upload tickets/storage, releases, the production deployment steps, starters and development launch codes: `delivery.Service`, wired in `Open` with the registry DB, `op.Runner`, `access.Checker`, config, `databases.Storage`, `publisher.Publisher`, registry name and warning logger; it does not import `net/http` or `httpapi`, so HTTP decoding, upload rate limits/slots and static file serving stay in `control`) → `internal/publisher` (entity publication jobs, activation and business-database connections/pools) → domain packages `internal/entities` → `internal/inventory` → `internal/membership` → `internal/roles` → `internal/databases` → `internal/op` → `internal/access` → `internal/registry`. Below control sit `internal/httpapi` (shared HTTP plumbing: `Decode`/`Send`/`Reply`/`SendRef`, the single error writer `WriteError` over `op.Classify`, the console-session wrapper `Session`, page shims, `Observe`/request loggers/`Diagnostic`, `Limiter`, `TrustedProxies.ClientIP`, `AllowedOrigin`/`CookieName`), `internal/sealer` (`Sealer`: owner-bound AES-GCM envelopes from `encryption_key`; format pinned by a fixed vector) and `internal/config`; `internal/entity`, `data`, `authz`, `artifact` are leaves. A lower package never imports a higher one; the transport packages `internal/control`, `internal/agent`, `internal/session`, `internal/dataapi` and `internal/httpapi` are imported only by `cmd/` and each other (`agent`, `session` and `dataapi` serve HTTP themselves, so they reuse `httpapi` rather than duplicating decode/send, errors, limits, origins, cookie names and client IPs; they are siblings that only `control` imports; agent OAuth consent gets the console principal through the `httpapi.Session` wrapper control passes to `agent.Server.Routes`, not by importing `session`); token hashing (`registry.TokenHash`) and PKCE formats and the S256 check (`registry.PKCEVerifier`/`PKCEChallenge`/`PKCEMatches`) are shared from `internal/registry`; shared queries move down into `registry` (or the owning domain package, e.g. `inventory.AppRow`) rather than behind interfaces. `TestPackageLayering` (`internal/layering_test.go`, `go list`) enforces this table and names the offending import; a new internal package must be added to it. `TestNoHTTPLoopback` fails if an operation package (including `internal/delivery`), or the non-HTTP files of `internal/dataapi` (`sql.go`, `cache.go`, `fairness.go`), imports `net/http` or `internal/control`, if `internal/agent`, `internal/dataapi` or `internal/session` imports `internal/control`, and if any of them references the removed MCP→HTTP loopback |
| Management routes and console-session wrapper | Routes in `internal/control/http.go`; the wrapper (deadline, authentication, limits, initial-password gate, CSRF, error writing) is `httpapi.Session`, given `session.Manager.Principal` and `CSRF`; login, logout and password-change handlers in `internal/session`; domain handlers in `internal/control` |
| Operations shared by console and MCP | `internal/op`: `Actor`, `Runner.Mutate` (lock, live authorization, production guard, receipt, audit), one error `Classify`, `PageRequest`/`Page`, request deadlines; live checks in `internal/access` (`Checker`: `Authorize`, `Scopes`, `CheckAgent`, production/workspace guards; `Principal`, `AgentCall`, agent grants). Domain services `inventory.Inventory` (`internal/inventory`), `membership.Members` (`internal/membership`, also local accounts and the password policy), `roles.Authority` (`internal/roles`, with security audit snapshots), `databases.Storage` (`internal/databases`), `entities.Authoring` (`internal/entities`) and `publisher.Publisher` (`internal/publisher`, wired in `Open` with the deployment authority recheck and deployment snapshot entities from `internal/delivery`, and the database connector and warning logger from control) and `delivery.Service` (`internal/delivery`: workspace, build, release, deployment, starter and development-launch operations, which share snapshot and hosting internals); console handlers for them stay in same-named files of `internal/control` (`development_workspaces.go`, `releases.go`, `artifacts.go`, `starters.go`). The account list and audit trail reads are `inventory.Inventory.Users` and `roles.Authority.Audits`. Handlers decode → call the operation → send; MCP adapters in `internal/agent` call the same operation directly |
| Fixed role IDs/actions/scope categories | `internal/authz/catalog.go`; `internal/access/auth.go`, `internal/roles` resolve current authority |
| Password policy and public settings | `internal/membership/accounts.go` (policy, verifiers, `Passwords.PrepareAccount` for every account-creation path and the single PBKDF2 work pool `membership.Passwords` shared with login and password change in `internal/session/passwords.go`) → `GET /v1/config` (`internal/control/http.go`); shared frontend controls |
| Entity document, types, expressions, ACL semantics and limits | `internal/entity`; `GET /v1/entity-capabilities` |
| Generated JSON Schema / TypeScript | `internal/entity/contracts.go`; entity/App contract endpoints |
| SQL query/storage compilation and pools | `internal/data` (also `ReleasePinnedLock`); business-database connections and the runtime pool cache are `publisher.Publisher.BusinessDB`/`RuntimeDB`; runtime orchestration in `internal/dataapi` (`runtime.go` dispatch and actor/definition loading, `records.go` reads/mutations, `sql.go` share/recipient/cursor SQL builders, `shares.go`, `options.go`; stored rows decode through `entities.BusinessRecord`) |
| Publication and database provisioning | `internal/entities` (queueing), `internal/databases` (resources, `OwnershipConflict`); `internal/publisher` (`Publisher.Job`, `Activate`, `RestoreBase`, `OpenGate`, permission projection, storage preparation); the worker leader (`GET_LOCK`, term start, per-step loops, heartbeat, idle pause, step backoff and transient retry) in `internal/worker/worker.go` with the cleanup, Tenant-fair queue (`Fair`) and publication adapters in `steps.go`; provisioning (`databases.Provisioner`: interrupted attempts back to pending at term start, live agent recheck, round-robin `NextJob`, `Provision`, `ValidateRuntime`, `ProvisionError`) in `internal/databases/provisioner.go`, with the target connector injected from control like `publisher.Connect`; expired agent token sessions, OAuth requests and stale OAuth clients are deleted by `access.CleanupAgentSessions`/`CleanupClients` (`internal/access/cleanup.go`, which also owns the client lifetimes); workspace preparation and deployment steps are `delivery.Service.WorkspaceJob`/`DeploymentJob` (`internal/delivery/snapshot.go`, `deployments.go`) |
| App origin approval, launch/session authentication | `internal/entities` (App access, `Authoring.ActorContext`), `internal/dataapi/sessions.go` (console launch, PKCE exchange, bearer sessions and the shared browser/delegated parent check), `internal/dataapi/runtime.go`; development launch codes in `internal/delivery/sessions.go` (the shared code lifetime is `registry.LaunchLifetime`); `packages/client/src/index.ts` |
| MCP schemas, tool catalog and typed adapters | `internal/agent`: `tools.go` (catalog `Server.Tools`, `InputSchema`, strict input decoding, visibility), `mcp.go` (`/mcp` admission and transport), `writes.go` (request keys, receipts, response bound), adapters by domain in `context.go` (reads), `resources.go`, `entities.go`, `access.go`, `databases.go`, `development.go`; schemas derive from the exported Go input types |
| Agent permissions, limits and live checks | `internal/agent/permissions.go` (catalog, presets), `server.go` (limits, bearer `Principal`, admission) → `GET /v1/agent-access/config`; grant checks in `internal/access/delegation.go` |
| OAuth consent, code/refresh flow and client registration | `internal/agent/oauth.go`, `clients.go` |
| Agent context and canonical runtime documentation | `internal/agent/context.go` → `lastapp_context`, `lastapp_describe`, `lastapp://docs/*` |
| Static releases, coherent security activation and status recovery | `internal/delivery/releases.go`, `deployments.go`; `GET /v1/apps/{app}/deployment` |
| Registry schema | Ordered `internal/control/migrations/*.sql`; generated `design/control_plane.sql` |
| Typed registry reads | `internal/registry`: `ScanAll`/`ScanOne` (and `op.PageScan`) scan hand-written SQL into record structs whose column list is a const beside each type and whose `ScanFields()` returns the scan destinations; `JSONValue` (`*_json` columns), `Bit` (TINYINT 0/1) and pointers (NULL) keep the JSON identical to the former `Row` maps. Plans, Tenant policy, quotas, record locks and the shared `APIError` live there too; `op.Classify` maps MySQL errors. `QueryRow`/`QueryRows` remain only for dynamic business records and their record ACLs; `TestRegistryRecordsMatchUntypedRows` and `TestRegistryRecordKeysAreSorted` guard the shapes |
| Console composition and policy consumers | `web/admin/src`; shared shell/forms/collections/selectors; Signal tokens in `themes.css` and the UI library entry `src/ui/` (see [Console UI library](#console-ui-library)); pages under `tenant/`, `platform/`, `entity-designer/` |
| Console data layer | Reads: `query.ts` (`useApiQuery`, TanStack Query cache, `refetchInterval` polling); mutations: `use-action.ts` (`useAction`, invalidates reads) |
| Console conventions | `useAction` is the only screen mutation path; confirmations use `confirmAction` (`ui/confirm.ts`, `ui/confirm-dialog.tsx`), never `window.confirm`; every console URL is built and parsed in `console-url.ts`; render and lazy-load failures stay in the shell via `ui/error-boundary.tsx` and `retryable-lazy.ts`; screens use the `src/ui` kit only; choices and numbers use `Select` / `NumberInput` (native `<select>`, date and number inputs are not allowed). No lint suppression file, `//nolint` or inline ESLint disable; the loose `Item` row type in `api-types.ts` is `Record<string, unknown>`, narrowed at use sites (`text`, `num`, `isItem`, `items` or a named shape) |

To add a management operation: write a typed method in its domain package that
takes an `op.Actor` and runs writes through `Runner.Mutate` (for example
`inventory.CreateTenant`) and its reads through `Runner.Authorize`/`Runner.Scopes`
(never `Checker.Authorize` directly: the Runner attaches the actor's agent call, so
the grant's tool permissions apply even when the context carries no call,
`TestAgentAuditAndUserReadsNarrowed`); add a thin decode → call → send handler in
`internal/control`; expose it to MCP by an adapter in `internal/agent` (the
domain's file, listed in `Server.Tools`) that calls the same method; regenerate the goldens (below) for the new tool schema/response;
and add a regression test for its authorization boundary.

### Background worker

One in-process leader per registry (`worker.Leader`, wired in `control.Open`)
runs the queues. Election stays single: one `GET_LOCK(lastapp:<registry>,0)` on
a pinned registry connection. A term then runs as follows.

- **Recovery first.** Every `Starter.Start` runs once, in step order, on the
  lock connection before any step runs (cursor resets; the provisioning step
  returns interrupted `provisioning` resources to `pending`).
- **Concurrency.** Each step then runs in its own goroutine loop, so a slow step
  (provisioning: up to 45 s plus a 5 s target lock; publication and deployment
  attempts: 45 s) never delays another kind of work. Steps get the leader's pool,
  not a pinned connection: they borrow a connection per query or transaction,
  never share a session and never use the lock connection (pinned out of the
  pool for the term). No step relies on session state of the lock connection:
  their transactions set `READ COMMITTED` per transaction, and the session-level
  locks they take (`lastapp:entity:*`, `lastapp:resource:*`) are on their own
  business/target connections. At most one goroutine runs each step, so a step's
  per-term state needs no lock.
- **Connections.** The worker has its own registry pool (`control.Open` wires a
  second delivery service, publisher and the provisioner on it through
  `deliveryServices`), so background work never takes a connection from
  console, MCP or data API requests, whose pool stays at 16. It is capped at
  1 + 2 × steps (11): the lock connection plus, per step, at most one open
  transaction and one read beside it, so steps never wait on each other for a
  connection. `TestWorkerOwnPool` keeps the request pool fully checked out and
  requires the worker to finish jobs anyway.
- **Pacing.** A run that did no work pauses a second; a busy run (a provisioning
  or publication job, a deployment/workspace job that progressed) loops at once.
- **Error isolation.** A step error is logged as `worker step failed` with the
  step name and the step backs off (1 s doubling to 60 s, reset on success); the
  other steps and the lock are unaffected. A MariaDB deadlock (1213) or lock wait
  timeout (1205), `op.Transient`, is a retry after 100 ms, not a failure; the
  deployment, workspace and publication jobs return it instead of failing their
  durable job (a publication stays `publishing` and resumes under a new attempt).
- **Term end.** The leader checks every second that its connection still holds
  the lock (`IS_USED_LOCK(...)=CONNECTION_ID()`). A broken connection, a lost lock
  or shutdown cancels the term; every step goroutine is waited for before the
  lock is released, and leadership is retried a second later
  (`worker leader disconnected`). Until the heartbeat notices a lost lock (≤ 1 s)
  an old step may still finish a job while a new leader recovers; the attempt-id
  and physical version fences, `WHERE state=...` claims and live authority
  rechecks make that overlap safe, as before.
- **Tenant fairness.** Provisioning (`databases.NextJob`) and publication
  (`Publisher.Job` cursor) round-robin over Tenants. Deployments and workspaces
  use `worker.Fair`: `DeploymentJob`/`WorkspaceJob` work on the first Tenant
  after a cursor and report whether a job progressed. One pass visits each
  Tenant with work at most once and stops at the first job that progresses; a
  Tenant with nothing that can progress is skipped, and a failing Tenant is
  passed over on the next pass.
- **Order within a Tenant.** Deployments keep strict creation order (oldest
  unfinished first): each one is checked against the production base hash and
  changes the Space's shared entities, roles and App policies, so a later
  deployment must not overtake an earlier one; a waiting deployment holds back
  only its own Tenant's later deployments. Workspaces need no order: each has its
  own backing Space and storage, and its preparation is a per-workspace state
  machine. `WorkspaceJob` therefore tries the Tenant's preparing workspaces
  oldest first and stops at the first that progresses, so a workspace waiting
  for its storage or publications never holds back another, and one workspace's
  error does not stop the others in that pass.
- **Lock order.** Concurrent steps take registry row locks in the same order as
  console mutations: the Tenant row (`tenants … FOR UPDATE`) first, then the
  Tenant's resources (`database_resources`, `entity_definitions`/
  `entity_versions`, `apps`, roles, `release_deployments`,
  `development_workspaces`). Publication's own finishing transactions lock an
  entity's version before its definition without the Tenant row; nothing locks
  them in the opposite order for the same version. Advisory locks are per
  entity on the business database (publication holds it while it takes the
  Tenant row; no transaction holding a Tenant row waits for it, and
  `RestoreBase` only tries it without waiting, after commit) and per resource on
  the target (provisioning holds no registry lock meanwhile). Cleanup touches
  only session/ticket/OAuth tables.
- **Fences across steps.** Steps that used to run strictly one after another can
  now interleave (e.g. a deployment failing while publication is preparing one
  of its versions). The publisher's `attempt_id` and `state='publishing'`
  claims, the deployment authority recheck inside the activation transaction and
  the storage gate (`_lastapp_entity_state.attempt_id`) keep the outcome
  consistent; the late publication then records its own failure.

Do not add a second policy language, ORM, query compiler, permission cache or MCP
service to make a local feature easier. Extend the existing typed contract and
shared operations. Keep the Go evaluator and parameterized SQL semantics aligned,
including exact decimals, nulls, permission paths and old/new update checks.

## Console UI library

The console has one visual theme, **Signal** ("Ink & Signal"), in light, dark
and system color modes. New console UI follows these rules:

- **Tokens.** `web/admin/src/themes.css` is the only source of colors, fonts,
  radii and shadows: semantic `--ui-*` variables, redefined for
  `:root[data-mode="dark"]` and, for the ink sidebar, inside `.sidebar`. Tailwind
  4 maps them to utilities (`bg-surface`, `text-ink`, `text-muted`,
  `text-primary`, `border-line`, `rounded-panel`, `font-heading`, `font-mono`,
  `bg-success-soft`, …). Do not write literal colors in components and do not
  add theme presets; a new value is a new token in `themes.css`, defined for
  both modes with AA contrast (text 4.5:1, marks and focus rings 3:1) and, for
  status colors, a lightness difference rather than hue alone.
- **Mode.** `data-mode` on `<html>` is set by `public/appearance-init.js` before
  CSS loads and by `applyMode` in `src/appearance.ts`. The browser preference
  `lastapp-appearance` stores only `{ mode }`; a `theme` saved by the former
  presets is ignored.
- **Fonts.** Inter is bundled from `src/assets`; Inter Tight (headings) and
  JetBrains Mono (identifiers) come from the pinned `@fontsource-variable`
  packages. Vite emits fonts as files (`vite.config.ts`), because the console CSP
  allows no `data:` fonts. No font is loaded from a CDN.
- **Kit only.** `web/admin/src/ui/` is the console UI kit and `src/ui/index.ts`
  its single entry point (no re-exports from old module paths). Screens are
  built only from kit components and `--ui-*` tokens: a screen that needs a new
  control, layout part or state gets it added to the kit, with stories, rather
  than hand-written markup. Kit styles live in `src/ui/kit.css`; kit components
  keep the established class names that screens and e2e tests match on
  (`.primary`, `.secondary`, `.danger`, `.icon`, `.badge`, `.error`,
  `.overlay`/`.modal`, `.tabs`, `.empty`, `.panel`, `.sidebar-collapsed`);
  everything new is `ui-`-prefixed. Native `<select>`, date and number inputs
  stay forbidden; TextField offers no number/date type.
- **Inventory.**
  - Primitives: `Button`/`IconButton` (primary, secondary, ghost, danger, link;
    sm/md; loading; the icon button's label is required), `StatusBadge` with
    the one state → tone map `statusTone` (`status-tone.ts`: success, warning
    for work in progress and suspension, danger, info for drafts/review,
    neutral; a different mark shape per tone), `Badge` (tag, mono, count),
    `Card`/`CardHeader`/`CardBody`/`CardFooter`, `StatCard`, `AttentionCard`,
    `PageHeader`, `Breadcrumbs`, `Tabs` (WAI-ARIA tablist, roving tabindex,
    automatic or manual activation), `EmptyState`, `Alert` (danger keeps
    `.error` and role=alert) and `InlineError`, `Dialog` (the one modal shell
    on `useDialogFocus`; FormModal, ConfirmDialog, the agent access editor and
    the entity designer's editors wrap it), `toast()` and `Toaster` (aria-live
    regions without alert/status roles), `Checkbox` (native input, custom
    17 px face, indeterminate), `Switch` (role=switch), `RadioGroup`,
    `FormField`/`TextField`/`Textarea` (label, hint and error wired by id),
    `Stepper`, `Progress`, `Timeline`, `Avatar` (person, agent, system),
    `Tooltip`, `Spinner`, `Skeleton`, `Kbd`.
  - Composites: `SidebarNav`/`SidebarFooter` and `useSidebarCollapse` (the
    68 px rail, remembered in the browser as `lastapp-sidebar`; ConsoleShell
    uses them), `ContextSwitcher` (Tenant/Space popover: filter combobox over a
    listbox), `CommandPalette` and `useCommandPaletteShortcut` (⌘K/Ctrl+K),
    `DataTable` (Collection adds server search and paging on top) and
    `RoleMatrix`.
  - Form controls and services: Select, ListboxPopup/`useListbox`
    (`listbox.ts`), RemoteSelect, NumberInput, NewPasswordInput,
    ConfirmDialog/ConfirmHost/`confirmAction`, FormModal, Collection,
    ErrorBoundary, `useDialogFocus`.
- **Not yet on the kit.** Screens still hand-write page headings, tab strips
  (their tests click tabs as buttons, so moving them to `Tabs` changes the
  e2e selectors), most buttons and the Tenant switcher (a paged
  RemoteSelect). The screen rewrites move these onto the kit.
- **Stories.** Every kit component has stories
  (`src/ui/controls/*.stories.tsx`, grouped as Primitives, Composites and
  Controls) with realistic Northstar data;
  `src/ui/foundations/` shows the tokens (read from `themes.css`, both modes,
  contrast ratios), typography, spacing/radius and the lucide icons in use.
  Stories supply the password policy and mock list APIs (`src/ui/storybook/`);
  they never copy server limits.

Storybook is local only: there is no CI step and nothing is published.

```sh
cd web/admin
npm run storybook        # http://127.0.0.1:6006, light/dark/system toolbar
npm run build-storybook  # static build in storybook-static/ (ignored by git)
```

`npm run lint` and `npm run typecheck` (and so `make lint`) cover `.storybook/`
and all stories. The console build bundles only what `index.html` reaches, so
stories never ship in the console.

## API boundaries

| Surface | Authentication and purpose | Contract entry points |
| --- | --- | --- |
| `/auth/*`, `/v1/*`, `/admin/v1/*` | Console session; exact Origin and CSRF for writes; administrative/authoring operations | `http.go`, `internal/session`, [FOUNDATION.md](FOUNDATION.md), [entity guide](ENTITY_RUNTIME.md) |
| `/data/v1/*` | App-bound bearer, approved origin and current user authority; business-record APIs | SDK in `packages/client/src/index.ts`; dispatch in `internal/dataapi/runtime.go` |
| `/mcp` | OAuth bearer bound to this resource and explicit user delegation; management only | [MCP.md](MCP.md), generated tool schemas/catalog and runtime guides |
| `/.well-known/oauth-*`, `/oauth/*` | Discovery, public-client setup and interactive delegated authorization | `internal/agent/oauth.go`, `clients.go`; no authority from registration alone |

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
make mcp-test                   # TestAgent* only (agent + control): MCP/OAuth/agent security + agent worker
make integration                # full backend race/security + browser suite
make e2e                        # browser suite only, on a disposable stack
make client                     # SDK + example client.js; needs web/admin npm ci
(cd web/admin && npm run build-storybook)  # local Storybook build; not in CI
python3 design/validate_schema.py --check  # offline DDL parity only
python3 scripts/verify_driver.py           # pinned driver integrity
git diff --check
```

CI (`.github/workflows/checks.yml`, job `test`) runs the same lint, type, schema,
driver, race-test and browser checks, plus `govulncheck`.

Integration/database tests over HTTP/MCP, `TestNoHTTPLoopback` and the goldens
live in `internal/control`, so `make integration` and `make mcp-test` target that
package. `internal/publisher` has its own storage tests and a disposable
business-database fixture (they skip without `LASTAPP_TEST_CONFIG`; run them with
`go test ./internal/publisher`). `internal/dataapi` has database-free unit tests
for its SQL builders, gates, definition cache and Tenant fairness
(`go test ./internal/dataapi`); its HTTP behaviour is covered black-box by the
`/data/v1` tests and `data_shapes` golden in `internal/control`.
`internal/delivery` has database-free tests (snapshot remapping, contract
hashes, deployment plans, pinned versions, hosted origins) and, with
`LASTAPP_TEST_CONFIG`, a disposable registry fixture built from the migrations
for deployment candidate entities, the production App quota and capture
readiness (`go test ./internal/delivery`); the workspace/release/deployment
flows and the `delivery_shapes` golden stay black-box in `internal/control`.
`internal/session` has database-free tests for cookie names/attributes, CSRF
and Origin checks, the per-client/per-email login limiter, the shared password
work bound, password policy application on change and the OIDC flow (sealed
state, expiry, nonce) against a stub token endpoint and key set
(`go test ./internal/session`); `internal/membership` tests account preparation
and the work bound. Password/OIDC login, CSRF and logout over HTTP and the
`session_shapes` golden stay black-box in `internal/control`.
`internal/agent` has database-free tests for the tool table (names, adapters,
schemas, catalog permissions, visibility), the permission catalog and OAuth
scope normalization, configured-client and redirect/SSRF checks, pre-database
MCP admission and the response bound on committed results
(`go test ./internal/agent`); OAuth, consent, token rotation and the MCP tools
run black-box (and through `Server.Tools`) in `internal/control`, with the
`mcp_tools`, `response_shapes`, `session_shapes`, `delivery_shapes` and
`audit_attribution` goldens.
`internal/worker` tests the leader with fake steps on a lock-only fixture
(`LASTAPP_TEST_CONFIG`, no schema): starters once per term on the lock
connection before any step, steps on connections of their own, standing by while
another session holds the lock, the per-step idle pause, transient retries,
error isolation with backoff and no lost leadership, a slow step not delaying
another, and term end (killed lock connection) stopping every step before the
next term's recovery (`go test ./internal/worker`); the `Fair` and publication
adapters have database-free tests. `TestWorkerTenantFairness`
(`internal/control`) runs the real leader: a Tenant's waiting deployment and
workspace do not block another Tenant's; `TestWorkerWorkspacesIndependent`: a
waiting workspace does not block a newer one of the same Tenant. `internal/databases` has database-free tests for the stored provisioning
error mapping and generated names; provisioning itself, queue fairness and
leadership-loss recovery run black-box in `internal/control`.
The agreed direction is that each layer gets its own tests and its own fixtures;
do not add a shared test-environment package.

`make mcp-test` and the backend half of `make integration` require the existing
local development config/server. `make mcp-test` runs only the `TestAgent*` tests
of `internal/agent` and `internal/control` with race detection; the general security suite (`LASTAPP_SECURITY_CONFIG`)
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
`main`; force-pushes and deletion are blocked. Merges go through the merge
queue: add a green pull request to the queue (`gh pr merge <n> --merge` or the
"Merge when ready" button) and GitHub tests it on top of the current `main` (the
`merge_group` trigger in `checks.yml`) before merging, so there is no manual
"Update branch" step.

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
