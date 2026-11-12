# Lastapp Space and Authorization Design

**Foundation approved and implemented — 12 November 2026. Entity/ACL runtime and App SDK deferred.**

The implemented scope and exact management API are documented in [FOUNDATION.md](FOUNDATION.md). Following the discussion, this delivery covers global identities, memberships, teams, fixed administrative roles, scoped Space/App business-role definitions/assignments and the console. The entity policy language, authorized query compiler, record ACLs, App credentials/runner and SDK below remain later-stage design direction, to be finalized with their developer API before implementation.

Make the Space the reusable business backend. A Space owns its database binding, entity definitions, invariants, business permissions and shared roles. Apps normally inherit this backend and assemble browser experiences from it. App-specific permissions, roles and entities are optional extensions governed by the Space.

Identity and authorization remain shared platform capabilities. One registry, one set of identity and role tables, one policy representation and one Go enforcement library serve all Tenants and Apps. Tenant and team roles come from a fixed platform catalog. Custom business roles belong to a Space or App. Each role assignment has one scope; there is no arbitrary scope-combination language.

The broader direction includes a generic MariaDB entity backend and console/SDK support. The approved foundation corrects the existing roles/scopes and preserves provisioning, security corrections and UI work; it does not implement that entity runtime yet.

Lastapp is in local development and has no deployed installations. Implement the corrected model directly; there are no production grants to convert, legacy clients to support or deployment cutovers to coordinate.

The ordinary path is small: create a Space, publish its entities and permissions, assign roles, and let Apps inherit them. Sharing, App roles and private entities are optional. The [review findings](review/authorization-plan-2026-11-12/REVIEW.md) explain the corrections made before this proposal. The delivered provisioning plan remains historical; this proposal governs the next implementation phase if approved.

## 1 Ownership and inheritance

```text
Platform
└── Tenant
    ├── Users through Tenant memberships
    ├── Teams and team role assignments
    └── Space
        ├── One database binding
        ├── Business entities and shared invariants
        ├── Business permission definitions and Space roles
        └── App
            ├── Inherited Space APIs and authorization
            ├── Optional additional access restrictions
            ├── Optional Space-authorized extensions
            └── Micro-apps and browser business logic
```

Teams are Tenant-level structures, alongside Spaces. A user can belong to several teams. Entities may declare an owning-team relationship, which policies use to evaluate team roles. A team is neither a database partition nor a mandatory parent of a Space.

| Owner | Defines | Default behavior |
| --- | --- | --- |
| Platform | Identity model, fixed administrative/team roles, supported operations and policy grammar | All Tenants use the same implementation |
| Tenant | Memberships, teams, Space inventory and administrative assignments | Tenant administration carries no automatic business-record access |
| Space | Database binding, entities, invariants, permissions, shared business roles and App ceilings | Apps reuse these definitions without copying them |
| App | Client composition, presentation, integration bindings and optional restrictions/extensions | Inherit the Space backend; create no parallel entity or role catalog |
| Micro-app | Pages, components, actions and browser flows | Uses its installed App’s authenticated API context |

An App remains a logical serving/deployment unit. Several Apps can share runtime containers, or one App can use dedicated replicas. Every runtime loads the same Space definitions and enforcement library. Physical deployment does not confer permission to redefine shared tables or rules. Tenant/Space/App ancestry and the Space database binding are immutable in this release; moving them requires a future migration operation, never a generic PATCH.

A Space role has the same meaning across its Apps. A micro-app has no separate data role or credential scope in this delivery; its installed App is the authorization boundary. All code within an App must therefore be trusted with that App’s capabilities. Unreviewed third-party code makes the containing App untrusted; finer isolation between its micro-apps is a later runner feature. Installing an App or library template never copies assignments or credentials from another Tenant or Space. Newly published entities and fields do not automatically enter existing permission allowlists.

## 2 Roles and assignments

### Role categories

| Category | Definitions | Who receives assignments | One assignment targets |
| --- | --- | --- | --- |
| Platform | Fixed and protected | User | Platform |
| Tenant | Fixed and protected | Active Tenant member | Tenant |
| Team | Fixed and protected | Active member of that team | Team |
| Space | Custom business roles; protected management role | Tenant user; teams may receive business roles only | Space |
| App | Optional custom business roles; protected management role | Tenant user; teams may receive business roles only | App |

Space/App management roles provide the authority to author definitions and assign roles. Custom business roles bundle data permissions. Keeping these purposes explicit prevents a generated “Editor” role from acquiring identity administration or schema-management authority.

Initial protected catalog:

| Role | Authority |
| --- | --- |
| `platform.superadmin` | Provision Tenants and infrastructure; manage platform control-plane access |
| `tenant.admin` | Manage that Tenant’s inventory, memberships and teams; assign Tenant/team roles and Space/App managers |
| `team.admin` | Manage members and fixed role assignments within that team |
| `team.reviewer` | Read permitted team metadata and act as the stable Reviewer relationship used by business policies |
| `space.manager` | Manage that Space’s definitions, permissions, business roles, assignments and App ceilings |
| `app.manager` | Manage that App’s client configuration, installations and permitted advanced definitions |

Membership is a relationship, not another duplicate “member” role. Ordinary membership alone grants no business CRUD. Tenant profile management exposes only permitted profile/default fields; plan changes, target administration and platform grants retain platform-only authority. Tenant/team role definitions can change only through platform releases. Space/App Viewer, Editor and Reviewer templates instantiate editable custom roles; changing a template does not silently change existing roles.

Managers do not implicitly read business records. A manager with authority to change policy can explicitly grant data access, and that change is audited. This is a governance power, not a promise that an administrator who can rewrite policy can never obtain data access.

### Definition ownership and assignment scope

Separate the definition from its assignment:

```text
Definition: fixed team.reviewer
Assignment: Joe → team.reviewer → Marketing

Definition: CRM Space’s Campaign Editor
Assignment: Joe → Campaign Editor → CRM Space

Definition: Forecast App’s Read Only Analyst
Assignment: Finance team → Read Only Analyst → Forecast App
```

The third assignment gives the App role to active Finance members. It is different from assigning Joe a role within Finance. Team-scoped roles initially accept individual users only, avoiding recursive “team holds a role on itself” semantics.

Each assignment references one role and one scope. A Space-owned custom role can be assigned only at its owning Space; an App-owned role only at its owning App. Protected definitions can be assigned at matching scope categories. Space/App creation names an active Tenant member as its initial manager; the console may visibly default to the creator only when eligible. A platform administrator outside the Tenant must choose a member. Create the manager assignment and audit atomically; assign no business data role. All non-platform assignments require a Tenant validated through stored ancestry.

