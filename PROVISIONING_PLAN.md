# Lastapp Superadmin Provisioning Plan

**Historical first-phase specification — 12 November 2026.**

The implemented [identity/roles/scopes foundation](FOUNDATION.md) supersedes this phase’s Tenant custom-role model with fixed Tenant/team roles and scoped Space/App business roles. The [entity runtime](ENTITY_RUNTIME.md) and [management MCP](MCP.md) are also implemented. This document preserves the earlier provisioning specification; use [DEVELOPMENT.md](DEVELOPMENT.md), FOUNDATION.md and README.md for current behavior. The linked SQL document now derives from all current migrations, not only this phase's original tables.

Build one Go application containing the management API, a provisioning worker and a small React administration console. One central MariaDB database records Tenants, Spaces, their databases, Apps and micro-app installations, together with shared identities and authorization metadata. Tenant business data stays in the databases bound to Spaces.

An implemented local first slice is available; see [README.md](README.md) for startup and review instructions and [VERIFICATION.md](VERIFICATION.md) for executed checks.

This is the implementation specification for the first phase. The exact database definition is [control_plane.sql](design/control_plane.sql). The broader framework direction remains in [CONCEPT.md](CONCEPT.md).

## Architecture and scope

```text
Go control plane: management API + provisioning worker + embedded console
└─ Central MariaDB registry
   ├─ Global users, sessions, permission catalog and built-in roles
   └─ Tenant
      ├─ Memberships, teams, custom roles and scoped grants
      ├─ Tenant-owned database resources
      └─ Space → one database resource
         └─ App → backend API/deployment unit
            └─ Client micro-app installations
```

An App inherits its Space's database binding. Its future runtime may share containers or use dedicated containers; that deployment engine comes later. This phase creates and manages inventory and database connections. It does not execute business-object definitions, build client bundles or start App containers.

Identity and authorization are platform capabilities shared by every Tenant and App. A Tenant owns its memberships, teams, custom roles and grants as rows in shared tables. Provisioning a Tenant or Space never creates another set of users/roles/permissions tables in its business database. The same Go authorization component will enforce permissions in the control plane and future App runtimes.

## Simplifications from the earlier draft

| Earlier design | Final approach |
|---|---|
| Three overlapping design documents | This plan is authoritative; SQL defines exact storage |
| Separate provisioning-operation table and database status | Database-resource rows are the durable work queue and current status |
| Parallel workers, leases, epochs and automatic retry scheduling | One active worker, restart recovery and explicit retry |
| Generic secrets registry and secret references | Encrypted credentials stored on their owning target/resource; one encryption helper |
| Idempotency table, request digests, response cache and expiry | Client-generated create IDs, unique constraints and conditional updates |
| Versioned plans and compatibility sweeps | Ordinary plan records; changes govern new requests and preserve existing resources |
| App configuration versions, activation and rollback | App inventory now; configuration publishing with the business-object runtime later |
| Replica ranges, runner settings and duplicate artifact metadata | Placement preference and an optional pinned artifact reference |
| Separately stored Space namespace | Derive it from the immutable Space ID |
| Separate superadmin flag, membership role and future ACL implementation | Protected built-in roles and Tenant custom roles use one catalog, grant model and authorization component |

Tenant ownership constraints, authentication, encryption, audit history and recovery from partially created databases remain required.

## Database design

Use 17 shared tables, plus the migration tool's history table when implemented. Six authorization tables extend the original provisioning inventory; their rows grow with Tenants, rather than creating tables per Tenant:

| Table | Stored information |
|---|---|
| `users` | Global internal ID, unique OIDC issuer/subject, name/email and enabled state; privileges come from grants |
| `sessions` | Hashed random session token, user, CSRF-token hash and expiry; delete to revoke |
| `tenant_memberships` | Tenant, user and active/disabled membership; membership alone grants no administrative capabilities |
| `groups` | Tenant-owned teams, names and enabled state |
| `group_memberships` | Users belonging to teams in the same Tenant; users can join multiple teams |
| `permissions` | Protected catalog of supported action codes, classified as platform or Tenant actions |
| `roles` | Shared protected built-in roles and Tenant-owned custom roles; each has a platform or Tenant assignment kind |
| `role_permissions` | Explicit permissions bundled into each role |
| `access_grants` | One user/team receives either a role or a direct permission, within platform, Tenant, Space or App scope |
| `plans` | Allowed database layouts/methods, default layout, dedicated-compute permission and optional resource limits |
| `tenants` | Company identity, state, plan, optional default layout and default shared-database reference |
| `database_targets` | Canonical server endpoint, TLS setting, availability, managed-creation permission and encrypted provisioning credentials |
| `database_resources` | Tenant, target, physical name, managed/registered mode, shared/dedicated layout, encrypted runtime credentials, ownership marker, status, attempts and last error |
| `spaces` | Tenant, name, state and one database binding |
| `apps` | Tenant, Space, name, state and desired `shared`/`dedicated` compute mode |
| `microapp_installations` | Tenant, App, name, state, navigation order and optional pinned artifact reference |
| `audit_events` | Actor, Tenant, action, resource, request ID, timestamp and small redacted details |