Space assignments participate in requests through all that Space’s Apps. App assignments participate only within their App. Team role assignments can participate in any Space policy that explicitly references that team relationship. Tenant/team administration does not imply a Space business role.

Several applicable grants provide additive access. There are no explicit deny grants, nested roles, nested teams or arbitrary combinations of assignment scopes in this delivery. Mandatory guards and App ceilings always intersect with grants and cannot be overridden by an additional allow.

### Assignment and delegation authority

The protected assignment authority is explicit:

| Caller | May assign or revoke |
| --- | --- |
| Platform superadmin | Platform roles; bootstrap/recover Tenant administrators |
| Tenant administrator | Tenant roles, team roles and Space/App managers in that Tenant |
| Team administrator | Fixed team roles to members of that team; membership changes select existing active Tenant members |
| Space manager | Space business roles and App managers within that Space |
| App manager | App business roles within that App |

Only users receive protected administrative roles. Allowing a team to hold a manager role would let its membership editor acquire policy-administration powers indirectly. Teams may receive custom data roles; editing their membership intentionally changes that data access. A team administrator can likewise assign Reviewer within their team, but cannot edit the Space policy that determines Reviewer’s business authority. Space/App managers cannot appoint other managers at their own level unless the table above authorizes it.

Every assignment service validates active recipient membership, resource ownership, role ownership, category and the caller’s assignment authority. Client-supplied IDs or a role’s display name never establish authority. Protect the last active Tenant administrator and platform superadmin, including changes to user state or membership that would deactivate their authority. A Tenant administrator can recover a team or Space with no manager; there is no additional mandatory last-manager lock at every level.

Check last-administrator invariants inside the same short mutation transaction, serialized by the affected administrative scope lock. Changes affecting several Tenants acquire those locks in deterministic order. Two concurrent removals must not each observe the other administrator and both succeed. This serialization belongs to control mutations, never ordinary data requests.

Disabling a user, Tenant membership, team, team membership or role removes the affected access on subsequent authorization snapshots. Disable/re-enable preserves assignments. Tenant membership remains disable-only in this release; hard removal would also require a policy for outstanding shares across business databases. Physical team-membership removal revokes dependent team-role assignments in the same registry transaction before removing the membership row. Rejoining that team does not restore a removed team-role assignment, although roles/shares assigned to the team apply to its current members. Definitions and historical audit events remain.

## 3 Business permissions and ACL semantics

### One reusable permission definition

Extend the central permission catalog to distinguish protected control permissions from Space/App business permission definitions. A business permission is a named, versioned rule for one entity and one operation, with an explicit field allowlist and an optional record predicate. Roles reference permission IDs through the existing join table; they do not copy policy JSON.

The first operation catalog is `create`, `read`, `update`, `delete` and `acl.manage`. A named permission such as `campaign.review` maps to one of these operations, for example updating the Campaign status field under a Reviewer predicate. It is not a new executable backend command. Multi-step workflows remain browser code; entity invariants still govern every write.

A Space permission has one of two activation modes:

- **Role permission:** active through an applicable Space role assignment, or an explicitly permitted record share.
- **Space policy:** active directly when its declared actor/record predicate matches. This supports team-role and ownership policies without duplicating Space role assignments for every user.

Both modes compile into the same authorization plan. Only Space managers publish Space policies. App-owned permissions are role permissions referencing an existing Space permission and adding constraints; they cannot become Space-wide policies.

Space-role joins accept role-mode permissions only; assigning a role never changes a Space policy’s activation mode. App-role gates may reference either kind of Space permission, but still require that permission’s own current Space activation. Entity lifecycle, fixed context guards and schema invariants remain separate from grant activation.

Role/permission edits validate against the active entity version and take effect atomically at registry commit. Authors can stage disabled definitions, then enable them. Only physical entity-schema publication uses the asynchronous queue; there is no second policy-deployment engine.

### Joe and the Marketing team

The Campaign entity declares that `owning_team` references a Tenant team. The Space defines this policy once:

```json
{
  "code": "campaign.review",
  "activation": "space_policy",
  "entity": "campaign",
  "operation": "update",
  "fields": ["status", "review_note"],
  "where": {
    "has_team_role": {
      "role": "team.reviewer",
      "team_from": "record.owning_team"
    }
  }
}
```

Names here are author-facing shorthand. Publication resolves entity, field and role references to stable IDs. A corresponding `campaign.read` Space policy uses the same team-role predicate and an explicit readable field set; review/update permission does not imply read permission. Because Joe is Reviewer for Marketing, the review permission applies to Marketing-owned Campaigns through every inheriting App. It gives him no review rights over another team’s Campaigns. Joe does not need another copy of Reviewer assigned separately in each App.

### Mandatory guards and App restrictions

For any entity operation, all mandatory gates apply, with parentheses fixed explicitly:

```text
allow = active identity and Tenant membership
    AND active Tenant, Space, App and entity
    AND ready database resource, correct ownership and authenticated App binding
    AND mandatory Space/entity access guards
    AND (applicable Space permission OR authorized record share)
    AND Space-imposed App ceiling
    AND optional App restriction/custom-role gate
```

Invariants are additional checks on the resulting data; permission alone does not make an invalid write valid. A read permission never implies write or sharing permission.

Keep the same base permission throughout an authorization path. For a Space permission `P`, a path consists of its current activation (Space role, Space policy or eligible record share), its predicate/fields, the App ceiling entry for `P`, and any App gate referencing `P`. An App gate for `P` cannot be combined with a different Space permission `Q`. Disabling `P`, removing its activation or losing its required team relationship breaks that path even if another permission allows the same operation.

Default App mode is `inherit`: existing Space permissions apply without App roles. Advanced mode is `custom_roles`: an applicable App-role permission for that same base path is also required. No matching App role means deny. An App role can directly reference `P` or a single App-owned narrowing definition based on `P`; recursive permission chains are unsupported. A narrowing definition keeps the entity/operation, intersects fields and conjoins its predicate. Runtime intersection enforces the limit; the system does not attempt to prove arbitrary logical implication between policies.

Space managers own the App ceiling. A restricted ceiling explicitly lists base permission IDs, allowed operations/fields and optional predicates; absent entries deny, and it contains no wildcard for future entities or fields. It is intersected with the caller’s current Space rights, not a grant in its own right. A trusted App can instead have an explicitly approved inherit ceiling. App managers may edit their own optional restriction or return their restriction to inheritance, but cannot remove a Space ceiling or change trust classification. If a restriction must survive an App manager’s edits, the Space manager puts it in the ceiling.

Every new App uses inherit mode but has no active browser runtime until a Space manager approves its origin and trust/ceiling settings. External/untrusted Apps require a restricted ceiling; Tenant authorship alone does not establish trust. Approval can be part of ordinary App creation by an authorized Space manager, avoiding a separate workflow engine.

App-specific entities are Space-registered entities with an owning App namespace. The Space authorizes their publication and base access; their mandatory visibility guard also checks the authenticated App. They do not create independent App DDL authorities or let App roles broaden access to shared Space entities. Advanced authoring can create drafts; publication requires explicit Space schema authority.

### Fields and combinations of grants

Preserve the relationship between each rule’s fields and record predicate. Do not union all allowed fields separately from all allowed records: “salary on my record” plus “names on everyone’s records” must not disclose everyone’s salary.

For reads, each requested field must be allowed on the row by at least one complete permission path. Different read paths may supply different fields, but the fields never detach from their predicates or App gates. Filter and sort fields participate in the same check. Asking for a restricted field can narrow the result set. Every read permission includes the record ID. Counts use the identical query/projection authorization predicate without pagination. Entity metadata supplies a small default projection, always including the ID; there is no implicit per-row masking. Target rows of a relation require their own read authorization.

For an update, one complete permission path must authorize the entire submitted field set and both old and new record values. Do not assemble a write from unrelated rules, or use one path before and another after the change. Create checks all client fields and the new values after server defaults; delete checks existing values. Server-owned Tenant/Space IDs, revisions and audit timestamps are never writable client fields. Delete and `acl.manage` have predicates but no writable-field wildcard; `acl.manage` uses an explicit delegable-permission list.

Successful writes return only submitted record IDs and new revisions by default, never a full entity. Returning fields is a separate authorized read, including on creates and bulk writes. Capabilities and metadata describe applicable caller permissions, not an unrestricted schema/policy dump; row-specific capabilities require an authorized lookup and never expose hidden predicate values.

Ownership/team fields are declared explicitly in entity metadata. Reassigning them requires `acl.manage` as well as update permission, and clears shares in the same transaction. Create permission and its new-record predicate control initial ownership; creating a record does not confer an owner role automatically.

### Record-specific sharing

Most access should derive from roles, team relationships and predicates. Do not create ACL rows for every record merely to reproduce inheritance.

For exceptions, use one indexed record-grant table in each physical business database. A row references Tenant, Space, entity, record, recipient user XOR team, and a Space permission ID **and revision**. A share is active only while that exact permission revision is active. Any permission edit makes old shares inert until explicitly renewed; it never silently expands existing shares. Ordinary role references follow live permission edits. All current Space guards, recipient membership, predicates and App ceilings still apply.

`acl.manage` lists which Space permissions the caller may delegate. Only same-entity, role-mode Space permissions for read/update/delete are shareable; never create, control actions or recursive `acl.manage`. The caller must possess that delegated base permission’s operation/fields on the record, in addition to delegation authority, through complete paths and the current App ceiling. The ceiling also limits which permission IDs may be delegated. A recipient must be an active Tenant member or active same-Tenant team. Clients cannot supply predicates/fields; the permission predicate is reevaluated for the recipient, including both states for updates.

Sharing is off until enabled by the Space. Grant creation/revocation locks the parent record and uses the same transaction as local audit. Listing/revoking shares requires `acl.manage`; a read-only recipient does not receive the recipient roster. Deleting a record deletes its shares transactionally; recreating an ID never restores old shares. Owning-team/user transfer also clears shares. An explicit share persists if its grantor later loses their role; revoke the share or disable/revise its permission to remove it. This is an explicit delegation model, not an inherited grantor relationship graph. Periodically clean up inert grants under a bounded retention policy; retained audit is separate.

## 4 Storage changes

### Global registry

Keep one registry shared by all Tenants. Scope ownership is expressed through rows and constraints, never by creating another users/roles database inside each Space.

| Table | Changes and purpose |
| --- | --- |
| `users`, `tenant_memberships` | Preserve identity and disable-only Tenant membership semantics, including exact OIDC identifiers |
| `groups`, `group_memberships` | Preserve physical names; expose them as teams in the API. Add enabled/disabled membership state and enforce it for team-scoped roles |
| `roles` | Add explicit `scope_kind` and `domain`; retain protected built-ins and add Space/App ownership columns for custom business roles |
| `permissions` | Give definitions stable UUID IDs and owner-scoped codes; distinguish control/data domain. Data rows reference entity, operation, fields, predicate, activation mode and optional base Space permission |
| `role_permissions` | Reference stable permission IDs. Keep one join model, with ownership/domain constraints; mutation checks the parent role revision |
| `access_grants` | Retain table name; represent role assignments only. Add `scope_team_id`, distinguish it from recipient `group_id`, require `role_id` and remove the direct-permission alternative |
| `spaces` | Preserve fixed database binding; expose schema/policy authoring and active definition metadata through Space APIs |
| `apps` | Add trusted/external classification, Space ceiling, inherit/custom-role mode and optional App restriction. Store a Space-approved runtime origin for credential binding; keep inherited database binding and current deployment preference |
| `entity_definitions` — new | Stable ID, Tenant/Space, optional owning App, immutable namespace, lifecycle, draft document/revision, active/pending version references and publication status/error |
| `entity_versions` — new | Immutable manifest/schema/invariants/mappings and expected prior version/hash; mutable pending/applying/published/failed job status with attempt identity |
| `sessions` | Reuse with console/App purpose, pending/active/consumed state, App/origin-generation binding and issuing-console reference; challenge/nonce fields belong only to pending launches |
| `audit_events` | Preserve append-only registry history; record old/new role, permission, assignment and definition changes with actor and request IDs |

This adds two registry tables to the existing seventeen. Other provisioning changes are limited to explicit schema-management credentials and a credential-generation counter on database resources where needed. There is no new authorization service, message broker, graph database or policy cache.

Important columns and constraints:

- `roles.scope_kind` is `platform`, `tenant`, `team`, `space` or `app`. `domain` is `control` or `data`. Protected management/team roles have global definitions. Custom data roles require an owning Tenant and Space, plus App for App roles. Custom Tenant/team roles are rejected.
- `permissions` uses `id`, owner identity, local `code`, domain, state and revision. Control definitions are protected. A data definition references one same-Space entity. `acl.manage` carries a validated list of delegable permission IDs; per-record grants accept only same-entity role-mode Space permissions for read/update/delete, never create, management actions or recursive delegation of `acl.manage`. App permissions additionally reference a same-Space base permission and matching owning App.
- Retain checked owner keys for composite foreign keys: examples are `builtin:team`, `space:<UUID>` and `app:<UUID>`. Each key must match real typed ownership columns; it is not a free-form security principal.
- Assignments have a recipient user XOR recipient team; protected control roles and true team roles require a user. Non-platform assignments always have `tenant_id`. Tenant scope has no child ID; team, Space and App scope each require exactly their matching child ID. App ancestry is resolved from `apps`, not expressed as a second assignment scope.
- Composite foreign keys enforce same-Tenant recipients/resources, matching role category and valid definition ownership. Team assignments also reference the recipient’s membership in that team. Do not substitute an unconstrained `(scope_type, scope_id)` pair for these references.
- Role/permission joins enforce matching domains. Space roles reference Space permissions; App roles may reference permissions from their Space or their own App. Cross-App and cross-Space combinations are invalid. Use checked owner keys plus composite references, supplemented by service validation for JSON field references.
- Logical duplicate assignments remain rejected by a canonical unique key. Names/codes are unique within the definition owner, and names never decide authority. Archived definitions remain referenced for history; active dependencies prevent destructive removal.
- All authoring resources carry revisions and use `If-Match`. New security-sensitive fields and new entities default to no data permission until explicitly included in publication.