Use UUIDs, UTC timestamps, explicit foreign keys and scoped unique slugs. Names remain reserved after archival. Named resources carry a `revision` for conditional updates; membership and role-permission joins use composite keys and are changed under their owning Tenant/group/role transaction. SQL contains no stored business logic or triggers.

Spaces, Apps and installations retain `tenant_id` and composite parent foreign keys. Authorization relationships also enforce matching Tenants: a team member must belong to that Tenant, scoped resources and principals must match, and a custom role cannot be assigned outside its owner Tenant. Platform actions cannot enter a Tenant role or grant. Shared built-in Tenant roles can be assigned within any Tenant. These constraints do not replace API authorization.

### Database bindings

Each resource has two independent choices:

| | Managed creation | Existing database |
|---|---|---|
| Shared within one Tenant | Lastapp creates it; several Spaces may bind | Lastapp registers it; several Spaces may bind |
| Dedicated to one Space | Lastapp creates it; at most one Space may bind | Lastapp registers it; at most one Space may bind |

Composite foreign keys enforce ownership and layout consistency. A conditional unique index enforces dedicated exclusivity. The Tenant's default shared reference must point to its own shared resource. These small constraints prevent integrity bugs without requiring a generic rules engine.

Creating a Tenant does not create a database immediately. The first managed shared-Space request reserves a database and records the Tenant's default shared reference in the same transaction. Subsequent requests reuse it, including while provisioning is pending. Explicit resource selection supports additional shared databases. Dedicated requests reserve an exclusive resource.

The Space ID provides its stable namespace identity. Actual table names and entity storage are defined with the business-object layer. Apps have no second database pointer.

Canonical endpoint/database pairs are unique, conservatively ignoring case in database names. Normalize known endpoint aliases; arbitrary DNS aliases cannot be proven distinct by SQL. A referenced target's endpoint and a Space's database binding remain fixed. Database moves require a later migration feature.

### Plans and lifecycle

Keep explicit fields for permitted shared/dedicated databases, managed/registered methods, default layout, dedicated compute, and optional limits on Spaces, Apps and databases. `NULL` means unlimited; zero forbids additional resources. A Tenant may select either layout when permitted and may mix layouts.

Count non-archived Spaces/Apps and all retained database-resource records, including pending, failed and retired reservations. Retirement preserves physical resources and does not release database quota. Reclamation is an explicit operator operation outside tenant self-service. Serialize creation on the Tenant row before checking usage. Reopening archived resources checks quotas again.

Plan edits and assignment changes affect new allocations and requested capability changes. Existing resources and already accepted provisioning requests continue; there is no automatic migration, deletion or downgrade sweep. If usage exceeds a new limit, additional allocations are blocked. An invalidated Tenant layout default falls back to the plan default. Existing resource retry does not consume another quota slot.

Tenant, Space, App and installation states are `active`, `suspended` and `archived`. Effective access follows the parent chain. Parent changes do not rewrite child records. Tenant suspension blocks ordinary Tenant mutations and new provisioning attempts; an attempt already running may finish. Its resource remains inaccessible through the suspended Tenant.

Database states are `pending → provisioning → ready` or `failed`; retry returns a failed resource to `pending`. `retired` is allowed only when unbound and not the Tenant default. Archive preserves data. Physical database/account deletion is outside this phase.

## Management API and Go structure

Use standard-library `net/http`, `database/sql`, explicit SQL, embedded Goose migrations, OIDC through `coreos/go-oidc/v3` and `golang.org/x/oauth2`, and a React/Tailwind console served by the same binary. Use the dedicated `github.com/aantich/go-mariadb` driver; the implementation pins a hashed source snapshot from the sibling driver checkout. No ORM, generic repository hierarchy or plugin framework is needed.

```text
cmd/lastapp/             serve, migrate, bootstrap-admin
internal/auth/          OIDC, sessions, shared authorization component
internal/control/       management service methods and policy checks
internal/store/         SQL, transactions, migrations
internal/provision/     worker and MariaDB provisioning/verification
internal/httpapi/       routes, DTOs, error mapping
web/admin/              React console
```