Constraint tests must try direct invalid inserts, including nullable composite-key bypasses. Every scope branch explicitly requires its mandatory columns to be non-null; a foreign key containing nulls is not sufficient enforcement. Cross-server record grants and JSON references use the same typed validators, since registry foreign keys cannot protect them. A malformed or unsupported stored policy fails closed at runtime even if it bypassed an API.

Protected control action codes and their allowed administrative ancestry are platform-defined. For example, Tenant administration covers permitted child inventory operations, while team administration covers only the selected team; this does not turn a Tenant role into a business data role. Persist this distinction in the permission domain and enforce it at both API and compiler boundaries.

### Business databases

Use InnoDB typed tables for entity data, with common `id`, immutable Tenant/Space identifiers, `revision` and UTC timestamps. Declare entity/field/permission dependencies through stable IDs. Physical names derive from immutable entity/field IDs and are stored in validated mappings; client names never enter SQL identifiers directly. Different Spaces sharing a Tenant database have separate entity namespaces and mandatory Space predicates. Record IDs are scoped by entity; all ACL, audit, cursor and relation keys include the entity and Space context.

Create a small local business-audit table before enabling entity writes; add `lastapp_record_grants` before enabling sharing in that physical database. Both have verified framework ownership markers, including on registered databases. Grants have recipient XOR, permission revision, bounded per-record cardinality and indexes for entity/record and principal lookup. A canonical unique key covers entity/record, recipient and permission ID; renewal updates that row rather than adding a duplicate per permission revision. They contain global IDs, not copied catalogs. A generic grant table cannot have a foreign key to every generated entity table, so supported mutations use the parent-lock/cleanup protocol.

The entity executor can address only registered entity tables and declared fields. Local ACL/audit tables and existing unrelated tables are outside that namespace. Registration of an existing database remains read-only verification; publishing new framework-owned entity tables is a separate, explicit mutation. Mapping arbitrary pre-existing tables is outside the first entity adapter.

Record-ACL and business-audit writes occur in the same local transaction as their parent mutation. Store operation, IDs, actor, revision, changed field IDs and request ID by default, not field values, bearer tokens or predicates populated with business data. Audit writes have byte/retention bounds and cannot be skipped on failure. Administrative policy audit stays in the registry. Local business audit is not exposed through the global administration audit endpoint; any later reader needs explicit data authority.

Runtime credentials are service-held and may access only the selected business database; schema/provisioning credentials are never available to App code or entity handlers. The service enforces access to the local ACL/audit tables even where runtime credentials need SQL privileges on them. This is an API-enforced boundary, not database row security against a compromised runtime. Multiple Spaces in one database share a service-level failure boundary; stronger infrastructure isolation requires dedicated databases/credentials and later dedicated runtime placement. Registered-database owners can bypass API rules with their own direct credentials; invariants are guaranteed for writes through Lastapp, not arbitrary external writers.

## 5 Definition language and schema publication

Use JSON Schema Draft 2020-12 for the supported data-shape vocabulary, plus versioned Lastapp metadata for field IDs, relationships, default projections, indexes and invariants. Advertise the subset and reject unsupported keywords, unknown submitted fields and executable extensions. Resolve only bounded local schema references; no remote `$ref`, arbitrary recursion or unrestricted regular expressions. Enforce payload depth/size limits before validation. JSON Schema alone is not an ACL, storage or operation language. [JSON Schema specification](https://json-schema.org/draft/2020-12)

Initial types: bounded text, boolean, browser-safe signed integer, decimal with declared precision/scale, UUID/reference, UTC timestamp/date, enum and validated JSON payload. Integer JSON numbers are limited to the exact JavaScript integer range; do not silently round a full 64-bit SQL integer. Decimals are strings and timestamps are canonical UTC strings; generated wire schemas reflect these encodings. Nullability is explicit; create applies declared defaults, and PATCH omission means unchanged. PATCH is a top-level field replacement operation, not arbitrary JSON Patch. Arrays/objects can be validated/stored as JSON, but nested query paths are unsupported.

Text equality/order is case-sensitive and non-normalizing, with trailing spaces significant; use an explicit compatible no-pad binary collation instead of database defaults. Define decimal rounding/range rejection and timestamp precision once for the evaluator and adapter. Policy comparisons with null fail unless an explicit null operator handles them; in particular `ne` does not silently include nulls. Missing and null are different at the API boundary; stored typed fields have explicit nullability.

Initial relationships are same-Space entity references to already published targets, plus declared global user/team references within the Tenant. A shared entity cannot reference an App-private entity; a private entity can reference shared or same-App private entities. New/changed entity references require target-ID read permission; fetching target content is separately authorized. Use composite local foreign keys including Tenant/Space and a fixed RESTRICT deletion policy, with no automatic cascades. New/changed global user/team references validate active same-Tenant membership in the registry snapshot; knowing an ID does not grant directory browsing. Disabling a global target preserves existing business references, while policies honor its inactive state. Cross-Tenant references, cross-Space joins and cyclic publication dependencies are outside this phase.

Initial invariants cover required/type/range checks, declared uniqueness, immutable fields, enum transitions and references. Uniqueness and relation restrictions require database constraints, not a racy preflight SELECT. Schema authors choose uniqueness scopes explicitly. A conflict can reveal that a value/dependency exists even if its row is hidden; generic errors remove details but cannot eliminate that signal. Do not promise secrecy of globally unique values through row permissions alone.

The predicate representation is a bounded, typed expression tree: `all`, `any`, equality/order comparisons, bounded set membership, `is_null`, `is_owner` and `has_team_role`. Actor context variables are fixed; fields and relationships must resolve through published metadata. No SQL fragments, JavaScript, shell code, remote policy evaluation or LLM execution are accepted. Set explicit depth/node/array limits and test each operator’s null/type behavior against the evaluator and SQL adapter.

Publication has one durable state sequence:

```text
draft → pending → applying → published
                         └→ failed → explicit retry
```

Use `entity_versions` as the schema-work queue. Snapshot one immutable manifest with its expected active version; only one pending publication per entity is allowed, and further draft edits do not alter that manifest. Validate references, existing policies and App restrictions against the exact version. Serialize physical work on a pinned business-database advisory-lock connection; use registry compare-and-swap on job/active versions to prevent a stale worker activating an obsolete manifest. Loss of either required ownership check stops further work. A partially executed DDL statement may finish; its successor inspects the schema rather than assuming rollback.

The active pointer changes only after physical verification. An existing entity continues using its previous version during compatible work/failure; a new entity without a published version is unavailable. New-field permissions activate only after that version is active. Runtime metadata and authorization load exact published versions; mutable drafts never enter execution. Keep compiled immutable schema artifacts by version/hash, without caching mutable permission decisions.

First delivery supports new owned tables, new nullable fields without backfill, non-unique indexes and display-label changes. New-field defaults apply to new API creates, not implicit rewrites of existing rows. Public keys stay immutable. Tightening existing invariants, adding required/unique constraints to populated entities, type changes, deletion and database moves need a later coordinated migration feature. This avoids promising safe online constraint changes while older requests are still writing. Relevant MariaDB DDL implicitly commits; there is no transactional schema rollback. [MariaDB implicit commits](https://mariadb.com/docs/server/reference/sql-statements/transactions/sql-statements-that-cause-an-implicit-commit)

Additive DDL can still take metadata locks or rebuild a table. Give schema jobs separate small concurrency budgets, lock/statement deadlines and an explicit failure/retry result; a long schema job must not monopolize provisioning. Fail rather than silently choose a blocking/copying alteration outside the configured budget. Publication does not promise zero downtime or unlimited online index creation.

Schema workers use explicit schema-management credentials; ordinary entity connections remain CRUD-only. Managed databases can use the restricted provisioning target path with reviewed CREATE/ALTER/INDEX privileges on owned resources. Registered databases need explicit opt-in credentials before framework DDL. Store any added credential envelope with the existing owner/Tenant-bound encryption helper.

Every generated table, including local support tables, has a persistent ownership marker. Retries inspect ownership and actual columns/indexes and complete the same immutable plan. Unknown tables or incompatible layouts fail safely. Reuse the provisioning worker’s lock/diagnostic primitives, while defining publication ownership checks explicitly; its current global leader lock alone is not a cross-database fence. Never hold a Tenant registry lock during DDL or network calls.

## 6 Request execution and query compilation

Implement one validated query representation with a MariaDB adapter first. Browser requests use entity and field IDs/names, bounded filters, explicit sorts, projection and cursor pagination. The adapter generates parameterized SQL; the database optimizer chooses execution plans. MongoDB and other adapters are later implementations of the same tested semantics, not additional drivers in this delivery.

Request sequence:

1. Select the control/data entry point, parse the credential and hash its token.
2. Authenticate purpose, state, expiry and binding in one consistent read-only registry snapshot: active identity/memberships, relevant assignments/definitions, Space/App lifecycle, origin generation and ceiling. Materialize the bounded authorization context, then close the registry transaction before business I/O. No positive-decision cache or global-admin data bypass.
3. Resolve entity/field references, validate the query, build the typed authorization predicate and include any local record-grant `EXISTS` predicates.
4. Apply mandatory namespace/lifecycle guards, Space allows, App restrictions and business filters before sort, count and pagination.
5. Execute with bounded SQL time, rows, projection size and response size. Serialize only declared authorized fields.

For example, Joe’s team predicate becomes a bound set of teams for which he currently holds Reviewer, compared with the entity’s declared owning-team field. Record-sharing checks use indexed `EXISTS` in the business database, including the permission revision. Load relevant registry metadata with bounded batched queries, not one query per role, field or returned row. Do not fetch records or huge lists of record IDs and filter afterwards.

Initial read operations include `eq/ne`, comparisons, bounded `in`, null checks, boolean combinations, stable sorting, cursor pages and authorized count. A relation read is separately authorized on its target entity. Defer relevance search, arbitrary joins/relationship traversal and arbitrary aggregates; requests return a clear unsupported-query error rather than an unfiltered fallback. Later aggregates/search must consume the same authorization plan before their own operations.

Read fields used by filter/sort/count are included in authorization. A hidden salary field cannot become an unauthorized filter or sort key. Signed cursors bind Tenant/Space/App, entity/schema version, query/projection and sort; they carry no authority and every page reauthorizes. Concurrent writes can change page/count results; this API does not promise a snapshot spanning requests. Counts are explicit requests, not an automatic full scan on every page.

Start with defaults of 50 rows, maximum 200, bulk maximum 100, predicate depth 8, 64 nodes and client `in` lists of 100. Also bound request/response bytes, expanded policy paths, actor-team expansion, SQL parameters, statement/lock duration and queued requests. Limits apply to generated authorization SQL as well as client filters; exceeding them is a clear error, never truncated authorization. Authoring/assignment services prevent configurations exceeding supported bounds. Per-Tenant/user admission limits and fair process concurrency prevent one App occupying every connection. Authoring quotas are per Space/App plus Tenant totals, not one Tenant-wide custom-role cap. These limits do not cap business row counts.

Use a unique ID tiebreaker and ordinary schema-selected indexes. Avoid index creation for every possible permission combination.

Writes use local READ COMMITTED transactions and record revisions. Determine the bounded target/reference lock set and acquire it in deterministic entity/ID order. Target-ID reads use bound namespace predicates; evaluate current local ACLs and complete pre/post-state paths under the parent lock, then mutate and audit. Sharing takes the same parent lock. `FOR UPDATE` runs inside the transaction; data writes never call the Tenant-serializing control-plane `mutate`. Deadlocks/lock timeouts roll back the whole attempt and return a retryable conflict; there is no unbounded automatic retry. [MariaDB row locking](https://mariadb.com/docs/server/reference/sql-statements/data-manipulation/selecting-data/for-update)

Missing and inaccessible individual records produce the same not-found response; authorize before returning revision details. Bulk failures omit per-item existence information. SQL errors, unique values, referencing rows and hidden rule inputs are never returned. Generic conflict/reference errors still have the constraint-existence limitation described in section 5. Do not claim constant-time responses or complete prevention of inference through resource timing.

Bulk create/update/delete is bounded and atomic for one entity within one Space database. Reject duplicate IDs and mixed operations in a batch. Every item is authorized/validated and any failure rolls back everything. No cross-entity/Space transaction, update-all-by-filter, upsert or arbitrary batch command language initially. Create IDs are stable client UUIDs; duplicate IDs return conflict rather than replaying an earlier response. After an ambiguous network failure, the client reconciles by an authorized read before deciding to retry. External integrations and browser workflows are outside the database transaction.

Revocation affects registry snapshots begun after its commit. An already authorized, bounded request may finish. Share revocation and writes order through their shared record lock; reads can finish using a local snapshot begun before revocation. No cross-database cancellation or distributed transaction is promised. Read authorization from the registry writer, not a lagging replica. An unavailable registry fails new attempts closed. Registry read capacity is an explicit scaling dependency; future caching needs a defined revocation contract. Cache immutable schemas by exact version/hash, not mutable roles/grants or their names.

## 7 Console and App authentication boundary

Keep management and data entry points distinct while sharing identity, role storage and policy primitives:

- `AuthorizeControl` checks the protected management action catalog and administrative scope. Its platform superadmin behavior applies only to control operations.
- `AuthorizeData` requires active Tenant membership, published entity metadata and a server-bound Space/App context. Platform or Tenant administration supplies no implicit business permission.

Extend `sessions` with console/App purpose and checked states. Console sessions have no App binding; development users sign in again after a schema reset. Both the console principal and CSRF loaders require console purpose; data requires an active App session with all bindings non-null. App tokens are opaque random values stored hashed, bound to one user, Tenant, Space, App and origin generation. They contain no trusted cached role list. Data routes accept only the App bearer, control routes only console credentials. Implement this before exposing any data endpoint.

Use a trusted launch broker: the SDK creates a nonce and PKCE S256 challenge; the host matches the actual registered frame/window and origin to the installed App, rather than trusting a posted App ID. Its authenticated, CSRF-protected control request creates a pending launch. Pass the code through an exact-target-origin message, never an HTTP query string. The App exchanges code/verifier/nonce for its token. Lock and consume the hashed pending code and create a new active token in one registry transaction; recheck current user/membership/App state. A pending row can never authenticate an API request.

Use configurable defaults of 60 seconds for launch codes and 5 minutes for App tokens, bounded pending/active sessions per user/App and expiry cleanup. Bind challenge, nonce, origin and generation to the launch; require matching browser Origin at exchange. Tokens stay in App memory, sent in Authorization headers. Renewal repeats the exchange. Generated code gets neither a provider refresh token nor a console session. Bind child App sessions to their issuing console session: expiry, revocation or logout invalidates pending launches and active children.

Issuance checks activation, membership and the Space ceiling; each request reloads policy/state and the issuing-session binding. Origin is a browser restriction, not proof of code identity: a bearer remains a bearer if stolen. CORS allows exact origins only, rejects opaque/null origins, sends no credentialed-cookie allowance on data routes and varies responses by Origin. Personalized data/metadata is excluded from shared HTTP/CDN caches. Tokens/codes are excluded from logs, server URLs and shared browser storage. Console cookies remain host-only, Secure and HttpOnly in production with CSRF/origin checks; sibling App hosts cannot be permitted to plant an admin cookie.

Untrusted code is isolated from the admin origin and unrelated Apps. A different port alone does not isolate cookies. Register one canonical exact HTTPS runtime origin per App, unique across active Apps and different from the admin hostname; loopback HTTP is development-only. Origin changes require Space authority, increment an origin generation and invalidate old launches/tokens even if the old origin is later reused. The trusted host controls frame sources, messaging and allowed embedding; App code cannot navigate it or read its credentials. Per-App origins protect storage, service workers and DOM boundaries. Isolation does not stop an App author exporting data deliberately entrusted to that App, which is why its ceiling matters.

The current `app_origin` check is a guardrail. This phase implements the credential contract and an isolated test harness; npm packaging, WebContainers and the general code-library runner remain separate. Integration proxies/service credentials need their own scoped contract later; entity tokens do not implicitly authorize arbitrary outbound URLs or provider requests.

## 8 API and console changes

All management APIs remain global platform APIs with Tenant/resource-scoped routes. They use the same registry; “Space API” does not mean deploying another identity service per Space.

| API | Operations |
| --- | --- |
| `/v1/roles/catalog?scope_kind=...` | Fixed role definitions and supported control actions |
| `/v1/tenants/{t}/role-assignments` | List/assign/revoke fixed Tenant roles |
| `/v1/tenants/{t}/teams/{team}/role-assignments` | Assign fixed team roles to team members |
| `/v1/tenants/{t}/spaces/{s}/roles` | Space business-role CRUD and protected management-role visibility |
| `/v1/tenants/{t}/spaces/{s}/permissions` | Space business permission/policy definitions |
| `/v1/tenants/{t}/spaces/{s}/role-assignments` | Space assignments to users or teams |
| `/v1/tenants/{t}/spaces/{s}/entities` | Entity drafts, inspection, publication status and explicit retry |
| `/v1/tenants/{t}/spaces/{s}/apps/{a}/roles`, `/permissions`, `/role-assignments` | Optional App business definitions and assignments |
| `/v1/tenants/{t}/spaces/{s}/apps/{a}/access` | Space ceiling, inheritance/custom-role mode and App restrictions, with separate editing authority |
| `/v1/tenants/{t}/spaces/{s}/apps/{a}/sessions` | Trusted launch and scoped App-session issuance |
| `/data/v1/auth/exchange` | One-use launch redemption |
| `/data/v1/entities/{entity}/...` | Metadata, query/count, single CRUD, bounded bulk CRUD and record-share administration; context comes from the token |
| `/data/v1/context` | Minimal caller identity and App/Space context; caller team/reference options, no provider identifiers/secrets or general Tenant directory |
| Scoped capability/explanation endpoints | Explain the caller’s role, scope and policy decision without exposing other users’ data |

These route groups share handlers/services rather than copied implementations per category. Use `/teams` consistently in the API and console while retaining internal `groups` table names. Replace the current Tenant custom-role/direct-permission APIs outright and update their React callers/tests together. No compatibility aliases, deprecation period or second permission engine is needed. Preserve inventory/provisioning behavior.

Data context returns the caller’s ID/display name and active team IDs/display names. Broader directory browsing stays in authorized control APIs; App endpoints do not inherit membership/authorization-list access. Sharing accepts an exact recipient ID and validates active same-Tenant membership with a generic invalid-recipient error. A general business directory needs a later explicit exposure policy rather than silently publishing all Tenant users to every App.

Console changes:

- Tenant access page shows fixed Tenant roles and assignments, with no “create custom Tenant role” flow.
- Team pages distinguish membership from Joe’s role within that team, with fixed-role selectors.
- Space pages manage entities, shared permissions/policies, business roles, assignments and App ceilings.
- App access defaults to “Inherit Space permissions”; advanced controls are collapsed until explicitly selected. Display the source of inherited access instead of copying editable role definitions into the App.
- Separate manager assignment from data-role assignment. Explain changes before saving and preserve current audit/revision feedback.
- Start with forms, tables and a validated JSON editor for schemas/predicates. No visual workflow engine, drag-and-drop policy builder or bespoke UI definition language.

An SDK exposes context, entity CRUD/query/bulk and advisory capabilities. Buttons may use capabilities to render useful UI; the server authorizes every operation independently. Two ordinary Apps should use the same entity client and Space role definitions without writing authorization logic themselves.

## 9 Required changes to the current code

| Current location | Required correction |
| --- | --- |
| Initial schema/bootstrap SQL and `design/control_plane.sql` | Update the development schema directly for typed scopes, permission IDs, session purpose and entities; retain the security constraints. No legacy conversion scripts |
| `internal/control/access.go` | Replace Tenant custom-role CRUD with scoped role/permission services; add team-role assignments; retire arbitrary direct permission grants; preserve duplicate/ownership/audit checks |
| `internal/control/auth.go` | Extract reusable identity/context and grant primitives. Replace flattened Tenant/Space/App scope unions with typed assignments and separate control/data authorization. Team-role lookup must include the target team’s active membership |
| `internal/control/diagnostics.go` | Keep the closed control-action catalog; add a separate validated data-operation catalog rather than broadening the superadmin bypass |
| `internal/control/http.go` | Add scoped APIs and context-aware capability responses. Remove “platform admin means all permissions” from any business capability response |
| `internal/control/tenants.go` and membership handlers | Protect the last active administrator across user/membership/assignment changes. Keep Tenant membership disable-only; team removal deletes dependent team-role assignments |
| `internal/control/inventory.go` | Space is the definition owner. App creation starts in inherit mode with runtime approval required; ancestry is server-loaded and immutable. Initial manager assignment is explicit and validated |
| `internal/control/core.go` | Keep `mutate` for short control changes. Data writes use local record transactions; add version-bound definitions, strict session purpose/state and issuing-session revocation |
| `internal/control/worker.go`, `databases.go` | Reuse lock/credential primitives with an explicit publication ownership/CAS protocol; separate schema credentials and job budgets from runtime CRUD/provisioning. Preserve TLS, markers and safe retries |
| `internal/control/development.go` | Seed fixed team/manager catalogs and explicit sample Space data permissions; do not seed another generic Tenant custom-role system |
| `web/admin/src/main.tsx` and API helpers | Replace Tenant role-authoring UI with the scope-aware pages above; preserve shared components, themes, pagination and current accessibility. Do not reuse the console’s `platform || ...` capability shortcut for business UI |
| Existing Go/browser tests and CI | Update fixtures/callers to the corrected model, preserve all ten security properties and provisioning behavior, and add the semantic matrix against the current installed MariaDB setup |

Reuse the dedicated pinned `github.com/aantich/go-mariadb` fork. No ORM or driver replacement is required. Add `internal/authz` for typed policy/context compilation and `internal/entities` for schema/CRUD execution as their reuse becomes real; keep control handlers in `internal/control`. A MariaDB compiler module and a TypeScript SDK are sufficient. No independent microservices or generic repository framework. Runtime pools are reused across Apps by database-resource ID and credential generation, closed when idle, and bounded by both per-resource and per-process connection budgets. Every query carries explicit Tenant/Space bindings; pooled session variables do not carry authorization state.

## 10 Development schema and fixtures

There is no deployed system to migrate. Replace the current development-only Tenant custom-role/direct-permission model with the approved schema and APIs directly.

1. Update the initial schema, bootstrap code and canonical SQL together, preserving security invariants and unrelated working changes.
2. Update development seeds and test fixtures to the fixed Tenant/team catalog and scoped Space/App business roles. Assign the sample managers and business permissions explicitly.
3. Test against a freshly named local registry or reset only known disposable fixtures. Do not drop registered business databases or discard unrelated local data.
4. Update backend handlers, React callers and tests together; remove superseded APIs without maintaining a compatibility layer.
5. Verify fresh initialization, sign-in, provisioning and the new authorization scenarios on the current development stack.

No role-mapping reports, legacy grant converters, maintenance windows, upgrade gates or production rollback procedures belong to this phase. Safe recovery of entity-schema publication remains necessary platform behavior: future users will change their business schemas while using their Apps. That is separate from migrating Lastapp’s current development database.

## 11 Implementation sequence and approval scope

Each milestone must leave the framework running and tested. The approval covers the listed implementation sequence; it does not authorize publishing generated third-party code, changing database drivers or testing unrelated MariaDB versions.

| Milestone | Concrete work | Exit condition |
| --- | --- | --- |
| 1 Fixed roles and true scopes | Correct the development schema/bootstrap and fixtures; typed scopes/catalog/constraints; team membership enforcement; fixed Tenant/team APIs and matching console changes | Fresh initialization works; Joe holds Reviewer for Marketing only; custom Tenant/team roles and invalid manager assignments are rejected |
| 2 Definitions and credential boundary | Space permissions/roles; entity drafts/versions; bounded evaluator; App inheritance/ceiling model; console/App sessions, minimal launch/exchange and origin binding | Team policies and exact App scope work in API tests; wrong-purpose/context credentials fail before any data route is exposed |
| 3 MariaDB entity operations | Additive owned schema worker; typed compiler; single/bulk CRUD, projection/count, invariants, local audit, revisions and bounded pools/admission | Two generic entities work through App credentials; fields/writes obey complete Space/App paths; publication survives interruption |
| 4 Sharing and consistency | Local record grants, revision-bound delegation/audit, ownership transfer, parent-lock ordering and registry snapshot boundary | Revocation/race and conditional-field tests pass; data writes take no control-plane Tenant lock |
| 5 Advanced App limits | Custom-role gate, correlated narrowing definitions and App-private drafts/publication; inheritance/ceilings remain the model from milestone 2 | Two Apps reuse one Space; an advanced App cannot cross-match base permissions, widen Space access or publish outside its authority |
| 6 Console and SDK | Reusable client SDK, isolated browser harness and scoped console screens using the existing credential boundary | Real UI demonstrates Joe’s Marketing access, cross-App reuse, scoped credentials and management/data separation |
| 7 Complete local system and verification | Run the integrated framework locally; update canonical docs and run current-stack API/browser checks | Approval scenarios pass from fresh fixtures; superseded APIs are gone; limitations and executed checks are recorded |

Dependencies follow this order. Keep provisioning and console behavior working as the schema, APIs and UI are updated together. Every data route uses the final credential/ceiling boundary from its first exposure; there is no temporary admin-cookie bypass. Reuse existing handlers/locks where semantics match, without a prerequisite codebase-wide refactor.

## 12 Acceptance scenarios

| Area | Required behavior |
| --- | --- |
| Team role meaning | Joe as Marketing Reviewer can perform the published Campaign review operation for Marketing, through both ordinary Apps; Engineering and sibling-Tenant records remain inaccessible |
| Membership and role revocation | Disable Joe’s user/Tenant/team membership or revoke his team role; subsequent snapshots lose affected access. Rejoining a removed team does not restore its deleted team-role assignment |
| Fixed catalogs | Tenant/team admins can assign supported roles but cannot create/alter them or assign platform authority |
| Shared Space roles | One Space role edit affects both Apps through shared references; no duplicated App role definitions or grant copies |
| Inheritance | A new ordinary App works with existing Space entities and permissions, with no App-specific business role configuration |
| App restrictions | Restricted mode denies users with no matching App role; extra App predicates/field sets narrow Space access; stale tokens cannot bypass a tightened ceiling |
| Base permission correlation | An App gate for P cannot use Space activation Q; disabling P breaks that path. A restricted ceiling admits no future field/entity implicitly |
| App extensions | Disabled by default; unauthorized drafts/publication fail; approved private entities stay in the owning App namespace and cannot alter shared tables |
| Authority separation | Platform/Tenant/Space/App managers with no data grants cannot read business rows; explicit policy/assignment changes remain audited |
| Administrative delegation | Teams cannot receive manager roles; team admins cannot acquire Space/App management by editing membership. Concurrent last-admin removal cannot lock out a scope |
| Field safety | “My salary” plus “everyone’s names” does not expose other salaries, including via filters, sort, count, relations or requested projections |
| Writes | One path covers the complete payload and both states; mixed pre/post rules, protected fields, invalid transitions/references and stale revisions fail. Write responses disclose no unreadable entity fields |
| Bulk | Mixed allowed/denied or stale items roll back the entire bounded batch; no partial unauthorized success or per-item existence leak |
| Record sharing | Delegation checks the same base permission and App ceiling; permission revision changes invalidate shares. Revoke/write order is defined; transfer/deletion clears shares, and recreation restores none |
| SQL isolation | SQL includes mandatory Tenant/Space bindings; entity/field names are resolved and quoted from trusted mappings; attacker strings cannot inject identifiers or predicates |
| Lifecycle | Suspended/archived parent scopes deny business access consistently for all users; management repair remains a separate authorized operation |
| Sessions | Wrong-purpose/pending credentials fail; App A cannot call App B/control. Concurrent redemption succeeds once; wrong nonce/verifier/window/origin fails. Logout, parent expiry and origin-generation changes invalidate child tokens |
| Browser isolation | Two App origins cannot read one another’s tokens/storage or the admin DOM/cookies. Forged launch messages fail; personalized responses never enter a shared cache |
| Publication | Interruption/lock loss reconciles owned partial DDL; obsolete workers cannot activate. Pending/failed versions stay inactive; unsupported blocking/constraint changes fail; unrelated tables remain untouched |
| Shared database | Two Spaces sharing one database do not see one another’s entity tables, records or ACL grants |
| References and types | Cross-App/private references, racy deletion, unknown fields, unsafe integers, decimal overflow, nulls, trailing spaces and time precision follow the published contract |
| Bounds and failures | Registry outage fails closed; generated policy expansion, request/SQL/lock limits and pool/admission budgets apply. Bounded batch failures roll back local audit and data together |
| Metadata and audit | App context exposes no full Tenant roster/provider IDs; capabilities reveal no hidden row values. Business values stay out of default audit and control-plane audit APIs |
| Fresh initialization | Schema/bootstrap and fixtures create the corrected model directly; no legacy grants/conversion dependencies; existing unrelated local data is untouched |
| Existing behavior | Provisioning/recovery, all ten prior security findings, pagination and console themes/accessibility retain regression coverage |

Use table-driven policy tests, evaluator-versus-SQL equivalence cases over generated small datasets, real MariaDB transaction/API tests, and browser scenarios for the user-visible flows. Bounds and failure behavior are part of those tests. Run on the current development stack; no server-version matrix, load benchmark project or new external authorization vendor is part of this approval.

## 13 Worked delivery example

The demonstration creates one Tenant, Marketing and Engineering teams, and a CRM Space with Company and Campaign entities. Company has ordinary Space Viewer/Editor role permissions. Campaign adds separate read and review/update team-role policies from section 3. Joe receives Marketing Reviewer once; an ordinary team member receives no review permission by membership alone.

Two Apps, Campaign Review and Campaign Overview, bind the same CRM Space in inherit mode. They use the same Campaign entity ID, permission definition and SDK. Joe can review Marketing Campaigns in both, cannot review Engineering Campaigns, and cannot edit undeclared fields. Changing the Space rule affects both Apps on subsequent requests.

A third App uses an explicit read-only ceiling and advanced App-role mode. Assigning Joe its Analyst role permits only the Space-authorized reads through that App; no App role enables editing or exposes another Space. A Space manager without data permission can administer these definitions but receives no implicit rows. An explicit data-role assignment is visible in audit history.

Finally, revoke Joe’s Marketing Reviewer assignment and demonstrate that both inheriting Apps lose review permission without schema changes, record rewrites or App redeployment. This is the acceptance demonstration, not a CRM-specific backend implementation.

## 14 Decisions requested for approval

Approval accepts:

1. The hierarchy and fixed Tenant/team catalog, one-scope assignments, user-only protected managers, and custom Space/App business roles.
2. Space-owned permissions with role or direct policy activation; additive grants, complete correlated permission paths, and optional App gates bounded by a Space-owned ceiling.
3. Conditional read fields that narrow rows; one complete path for writes; no implicit business access for administrators or full-row write responses.
4. Explicit optional sharing bound to a permission revision; ownership transfer clears shares, and grantor-role removal alone does not revoke an existing share.
5. MariaDB-first typed entities, bounded CRUD/query/count, single-entity atomic bulk writes, minimal JSON Schema/predicate vocabulary and narrowly additive publication.
6. App-scoped credentials and browser-origin isolation before data exposure; current-policy checks, a defined in-flight revocation boundary and explicit registry/pool capacity limits.
7. Direct development-schema changes, updated fixtures/callers and the seven implementation milestones, preserving provisioning/security behavior without migration or compatibility machinery.

The ordinary App path remains small: choose a Space, reuse its entities and permissions, build client micro-apps. Advanced authoring is opt-in. Broader database adapters, complex search/aggregates, arbitrary backend code, general workflow engines, dynamic container scheduling and the full generated-code/npm runner are outside this implementation plan.