Small configuration and encryption helpers can live beside their callers; split packages when reuse warrants it. Share service functions between HTTP routes and provisioning actions. Keep subscription/quotas in one policy helper and authorization in one shared component. Avoid a generic entity-management engine for these administrative tables.

| API group | Operations |
|---|---|
| `/auth/*`, `/v1/me` | Login, callback, logout, identity and accessible Tenants |
| `/admin/v1/*` | Users for assignment, plans, Tenants, platform grants and managed targets |
| `/v1/permissions` | Read the supported action catalog; permission definitions are platform-maintained |
| `/v1/tenants/{tenant}/memberships` | Read/manage registered users' Tenant memberships with the required permission |
| `/v1/tenants/{tenant}/groups` | Teams and their memberships |
| `/v1/tenants/{tenant}/roles` | Read built-ins; manage custom roles and their permission bundles |
| `/v1/tenants/{tenant}/access-grants` | Assign/revoke roles or direct permissions for users/teams at a supported scope |
| `/v1/tenants/{tenant}/database-resources` | Create/register, list, inspect, correct credentials, retry/verify and retire |
| `/v1/tenants/{tenant}/spaces` | Create, list and update metadata/state |
| `/v1/tenants/{tenant}/spaces/{space}/apps` | Create, list and update App inventory |
| `/v1/tenants/{tenant}/apps/{app}/microapps` | Create, list and update installations |
| `/v1/tenants/{tenant}/audit-events` | Inspect Tenant history |

Database-resource status is the provisioning-status endpoint. There is no separate operations API. Space creation accepts either an existing resource ID or an inline managed-database request; both paths reuse the same resource service functions. Existing database registration can precede Space creation.

Clients generate a UUID once when creating an ID-bearing resource and keep it across retries. A duplicate ID returns `409` without creating anything; the console reads the existing authorized resource to reconcile a lost response. This guarantees no duplicate allocation for the same create ID without promising replay of an old response. Updates to versioned records use `PATCH` with `If-Match: <revision>`; stale revisions return `412`. Membership joins use their composite keys; editing a team's membership or a role's permission bundle checks/increments the parent revision.

Tenant-scoped mutations run in short transactions: lock the Tenant, authorize against current membership/grants, check policy/parents/revision, write records and append audit. Read the applicable plan without an exclusive plan-row lock while evaluating a new allocation. Authorization changes take that same Tenant lock, so revocation and mutation have a defined order. Use READ COMMITTED and a consistent Tenant-before-resource order; shared plan updates lock only the plan. Platform grant changes serialize on the protected superadmin role. Revocation affects subsequent authorization checks; it does not cancel already authorized external work. Never make target network calls while holding registry transaction locks. Scope every lookup to the authorized Tenant; return not-found for out-of-scope IDs.

## Provisioning and recovery

Start with one active worker processing one resource at a time. It holds a database-specific `GET_LOCK` on a pinned registry connection for its lifetime. Additional control-plane processes can serve HTTP, but cannot provision concurrently. All worker status writes use that connection. Losing it ends the attempt and closes its target connections; a new worker must acquire the lock before recovering work. Use one writer endpoint for the registry and each managed target. MariaDB advisory locks survive transaction commits and release when their connection ends. [MariaDB advisory locks](https://mariadb.com/docs/server/reference/sql-functions/secondary-functions/miscellaneous-functions/get_lock)

Release advisory locks explicitly before returning pinned connections to a pool; discard the physical connection if release cannot be confirmed. After acquiring leadership, return abandoned `provisioning` rows to `pending`. Process eligible pending resources for active Tenants and targets. Mark one `provisioning`, increment attempts, perform the external work, then record `ready` or a redacted `failed` result with audit. Failures wait for explicit retry. A restart reconciles the same resource instead of allocating another.

### Managed database

Persist generated database/account names, runtime password and ownership token before external work. Use a pinned target connection with a resource-specific advisory lock, so an old disconnected worker's in-flight DDL cannot overlap recovery on that resource. Stop on connection loss and never reconnect halfway through an attempt.

Create the database with its resource/token ownership COMMENT. On recovery, require that marker before reuse. Create the restricted runtime account and database-scoped CRUD grants; if the account already exists, verify the stored credential and compatible grants before completing it. Never overwrite an unrelated database or reset a colliding account. Verify runtime access before marking ready. Bound all target connections and queries with timeouts.

This reconciliation is necessary because database/account/grant DDL can implicitly commit independently of registry changes. It remains a small MariaDB provisioning function, with no saved workflow program or step engine. [MariaDB implicit commits](https://mariadb.com/docs/server/reference/sql-statements/transactions/sql-statements-that-cause-an-implicit-commit)

### Existing database

Connect using submitted runtime credentials and verified TLS; inspect database selection, supported server version and effective required CRUD grants. A read probe alone does not establish write permission. Do not change business tables or accounts. Apply the deployment's endpoint/network policy and reject known control-plane database connections. Disabling TLS verification requires explicit development configuration.

Correcting registered credentials re-encrypts them and puts the resource back into `pending`, using the same verification path. Reject edits/retries while `provisioning`. Managed runtime credentials remain stable across retries. Correct provisioning-target credentials through the platform-only target API. Seamless credential rotation and managed runtime password changes come later.

One worker is appropriate for infrequent administrative DDL. Add parallel provisioning only when observed queue delays justify it. This limit does not constrain the later App runtime pools.

## Shared identity and authorization

Configure trusted OIDC providers, code flow with PKCE/state/nonce, and library token validation. Map exact issuer/subject pairs to global users; email never grants access. Use random session cookies with hashed tokens in `sessions`, Secure/HttpOnly/SameSite attributes, expiry and CSRF/origin checks. Authentication establishes the user; grants establish authority.

### Roles, teams and grants

Seed two protected role definitions once: platform `superadmin` and `tenant_admin`. The latter is a shared role definition assigned separately within each Tenant. The operator CLI bootstraps a user by creating a platform-scoped superadmin grant. Creating a Tenant atomically records its initial membership and Tenant-scoped administrator grant. No `users.platform_admin` flag or `tenant_memberships.role` column remains.

A custom role belongs to one Tenant and bundles supported Tenant permissions, such as `space.read` and `app.manage`. It can be reused across that Tenant's users/teams and scopes. Tenant administrators can edit custom roles with `authorization.manage`; they cannot change the action catalog, protected built-in definitions or platform grants. Membership administration requires `membership.manage`.

Teams are flat groups inside a Tenant; a user can belong to several. Membership is required before a user can join a team or receive a Tenant grant. Disabling a user, Tenant membership, team or custom role makes its affected grants ineffective without deleting their history.

One `access_grants` row assigns exactly one role bundle or direct permission to exactly one user or team. A Tenant grant applies to the whole Tenant, one Space and its descendants, or one App and its micro-apps. Scope follows stored resource ancestry and never widens to siblings or parents. A platform grant has no Tenant/Space/App binding and initially targets a user only. Logical duplicates are rejected by the service within the scope's mutation lock.

For example, Tenant Acme can define a `reviewer` role containing `app.read`, then assign it to the Sales team within the Sales Space. The same role can be assigned to another team in a different Space. Another Tenant's reviewer role is a different row, and its users, teams and scopes cannot be mixed into Acme's grant.

### Shared enforcement

Use one backend authorization component with an explicit user, action and server-resolved resource context. It checks current user/session state, Tenant membership and lifecycle, then applicable direct and team grants and their role permissions. No matching grant means deny. Initial grants are additive allows; there is no deny-precedence language, nested-team traversal or arbitrary policy code in this phase.

Management endpoints request concrete permissions, such as `space.manage`. Platform superadmins can administer the control plane across Tenants through the same component's explicit platform permission. That permission is not an implicit right to read all future business records. Protect the last active platform administrator and each Tenant's last direct Tenant-wide administrator assignment against accidental revocation.

Lists, counts and pagination must respect allowed scopes in their queries, as well as individual-object checks. Client visibility does not establish authority. Initial control-plane requests read current registry authorization data; add no policy cache or separate authorization server. Future App runtimes reuse this Go component. Distribution/caching must define revocation behavior before it is enabled.

### Business entity authorization boundary

This phase delivers centralized teams, roles, permissions and Tenant/Space/App grants and uses them for provisioning APIs. Entity-, field- and record-level ACLs extend the same authorization model with the business-object layer. Its query compiler must apply permission predicates before filtering, counting, sorting and pagination, and authorize writes and bulk items consistently.

Users, memberships, teams, role definitions and the policy catalog remain platform-managed. Decide placement of potentially large record-grant datasets alongside entity storage and query design; those grants may live with business records while still using the shared authorization implementation. Do not create an independent permission engine or duplicate identity catalogs inside Space databases. Record ACL storage, advanced rule semantics and caching are not implemented by this provisioning schema.

## Credential handling

Store credentials as encrypted envelopes directly on their owner rows. One AES-GCM helper handles both, with key ID, random nonce and authenticated record type/ID plus Tenant ID where applicable. Keys remain in deployment configuration. Use explicit response DTOs; never expose credentials in responses, errors or audit details. No general vault abstraction is needed. Keep migration, control-plane and target-provisioning database privileges separate; the service can only append/read audit history.

## Implementation sequence and completion criteria

| Step | Deliverable | Required verification |
|---|---|---|
| 1. Foundation and access | Go binary, SQL migrations and authorization seeds, config/encryption helpers, OIDC/sessions, grant-based bootstrap, Tenant/plan APIs, memberships/teams/custom roles/grants | Clean installation; only superadmins create Tenants; session/CSRF checks; direct/team/scope grants and revocation; built-in protection and cross-Tenant denials |
| 2. Databases and Spaces | Targets/resources, both layouts/methods, quota checks, one worker, retry/recovery | Shared reuse under concurrent requests; dedicated exclusivity; ownership conflicts; crash recovery; existing data/accounts unchanged |
| 3. App inventory | Apps and micro-app installations with lifecycle and optional artifact references | Correct parent ownership; duplicate create IDs; stale revisions; inherited suspension |
| 4. Console and release | Tenant tree, memberships/team/role/grant forms, status/retry, audit, binary/container packaging and setup guide | Full flow in the console; no platform privilege escalation; redacted diagnostics; backup/restore of registry plus separately held keys |

Use real MariaDB integration tests for constraints, quota races and provisioning boundaries. Kill a worker after database creation, account creation, grants and before final status; verify recovery and worker-lock loss. Test that plan edits preserve existing/queued resources while blocking disallowed new allocations. Reject retirement of bound/default resources. Keep authorization tests at the HTTP boundary and encryption/session tests around their shared helpers.

Authorization cases must include cross-Tenant users/teams/roles/resources, role-versus-direct grants, App/Space scope limits, disabled memberships/teams/roles, list/count filtering, built-in role edits, Tenant attempts to assign platform actions, last-admin protection and membership/grant revocation racing with a mutation. SQL checks verify structural relationships; they do not prove Go authorization behavior.

Run migrations explicitly as a deployment task; replicas only check schema compatibility. MariaDB DDL can partially commit, so document migration recovery. The [validation report](design/VALIDATION.md) records the current SQL checks. MariaDB 10.11 is the proposed minimum and needs CI execution before support is declared; current local validation uses 12.3.3.

Business-object execution/schema migration, App configuration publishing, artifact builds and browser runners, connector OAuth, compute scheduling, billing and physical purge follow later. The initial release remains a small, self-hostable provisioning application.


### Security boundaries for the next phase

The control-plane authorizer accepts only the fixed management action catalog. Unknown domains fail closed, including for superadmins. Business APIs must have a separate entry point that requires Tenant membership, resolves Space/App ownership on the server and enforces active Tenant/Space/App states. Reuse grant evaluation primitives; do not reuse the control-plane administrator bypass or Tenant-wide mutation lock for business-record writes.

The admin origin serves trusted console code only. Generated apps and npm dependencies execute on separate hosts in isolated contexts with narrowly scoped, App-bound authority. `app_origin`, when configured, must use a different hostname from `public_url`; a different port on the same host does not isolate cookies. Production admin cookies use the `__Host-` prefix. App execution and its credential broker remain part of the later runtime phase.

A Space owns the shared physical data schema, migrations and invariants. Apps refer to those definitions and expose selected APIs/views; separate App definitions cannot independently rewrite shared tables or weaken Space rules. The existing registry introduces no entity compiler or automatic business DDL.

Operators register one canonical target per physical database server. DNS aliases must reuse that target ID; host strings are not proof of distinct servers. Current targets form the shared provisioning pool. Private/premium targets require explicit placement entitlements before they are offered; they are not implied by the current shared-pool API.

Plan policy is read once inside each allocation transaction under READ COMMITTED. The Tenant row protects usage and quotas; shared plan rows are not exclusively locked. Metadata ceilings bound Tenant members (1,000), groups (200), group memberships (10,000), custom roles (200), grants (2,000) and micro-app registrations (1,000). At most eight database requests per Tenant may await provisioning. The local request limiter allows a 120-request burst and 20 requests/second per user and Tenant, per process. The single provisioning worker rotates between waiting Tenants and removes expired sessions in bounded batches; these are control-plane safeguards, not a claim of distributed runtime capacity.
