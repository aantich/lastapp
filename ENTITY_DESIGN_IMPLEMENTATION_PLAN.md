# Lastapp business entities ACLs and rules implementation plan

**Status: approved for implementation; core milestones and optional sharing implemented. Date: 12 November 2026.**

See [the runtime guide](ENTITY_RUNTIME.md) for current usage and supported boundaries, and [verification](VERIFICATION.md) for executed checks.

Build a reusable Space backend that lets authors describe business entities, grant access, define rules, test the result and publish a working API. Authors work with business concepts. Lastapp creates and manages tables, constraints, indexes and runtime metadata behind the scenes. Apps inherit the Space backend without copying its definitions.

The first usable release delivers the complete authoring and secured-data flow, including its React UI. The existing Go provisioning, identity, team and role foundation remains the base. Anton subsequently authorized full implementation of this plan.

## 1 Decisions and scope

### Confirmed product decisions

- Hierarchy: Tenant → Space → App → micro-app. Teams belong to the Tenant alongside Spaces.
- A Space owns shared entities, business rules, permissions and database binding. App-specific entities are an advanced extension, not the ordinary authoring path.
- Users describe entities only. There is no table designer, SQL editor, column mapping wizard or storage configuration in entity authoring.
- Initial rules cover validation, defaults, calculated fields and allowed changes. Workflows, navigation and arbitrary application logic remain browser code.
- Backend rules and authorization apply to every API write, including bulk requests. No arbitrary generated backend code is accepted.
- Existing global identities, teams, memberships and scoped roles are reused. No identity or role catalogs are copied into business databases.
- DRY is the main foundational principle. Visual authoring, agent authoring, runtime enforcement, generated contracts and preview use the same authoritative definitions.
- MariaDB is the first adapter, using the existing pinned Go MariaDB driver. The public contract describes business data independently of SQL syntax.

### Included in the first usable release

Space-owned entity definitions; field types and relationships; entity/record/field access policies; inherited App access with Space-controlled ceilings; typed rules; draft/test/publish; generated storage; single and bounded bulk CRUD; filtered reads, sorting, cursor pagination and count; scoped metadata and a small TypeScript SDK; the App authentication boundary needed to expose those APIs safely.

Record-specific sharing follows as an optional milestone after the core flow. Its semantics and storage are specified here so the core does not require a redesign. Existing custom App roles gain an optional narrowing gate; they never grant authority beyond the Space.

### Deferred explicitly

Automatic side effects, integration calls, scheduled rules, arbitrary backend functions, cross-record calculated aggregates, arbitrary joins, full-text/vector search, cross-Space relationships, destructive field/type conversions, a general backfill/transformation service, App-private entity authoring, generated micro-app builds/runners and container scheduling. JSON payload fields are supported as bounded values; nested-path querying is deferred.

This is a local, undeployed framework. There is no production migration/cutover project or legacy grant conversion. Normal registry schema changes and generated business-schema publication are still required. Preserve existing local data and credentials during development; do not reset them implicitly.

## 2 Authoring experience

### Navigation and creation

Add **Entities** to a Space's tabs: **Apps · Entities · Access · Settings**. Retain Space Access for people, teams and role assignments. Entity Access configures what those roles and relationships permit on an entity; these are two views of the same authorization model, not separate membership systems.

The Entities page lists name, short description, published/draft state and usage by Apps. Its empty state offers **Create entity** and a small set of templates. Creation asks for name and optional description; API keys are generated and placed under Advanced. Templates create editable definitions and explicitly chosen role references, never users, assignments, credentials or inherited access from another Tenant.

An entity opens with four tabs: **Fields · Access · Rules · Test**. The header shows draft state, last saved state and **Review changes**. Published content stays active while a draft is edited. A failed save keeps the draft in memory and says it is unsaved; no sensitive sample record or draft is silently persisted to browser storage.

Drafts may be incomplete. Saving checks the document envelope, supported syntax, ownership and size limits; semantic errors are retained as diagnostics. Publishing requires a complete valid definition. This lets an author save work before adding the final field, role or rule.

### Fields

Show name, type and a concise summary such as “Required”, “Default: Current user” or “Calculated”. Add/edit opens one consistent field editor. Start with the essential choices; reveal type-specific constraints only when relevant.

Relationships are phrased as “One Company” or “Many Contacts through Company”. A user/team reference uses the existing scoped picker. Choosing an owner or owning-team field is an explicit entity setting, not inferred from its label. Selecting a calculated field opens its formula directly. Formula and default editing here are views of the same definitions shown under Rules.

Users choose business requirements such as uniqueness, permitted values and whether a field is available for filtering/sorting. Lastapp selects physical representations and a bounded index plan. The UI never asks for SQL types, indexes or table names.

New fields receive no access merely because a role can access existing fields. Review changes offers an explicit step to include the new field in selected existing permissions. “All current fields” expands to an explicit field list when saved; it is not a future wildcard.

### Access

Use readable access-rule rows: **Who · Can · Which records · Which fields**. The first screen offers Viewer/Editor-style starting points that fill ordinary rules and remain editable. Avoid a dense role-by-field checkbox matrix as the primary editor.

The rule editor asks:

1. **Who:** a Space role, named user/team, record owner, members of the record's team, or holders of a fixed role in that team.
2. **Can:** read, create, update or delete. Manage access is under Advanced.
3. **Which records:** all eligible records, a relationship preset, or a typed condition.
4. **Which fields:** an explicit selection for read/create/update.

The UI generates a sentence such as “Reviewers of the record's team can update Stage and Close date.” Selecting Marketing is different from selecting the record's team; the relationship choice is visibly dynamic. Joe's Marketing Reviewer assignment therefore works for Marketing records without another assignment in every Space/App.

A row may display several operations together for convenience, but the stored permission entries remain operation-specific. Reuse the same editor when managing a role's permissions; changes target the relevant entity draft. Do not maintain a second permission editor with independent JSON.

Show inherited access in Apps with its Space source. App restrictions are secondary controls. Restrictions owned by the Space appear locked to an App manager. Record sharing is off by default and appears only after the optional sharing milestone.

### Rules

Group rules into **Defaults · Calculations · Validation · Allowed changes**. Every rule has a stable key, name and an optional description. Validation/allowed-change rules also have a message and optional field placement.

- “When created, Owner defaults to Current user.”
- “Expected value = Amount × Probability ÷ 100.”
- “When Stage is Won, Close date is required.”
- “When the existing Stage is Won, Amount cannot change.”

Use one type-aware condition/formula builder. Fields, operators and functions come from the backend capability catalog. The builder prevents invalid choices rather than exposing an error after a large expression is written. Plain text expressions may be added as an authoring convenience only if they parse to this same representation.

Permissions determine who may make a change. Rules determine whether the resulting record/change is valid. Role-specific restrictions on a change belong to the access permission's optional change condition; the entity's general transition rules remain common to everyone.

### Test

Test accepts a draft or active version, an App context, an eligible user, an operation, a synthetic existing record and proposed values. It returns a structured explanation of mandatory gates, matching permission paths, authorized fields, defaults, calculated values and validation results.

The first release uses synthetic records only. It never queries business records to populate a test, writes data, issues an impersonation token or invokes integrations. Current same-Tenant role/team relationships can be used to test “as Joe”. Publication/storage-dependent checks are marked **not checked**, such as uniqueness against live data, foreign-record existence and real record shares. A successful simulation means the demonstrated checks pass, not that a live write is guaranteed to succeed.

The production evaluator is reused; there is no browser reimplementation of policy/rule semantics. Explain output is available only to authorized entity authors. Later real-record testing must first authorize the requesting author's own read, independently of the simulated identity.

### Review and publish

Review changes shows a business-level diff: new/changed fields, access widened/narrowed, rules changed, affected Apps, shares made inactive and whether an existing-data check is required. Publishing takes one immutable draft revision. Status is visible and refresh-safe: **Queued → Publishing → Published**, or **Failed** with a safe reason and retry.

There is one ordinary review-and-publish action, not a multi-person approval workflow. Changes that the first release cannot apply safely remain a draft with a precise explanation. Data destruction is not disguised as a rename or ordinary edit.

All controls support keyboard use, accessible error placement, small screens and the existing appearance system. Route changes and background refreshes preserve captured revisions and unsaved edits. Navigation away from a dirty draft is explicit.

## 3 Authoritative definition and generated contracts

### One versioned entity document

Use a small, typed Lastapp JSON document as the authoring contract. It contains entity identity/labels, fields, relationships, default projection, business rules and access permission definitions. It references existing role/user/team IDs; it does not contain assignments or copied identities.

The authoritative draft and immutable published versions are stored centrally. SQL plans, JSON Schema, TypeScript types, form hints, dependency information and authorization indexes are generated artifacts. They cannot be edited independently.

This refines the earlier JSON-Schema-first proposal in `SPACE_AUTHORIZATION_PLAN.md`: JSON Schema is generated for record input/output shape and for validating the authoring document. It is not a second editable business model. Its annotations do not implement defaults or permissions; JSON Schema's `default` annotation does not fill missing values during validation. [JSON Schema annotations](https://json-schema.org/understanding-json-schema/reference/annotations)

Each entity has a UUID and an immutable API key within its Space. Each field has an immutable key within its entity; the entity UUID plus field key is its stable identity. Display labels are freely renameable. Published keys are not renamed or reused for a different field. SQL identifiers are generated from trusted internal mappings, never interpolated from labels or arbitrary request input.

Every document includes `format_version`. Unknown properties/operators and unsupported versions fail validation. First-party templates, agents and the visual editor submit exactly the same API format and receive the same diagnostics.

Illustrative field and rule fragment; the full document also includes identity and access definitions:

```json
{
  "format_version": 1,
  "key": "deal",
  "label": "Deal",
  "fields": [
    {"key":"name","label":"Name","type":"text","required":true,"max_length":200},
    {"key":"amount","label":"Amount","type":"decimal","precision":18,"scale":2,"required":true,"minimum":"0"},
    {"key":"probability","label":"Probability","type":"integer","required":true,"minimum":0,"maximum":100,"default":{"literal":0}},
    {"key":"expected_value","label":"Expected value","type":"decimal","precision":18,"scale":2,
      "calculation":{"op":"div","args":[{"op":"mul","args":[{"field":"amount"},{"field":"probability"}]},{"literal":100}]}},
    {"key":"stage","label":"Stage","type":"choice","required":true,"values":["new","qualified","won","lost"],"default":{"literal":"new"}},
    {"key":"close_date","label":"Close date","type":"date"},
    {"key":"owner","label":"Owner","type":"user","default":{"context":"actor.id"}},
    {"key":"team","label":"Team","type":"team"}
  ],
  "owner_field":"owner",
  "team_field":"team",
  "rules":[{
    "key":"won_requires_close_date",
    "kind":"validation",
    "when":{"op":"eq","args":[{"field":"stage"},{"literal":"won"}]},
    "assert":{"op":"is_not_null","args":[{"field":"close_date"}]},
    "message":"Add a close date before marking this Deal won.",
    "field":"close_date"
  }]
}
```

The field type supplies literal typing where unambiguous. Decimal literal arguments use an explicit decimal literal node when outside a typed field constraint. Publication resolves and type-checks every expression; string labels are never interpreted as executable policy.

### Source ownership

| Information | Authoritative source | Derived consumers |
| --- | --- | --- |
| Field/rule/access definitions | Entity draft or immutable entity version | SQL plan, runtime model, schemas, authoring UI, SDK metadata |
| Users, memberships and role assignments | Existing registry tables | Per-request identity/authorization context |
| Space permission identity and active revision | Registry permission projection of the active entity version | Role lookup and local share matching |
| Space-role permission links | Role references in active entity permission definitions | `role_permissions` lookup rows |
| App ceiling and App-role gates | Versioned/revision-guarded App access settings | Effective authorization paths and App role lookup rows |
| Record shares | Local business-database grant rows | Authorized query predicates |
| Operators, supported types and limits | Go capability catalog | Editor controls, validation schema and client hints |

Projections are transactional materializations, never independently editable definitions. Historical snapshots are intentional history. A rebuild/parity test proves projections match their canonical versions.

## 4 Field and value semantics

| Type | API value and initial behavior |
| --- | --- |
| Text | Unicode string, explicit length bound; case-sensitive, no implicit normalization or trimming |
| Integer | JSON integer inside JavaScript's exact integer range; reject unsafe numeric values |
| Decimal | Decimal string with declared precision/scale; exact arithmetic, explicit result rounding |
| Boolean | JSON boolean |
| Date | Valid calendar date, `YYYY-MM-DD`, without a timezone conversion |
| Timestamp | Canonical UTC timestamp with fixed supported precision |
| Choice | Stable string keys with separately editable labels |
| Entity reference | UUID of a record in an explicitly declared same-Space entity |
| User or team | Global UUID validated within the same Tenant |
| JSON object/list | Bounded structured value with declared supported shape; whole-field replacement, no nested queries initially |

Missing and null differ. On create, omission invokes a default if one exists; otherwise a required field fails and an optional field becomes null. Explicit null never invokes a default. On PATCH, omission means unchanged and explicit null clears an optional field. PATCH replaces supplied top-level values; it is not JSON Patch.

System `id`, `revision`, creation/update timestamps and creator/updater identities are separate from author fields. IDs may be supplied as validated stable create UUIDs for retry reconciliation. Other system values and calculated fields are not writable. Creating a record does not automatically grant access; an explicit owner policy may grant it.

The compiler derives structural validation and SQL constraints from the same field definitions. Uniqueness is enforced by an actual database constraint, with explicit entity/team/owner scope. Null semantics of unique fields are fixed in the contract: optional null values do not conflict. Generic conflict messages avoid returning a hidden conflicting record; success/failure can still disclose that a constrained value exists.

Use exact decimal arithmetic with bounded precision/scale and a single rounding rule, initially half-even at the declared calculated-result scale. Reject excess-scale client input, overflow, division by zero and invalid dates; never silently coerce or round an input amount. Formulas do not use binary floating-point money arithmetic.

Arithmetic involving null produces null; authors use `coalesce` when they want another value. A required calculated field rejects a null result. Scoped uniqueness requires a non-null scope field, so nullable owner/team values cannot accidentally bypass its constraint. System field names are reserved and cannot collide with author keys.

References initially target the same Space and a published entity. Self-references are allowed with the same checks. Relationships between two new entities can be established after both exist. A many-to-many relationship is an ordinary link entity with two references. Deleting a referenced record is restricted; no cascade engine is introduced. User/team disable preserves historical references while removing access from inactive relationships.

Changing a reference validates the target and the caller's read authority. Fetching related content always authorizes the target entity separately. Existing references are not revalidated as new assignments on unrelated updates. Existing-database registration does not imply mapping or adopting pre-existing tables: entity publication creates only Lastapp-owned storage in the bound database.

## 5 ACL semantics

### Permission definitions and activation

A permission has stable ID/key, one owning entity, one operation, explicit fields where applicable, a record condition and activation. Operations are `read`, `create`, `update`, `delete` and advanced `acl.manage`.

Two activation forms cover the UX without duplicate role assignments:

- **Roles:** activated through current Space role assignments; its referenced roles generate the existing role-permission joins.
- **Relationship:** activated by a typed actor/record relationship, such as owner, same-Tenant team membership or the fixed `team.reviewer` role for `record.team`. Explicit same-Tenant user/team selections are also relationship predicates.

Only role-mode permissions can be delegated through the optional record-sharing feature. A share replaces role activation for that permission; its record condition, fields, lifecycle and App limits still apply.

No matching allow path means deny. Allows are additive. There are no explicit deny entries, nested role inheritance, recursive teams or arbitrary scope combinations in this release. Mandatory guards always apply.

### Complete permission paths

Each allowed path must retain the same base Space permission throughout:

```text
active identity, Tenant membership and Tenant/Space/App/entity
AND correct server-resolved database and authenticated App binding
AND mandatory Space/entity guards
AND current activation of permission P, or an eligible share of P
AND P's record condition and fields
AND Space-owned App ceiling for P
AND optional App restriction/custom-role gate for P
```

Do not combine a Space activation for permission P with an App gate for permission Q merely because both grant `update`.

Space business-role assignments apply through all inheriting Apps. Fixed team roles participate only where a policy references that relationship. Administrative roles confer no automatic business-data access. An author who can change policy can explicitly grant themselves data access; that governance change is auditable and must be shown in the publish diff.

### Read behavior

For each returned record, every selected field must be allowed by at least one complete read path. Different paths may supply different fields, but each field stays attached to its own record predicate. “Salary on my record” plus “Names on all records” must never reveal all salaries.

Filter and sort fields require the same authorization as returned fields. A request's authorization predicate includes the union of those referenced fields and intersects their per-field allowed-row predicates before filtering, counting, sorting or pagination. Including a restricted field can narrow results. Do not perform implicit per-row null masking.

Every read path includes the record ID and revision. An entity has an explicit small default projection; caller metadata identifies usable projections. Counts use the same authorized field/query predicate as their corresponding read, without pagination. Related records require their own read paths. Missing and inaccessible individual records have the same not-found response.

### Write behavior

One complete path must authorize an update's entire client-supplied field set. Do not assemble a multi-field write from unrelated permissions. Record-scope conditions must hold on both the old and resulting record; this prevents changing an owner/team to escape a policy.

An optional update `change_when` predicate may reference `old` and `new` to authorize a particular transition. It is separate from record scope, so “may approve a pending item” can explicitly require pending → approved rather than accidentally requiring the result to remain pending. General allowed transitions remain entity invariants.

Create authorizes the supplied fields and final values after trusted defaults/calculation. Defaults do not require the caller to submit hidden system-supplied fields, but the resulting record must pass its create predicate. Delete authorizes the current record. Calculated/system fields are never accepted as client fields, even when an access rule mistakenly lists them; publication rejects such permissions.

Reassigning a declared owner/team requires both the update path and `acl.manage`; it clears local shares transactionally. Initial ownership is controlled by create permission and defaults. Successful writes return IDs and revisions, not an unauthorized full record. Returning data is a separate authorized read.

### App inheritance and restrictions

The default is inheritance. A trusted App can have a Space-approved inherit ceiling. A restricted/untrusted App has an explicit list of base permission IDs, fields and optional record constraints. Missing entries deny, including future entities/fields. App managers can add restrictions but cannot edit or remove the Space ceiling or approve their own trust/origin.

Optional App custom-role mode requires an applicable App role for the same base Space permission. App role gates narrow fields and conditions; they never replace the required Space activation. Return to ordinary inheritance remains an explicit App setting within the Space ceiling. New permission definitions become available in trusted inheriting Apps only through their explicit Space activation; publishing shows affected Apps.

### Optional record sharing

Enable sharing per entity. Authors choose a small list of shareable Space permission IDs; “View” and “Edit” labels reference those definitions rather than copying fields/conditions. `acl.manage` contains the allowed delegable list. It cannot delegate itself, control authority or create permission.

A grant records recipient user XOR team, entity/record, permission ID and security revision. The grantor needs `acl.manage` and the complete permission they are delegating on that record through the current App ceiling. The recipient must belong to the same active Tenant context. Predicate conditions are reevaluated for the recipient.

Security-relevant permission changes increment its security revision; older shares become inactive until explicitly renewed. Label-only changes do not. Unchanged permission definitions retain their revision across unrelated entity publication. Publishing identifies affected shares without disclosing records to an author who cannot read them.

Sharing/revocation and writes lock the parent record. Delete or ownership transfer removes shares in the same local transaction. A granted share persists if the grantor later loses their own role; it is explicit delegation, not a recursive grantor graph. Disabling the recipient, membership, team or base permission removes effective access. Share rosters require `acl.manage` and are not exposed to ordinary readers.

## 6 Rules engine and execution order

### Shared typed expressions

Implement one bounded expression tree with an operator catalog. Each operator declares accepted types, result type, permitted contexts, evaluator and, where queryable, MariaDB compilation. Editor descriptors and authoring validation derive from that catalog. Unsupported expressions fail before publication; no executable SQL/JavaScript fragments, network calls, remote schema references or LLM evaluation enter request execution.

Initial conditions: `all`, `any`, `not`, equality/order comparisons, bounded `in`, explicit null tests and the supported identity/team relationship predicates. Formula operations include bounded exact arithmetic, explicit rounding and coalesce. Conditions compare stored values; arithmetic in a condition must first be expressed as a stored calculated field, preserving Go/SQL error semantics. Additional functions enter through the catalog with conformance tests; authors cannot upload operator implementations.

Ordinary comparisons involving null evaluate false; null must be tested explicitly. Logical NOT operates on that defined boolean result. Go evaluation and SQL compilation must agree, including SQL's own null behavior. Text comparison uses the selected no-pad binary semantics rather than default database collation. Equality, trailing spaces, Unicode lengths, decimals and dates have shared fixtures.

### Defaults and calculations

Defaults run only for omitted create fields and are either typed literals or allowlisted trusted context providers such as current user or request time. They do not secretly overwrite supplied values. A field that must always be server-set is declared system/calculated rather than described as a default.

Calculations initially depend only on fields in the same resulting record. They are pure and stored on create/update, in a compiled dependency order. Cycles and ambiguous types are publication errors. Request time is fixed per operation/batch for defaults; time-varying calculations and external lookup calls are unsupported. Every mutation recomputes affected stored calculations before validation and persistence.

Readable calculated fields can reveal information about inputs even when those inputs are hidden. They require their own explicit read grants; the publish diff flags new calculated-field exposure. Input secrecy is not inferred from the output's label.

### Write pipeline

1. Parse bounded input, reject unknown/system/calculated client fields and load the authenticated, server-bound context.
2. Begin a local business transaction; verify the published storage generation and write gate; lock target records/references in deterministic order.
3. Check current record visibility/operation eligibility without exposing hidden data or revision details.
4. Construct candidate values: merge PATCH or apply create defaults, parse types, and evaluate stored calculations.
5. Authorize the entire submitted change through one complete path, including old/new scope and any `change_when` condition.
6. Run field constraints, resulting-record validations and allowed-change assertions. Validate changed references and ownership rules.
7. Apply SQL mutation with expected revision, database constraints, share cleanup and local audit in one transaction.
8. Commit and return IDs/revisions. Optionally perform a separately authorized read.

The preliminary check in step 3 is not the final authorization decision. Failures before step 5 are generic for records the caller cannot access; detailed rule errors never expose hidden values. Batch operations execute this pipeline for every item and roll back together.

## 7 Registry and business storage

### Central registry additions

Reuse existing identities, memberships, teams, roles, assignments, audit, database resources and App inventory. Add two entity tables rather than separate tables for every field/rule/function.

| Table | Purpose and essential columns |
| --- | --- |
| `entity_definitions` | Stable identity and draft: UUID, Tenant/Space, immutable key, label, lifecycle, draft JSON, draft revision, active version ID, pending version ID, timestamps |
| `entity_versions` | Immutable snapshot and publication job: UUID, entity/owner keys, base active version, definition JSON/hash, compiled format/compiler version, immutable storage plan/hash, state, attempt/fence identity, safe error, creator, timestamps |
| `permissions` extension | Protected control entries plus data-permission identity/projection: owner key, entity ID, domain, local code, active entity-version reference, security revision, active/disabled state. Data policy bodies remain solely in the canonical entity version |
| `role_permissions` extension | Reuse the existing domain-safe joins; add ownership validation and transactional projection updates for Space definitions/App gates |
| `apps` extension | Space-owned trust/origin/ceiling settings and App-owned restrictions/custom-role gates, revision guarded; references only to same-Space permissions and same-App roles |
| `sessions` extension | Console/App purpose, pending/active/consumed launch state, parent console session, Tenant/Space/App binding, origin generation and hashed launch challenge data |
| `database_resources` extension | Explicit encrypted schema credentials for registered databases where required, credential generation for connection-pool invalidation |

Use composite ownership keys/FKs wherever registry rows reference Tenant/Space/entity/role. Replace global permission-code uniqueness with owner-scoped uniqueness while retaining protected control identities. `permissions.domain='control'` must be extended to a checked control/data model; do not remove the domain boundary. Validate JSON references against typed ownership constraints at publication.

Entity versions are the publication queue. Avoid another generic jobs database, event broker or separate authorization service. Maintain one pending publication per entity. Draft saves use the existing optimistic revision pattern.

### Business database storage

Create one generated InnoDB table per entity with typed columns, system metadata, explicit Tenant/Space keys and an entity-owned physical namespace. Even when Spaces share a physical database, generated mappings and bound namespace predicates remain mandatory. Do not use a universal entity-attribute-value table.

Create platform support tables once per physical business database:

| Table | Purpose |
| --- | --- |
| `_lastapp_entity_state` | Ownership marker, entity/Space identity, physical layout hash, active storage generation, publication fence and write gate |
| `_lastapp_record_acl` | Optional indexed record grants using global identity/permission IDs; no copied identity/role definitions |
| `_lastapp_data_audit` | Transactional business mutation/share audit: actor, operation, entity/record, revisions, changed field keys and request ID |

Support tables are not addressable through the entity API. ACL records include Tenant/Space/entity/record, recipient user XOR team, permission ID/revision, grantor and creation time. Use indexes for entity/record and recipient lookup, plus a canonical uniqueness constraint. Record-parent locks and cleanup replace impossible cross-table foreign keys from one generic grant table to every entity table.

Generate local reference/unique constraints from the same definitions. Automatically index namespace/record identity, owner/team, references and explicitly filterable/sortable fields within limits; select composite indexes only from declared access patterns. Avoid automatically indexing every combination of permissions. Large text/JSON remain non-sortable/non-filterable unless the adapter explicitly supports the requested capability.

Audit stores changed field keys rather than business values by default, with bounded retention and payload size. Business audit stays in the business database and has no platform-superadmin data-reading endpoint. Global policy/definition changes use existing registry audit.

Runtime credentials are CRUD-only and server-held. The publication worker alone uses schema authority. Managed resources reuse the approved provisioning path for schema work; registered resources need explicit platform/Tenant infrastructure setup of schema credentials. An entity author sees “Storage setup required” with the responsible administrator, not a credential or SQL wizard.

The API is the enforcement boundary. Owners of externally registered databases can change data directly using their own credentials; Lastapp cannot guarantee invariants for those external writes. Sharing a physical database also shares an infrastructure failure boundary.

## 8 Publishing and recovery

### Supported changes

Initially support new entities, optional field additions, display/documentation changes, permissions and supported rule changes. Default changes affect future creates. Allowed-change rules affect future mutations. Tightening a resulting-record constraint requires validating existing records before activation. Adding a required field or changing stored calculations on nonempty storage requires a backfill; reject it with a clear explanation until backfills are implemented. On empty storage those changes may use a verified empty-table path under the publication write gate.

Do not implement dropping columns, type conversions, key reuse or silently deleting incompatible records. Archive disables the entity API and preserves storage. Archive is refused while an active reference depends on the entity. Removed permission definitions become inactive; historical IDs are never reassigned. Role/user/team removal must not silently change a draft's meaning: show unresolved references and reject publication until corrected. Runtime references to inactive recipients grant nothing.

### Publication protocol

MariaDB DDL can implicitly commit; ordinary transactions cannot atomically cover both schema changes and registry activation. Publication is therefore a small recoverable state machine, not a transaction wrapped around `ALTER TABLE`. [MariaDB implicit commits](https://mariadb.com/docs/server/reference/sql-statements/transactions/sql-statements-that-cause-an-implicit-commit)

1. **Validate and enqueue.** Check author authority, captured draft revision and active base version. Type-check expressions, ownership references, permissions, defaults, dependency cycles, quotas and adapter support. Save an immutable version and deterministic storage plan. Enforce one pending publication per entity.
2. **Claim.** Reuse the existing worker's leadership/recovery pattern. Claim with a fenced attempt identity. Acquire a pinned advisory lock in the actual business database for the entity; registry leadership alone does not serialize schema work there. Verify database and entity ownership markers before changing storage.
3. **Drain writes.** A normal data transaction holds a shared lock on the entity-state row while writing. The publisher obtains an exclusive lock, closes the gate and commits, allowing in-flight writes to drain without holding registry locks. New writes receive a retryable publication response. Multi-entity reference checks acquire state/record locks in a consistent order.
4. **Prepare storage.** Apply only the approved bounded operations and validate existing data where required. Record successful steps and physical layout hash. Set lock/statement deadlines; fail cleanly rather than wait indefinitely or fall back silently to an unexpectedly expensive table rebuild. Supported algorithms depend on the actual operation. [MariaDB ALTER TABLE](https://mariadb.com/docs/server/reference/sql-statements/data-definition/alter/alter-table)
5. **Activate.** Recheck author publication authority, dependency versions and base-version CAS. Lock the entity and referenced dependency rows in stable order, shared with archive/publication checks, so a dependency cannot disappear during activation. In one registry transaction set the active version, materialize permission/role projections, update security revisions, and append audit. A lost authority or changed dependency fails activation and requires an explicit retry after review.
6. **Open.** Mark the local state ready for the activated version and reopen its gate. Return published status only after this step is confirmed. Requests carrying a mismatched version retry with current metadata; they never guess which layout or rules are active.

Policy-only publication uses the same activation path with an empty DDL plan. Reads during preparation may use the old active version only while the compiler verifies that the prepared layout remains compatible. Otherwise return a retryable response. The registry/local activation gap is explicit and brief; temporary unavailability is preferable to using the wrong rules. No promise of zero-downtime schema changes is made.

On restart, compare the immutable plan, registry active version, local marker and fenced attempt before resuming. A stale worker cannot activate or reopen another attempt's gate. If activation already committed, finish opening that exact version. If it did not, restore the old version's gate only after proving compatibility; otherwise leave writes closed with an actionable failure. Never repair by dropping business data. A retry creates a new attempt for the same immutable snapshot, not a silently edited plan.

The UI exposes meaningful progress and a safe error, not SQL or credentials. Author edits can continue in a newer draft while the captured version publishes. Completion must not overwrite that newer draft.

## 9 Data runtime and query contract

### One typed query representation

Use a small JSON query document: selected fields, typed filter, ordered sort fields, limit and optional opaque cursor. Reuse the expression catalog for compatible operators, but accept only operators advertised for reads. No SQL fragments, arbitrary joins, scripts, raw regular expressions or unbounded relation traversal.

Compile in this order: resolve entity/version → resolve current actor/App authorization → validate requested fields/operators → add namespace and complete permission predicates → apply user filter → stable sort → keyset pagination. Use parameter placeholders for values and trusted generated mappings for identifiers. The record ID is the final sort tie-breaker. Define null ordering explicitly and include it in cursor comparisons.

Cursors are integrity-protected and bound to Tenant/Space/App/entity, version, projection/filter/sort and last sort values. Reauthorize every page. A cursor is neither a permission token nor a stable database snapshot; concurrent record changes can affect subsequent pages. Count is optional, explicitly requested and deadline-limited, using the identical authorized predicate. No unrestricted fallback or in-memory filtering after fetching an unauthorized page.

Related records use separately authorized batched reads initially; a reference picker also uses the target's normal data API. Avoid a generic join/GraphQL engine in this phase. Scoped authoring identity pickers and runtime reference pickers must not leak the global user directory.

### Writes and bulk operations

Single update/delete requires the record's expected revision. A hidden record returns not found before any revision detail is disclosed. Bulk operations are bounded, within one entity and one business database, and atomic: every item is authorized and validated, or all changes roll back. Reject duplicate target IDs and ambiguous repeated mutations within one batch. No unbounded update/delete-by-filter, implicit upsert or cross-Space transaction endpoint.

Create accepts a stable client-generated UUID. After a network timeout the client may reconcile through an authorized read; a repeated create with the same ID returns a generic conflict rather than overwriting data. The first SDK does not promise automatic exactly-once delivery. Writes return only accepted IDs and revisions; optional follow-up reads enforce normal visibility.

### Authorization freshness and resource bounds

Build a consistent current authorization snapshot from the registry for each request, then close that transaction before business-database I/O. Do not reuse `control.mutate()`'s Tenant-wide mutation lock for record traffic. An already-authorized bounded request may finish after a concurrent role revocation; requests beginning their snapshot after revocation commits must observe it. Storage/rule version changes additionally use the local publication gate. This is explicit request-snapshot semantics, not a distributed transaction across two databases.

Initially cache immutable compiled definitions by content hash only. Resolve active versions, identity state, role/team membership and App ceilings from the authoritative registry. Do not add a positive-permission TTL cache. Runtime SQL checks current local shares in the same business transaction/snapshot as the record operation. Add invalidation infrastructure only if measured load justifies it.

Reuse validated/TLS-aware database connection setup and the pinned driver. Maintain a bounded pool cache keyed by database-resource ID and credential generation, not one unbounded pool per App. Use idle eviction, per-resource connection limits, Tenant request limits, bounded queues, cancellation and query deadlines. A large Tenant must not exhaust all database connections or compiler capacity.

Initial configurable limits belong to one Go catalog: proposed read page default 50/max 200, bulk max 100, expression depth 8/max 64 nodes. Also bound request/JSON sizes, entity fields, rules, permission paths, memberships expanded into SQL, concurrent publications and schema-validation work. Exceeding a limit returns a specific error; never truncate authorization results. The UI and SDK consume the catalog. Confirm practical defaults against representative local fixtures before release; no unrelated database-version benchmark project.

## 10 Authoring authority and App authentication

### Management versus data authority

Add explicit `entity.manage` and `entity.publish` control actions to the existing Go catalog. Initially Tenant admins and Space managers hold both within their scope. App managers can consume permitted entity contracts and manage App-owned narrowing settings; they cannot publish Space definitions or change Space-owned ceilings.

Protect draft reads, validation, simulation and publication with authoring authority. The current platform read shortcut in `internal/control/auth.go` must not expose sensitive definitions, policy details, simulated identities or business data through a new `*.read` action. Platform operators retain operational health/status visibility only. Administrative roles are not implicitly data roles.

Authoring authority is deliberately powerful: its holder can publish a policy granting themselves data access. Ordinary read-only metadata, runtime capability checks and authoring simulation must remain separate endpoints and response shapes so this authority is never accidentally given to an App.

### Minimal App session boundary

Before external client Apps use the new data API, implement the previously planned App-bound session flow. Reuse the registry session infrastructure; do not introduce a second identity provider.

- A signed-in console user launches an App whose exact origin and Space ceiling were approved by a Space manager. The console's trusted launch broker validates the user/App and starts a short-lived, one-use exchange bound to parent session, Tenant, Space, App, origin generation, nonce and PKCE S256 challenge.
- Only the expected browser window/origin receives the exchange result. The App redeems the code with its verifier for an opaque bearer token; store only token hashes server-side. Token and code lifetimes come from the central security catalog. Consume codes atomically and reject replay.
- Keep the App bearer in memory. The SDK handles launch/exchange and reauthentication. App tokens cannot call management, authoring or simulation APIs; console cookies cannot call the external data API. Management keeps its existing CSRF protection.
- Data routes derive context from the validated token. Caller-supplied Tenant/Space/App IDs are never authority. Check current parent-session/account/membership/App state and origin generation; disabling any of them invalidates effective access.
- Require exact registered browser origins and CORS allowlists; never reflect arbitrary origins or enable credentialed wildcard CORS. Console cookies are host-only and production-secure. Untrusted Apps must run on separate hosts from the console and other Apps; different ports alone do not isolate cookies. Sandbox/browser packaging remains a separate App-runtime concern, but the test harness must exercise the actual isolation boundary.

Origin checks do not make a stolen bearer safe or stop an authorized malicious App from exporting data it can read. Space-approved ceilings are the exposure control. The minimum deliverable is the launch/exchange API, a tiny SDK and an isolated static test App; a general micro-app hosting/build system is outside this phase.

## 11 API surface

Use existing `/v1` management conventions and revision/error formats. Names below are the proposed contract, finalized in the first implementation milestone. Existing role/user/team APIs remain the only assignment/catalog APIs.

| Endpoint family | Purpose |
| --- | --- |
| `GET /v1/entity-capabilities` | Authorized authoring catalog: types, operators, constraints and limits |
| `GET/POST /v1/spaces/{space}/entities` | List entity summaries or create a draft |
| `GET/PATCH /v1/entities/{entity}` | Read/save a draft with its captured revision; archive through an explicit lifecycle operation |
| `POST /v1/entities/{entity}/validate` | Validate a submitted candidate document without persistence |
| `POST /v1/entities/{entity}/test` | Evaluate a submitted candidate and synthetic scenario; author-only |
| `POST /v1/entities/{entity}/publications` | Enqueue the captured draft revision and active base version |
| `GET /v1/entities/{entity}/publications/{version}` | Read status/diff/safe diagnostics; author-only |
| `PATCH /v1/apps/{app}/data-access` | Revision-guarded ceilings or narrowing settings, with field-level ownership checks |
| `POST /v1/apps/{app}/launch` | Console-authorized, CSRF-protected launch initiation |
| `POST /data/v1/session/exchange` | One-use launch exchange; no general login or management access |
| `GET /data/v1/schema` | Current actor/App-visible entity contracts and supported query capabilities |
| `POST /data/v1/entities/{key}/query` | Authorized filtering, selection, sorting, cursor pagination and optional count |
| `GET /data/v1/entities/{key}/records/{id}` | Authorized single record with explicit/default projection |
| `POST /data/v1/entities/{key}/records` | Create one record |
| `PATCH/DELETE /data/v1/entities/{key}/records/{id}` | Revision-checked update/delete |
| `POST /data/v1/entities/{key}/bulk` | Bounded atomic create/update/delete batch |
| `POST /data/v1/entities/{key}/capabilities` | Bounded advisory operation/field checks for known record IDs; no policy disclosure |
| `/data/v1/entities/{key}/records/{id}/shares` | Optional later share list/grant/revoke, requiring `acl.manage` |

Runtime metadata excludes hidden fields, policy bodies, role membership and ungranted entities. An App-author contract endpoint may expose the Space-approved schema to authorized authors independently of their personal record access; it must be scoped by that App's ceiling and cannot return policy internals. Capability results are UI hints, not proof for a subsequent write; mutations always authorize again.

Example read body, using the same typed expression representation as the authoring contract:

```json
{
  "select": ["name", "amount", "stage"],
  "filter": {"op":"eq","args":[{"field":"stage"},{"literal":"qualified"}]},
  "sort": [{"field":"amount","direction":"desc"}],
  "limit": 50,
  "include_count": false
}
```

Use structured errors with stable code, field/rule key where safe, request ID and retryability. Distinguish invalid input, revision conflict, unsupported draft change, publication in progress and resource limit. Never return raw SQL, credentials, hidden record values or inaccessible conflict targets. Permission/validation errors must not become a record-existence oracle. Sensitive diagnostics belong in access-controlled server logs with redaction.

Generate TypeScript record/input/query types and SDK declarations from the same published model. The SDK supplies request plumbing, App auth, revisions and typed errors; it does not implement a second permissions or rules engine. Generated artifacts identify their definition version; stale clients receive actionable compatibility errors for changes that affect their requests.

## 12 Implementation structure and current-code corrections

Keep one Go service and the existing worker. Introduce only modules with distinct responsibilities, not independent deployable services:

| Location | Changes |
| --- | --- |
| `internal/entity/` (new) | Typed definitions, capability/operator catalog, validation, expression evaluation, compilation, semantic diffs and generated contracts |
| `internal/data/` (new) | Authorized CRUD/query planning, MariaDB SQL adapter, write pipeline, bounded pools, local audit and optional shares |
| `internal/control/` | Entity management/publication handlers, App session exchange, registry persistence and worker integration; reuse existing auth/CSRF/revision/error plumbing |
| `internal/authz/catalog.go` | Authoring control actions and protected role permissions; retain control/data separation |
| `internal/control/auth.go` | Explicit authoring checks and management/data token separation; prevent platform read wildcard exposing authoring/data |
| `internal/control/roles.go` | Expose derived data-permission links through the canonical entity/App configuration, not a second editable policy model |
| `internal/control/worker.go` | Publication claims/recovery using the existing leader pattern; no long DDL under a registry Tenant lock |
| `internal/control/databases.go` | Separate runtime/schema credentials, resource generations, shared safe connection setup |
| `internal/control/migrations/` and `design/control_plane.sql` | Add entity/version schema and domain-safe extensions together; keep schema-parity validation passing |
| `web/admin/src/entity-designer/` (new) | Reusable Fields/Access/Rules/Test and review/publish components consuming backend catalogs |
| `web/admin/src/tenant-workspace.tsx` | Entities navigation and routing; compose the designer rather than embedding all its logic |
| `web/admin/src/console-forms.tsx` and shared controls | Remove duplicated fixed role UUIDs in favor of catalog lookup; reuse selectors, errors, revision handling and field controls |
| `packages/client/` (new) | Small TypeScript SDK and generated types; no generated App runner |

Keep evaluator/compiler interfaces concrete for MariaDB. A second storage adapter can implement the existing query/value contract later; do not build a plugin framework, universal SQL abstraction or hypothetical Mongo adapter now. MariaDB constraints, Go evaluation and generated validation are implementations of one typed semantic model with parity tests.

## 13 Implementation sequence and acceptance gates

Each milestone includes its API, tests and applicable UI. Do not deliver a backend-only foundation and postpone all authoring UX.

| Milestone | Work | Acceptance gate |
| --- | --- | --- |
| 1. Contract and registry | Finalize document/expression/query schemas and catalog; add registry tables/projections; authoring controls; draft CRUD and Entities/Fields UI | Tenant/Space scoping proven; agent and visual editor round-trip the same draft; incomplete drafts save with diagnostics, malformed/oversized documents fail, and invalid definitions cannot publish; no duplicated role IDs or field/policy definitions |
| 2. Rule/access authoring | Typed evaluator, complete-path ACL model, rule/permission editors, synthetic Test, semantic diff | Joe/Marketing scenarios and field-composition attacks pass; no real-data reads or writes through simulation; expression diagnostics match UI field/rule locations |
| 3. Publisher and storage | Ownership markers, schema credentials, local gate, deterministic plans, projections, worker recovery, publish UI | Create/publish a Deal entity on managed and registered storage; restart/failure tests preserve active definitions and data; unsupported changes remain drafts |
| 4. Secured vertical slice | App-bound launch/exchange, one static isolated App, metadata/SDK, single CRUD and authorized queries | Real user logs in, launches approved App, reads/creates/updates only eligible records; cross-Tenant/Space/App and management-token attacks fail |
| 5. Complete first release | Bulk, pagination/count, reference pickers, App ceilings/gates UI, pool/limit controls, accessible authoring and full regression suite | End-to-end tenant authoring and two-App reuse work without copied entities; bounded concurrent workloads and rollback behavior verified |
| 6. Optional sharing | Grant/revoke UI/API, share predicates/security revisions and ownership cleanup | Recipient and grantor semantics, App ceilings, security changes, concurrent revoke/update and inaccessible share roster tests pass |

Milestones 1–5 are the core delivery. Publication may initially be exercised by integration fixtures before the external data slice is ready; it must not imply that the data API is available. Record sharing is the only later optional milestone in this plan. App-private entities, general backfills and the App execution platform remain separate proposals.

Use the existing local MariaDB and pinned driver. Preserve the current working tree and development data. Update local schema normally; do not reset databases, change driver code or introduce a database-version compatibility project as incidental work.

## 14 Verification plan

### Contract and semantic tests

- Golden fixtures: manifest → validated model → generated JSON Schema/TypeScript → SQL plan. Draft round-trip and projection rebuild agree with the canonical document.
- Go evaluator versus SQL predicate parity: null/NOT, empty membership, case/trailing spaces, Unicode, dates and decimal boundaries. Reject ambiguous or unsupported nodes before execution.
- Defaults only on omission; PATCH missing versus null; cycle detection; exact rounding; calculation writes forbidden; transition `old/new` behavior; failed validation produces no mutation.

### Authorization and isolation tests

- User/role/team state across two Tenants, shared-database Spaces and multiple Apps. Same labels/keys never cross ownership boundaries. No global-admin implicit business reads.
- Complete permission paths: combine salary/name reads, mismatched App permission IDs and unrelated multi-field update grants; demonstrate no field/record leakage.
- Filtering/sorting/count/pagination/related-record lookup apply the same visibility. Default projections and hidden columns cannot leak through metadata/errors/capability checks.
- Owner/team transfer, team reviewer relationships, create defaults and references cannot grant unintended authority. Disabled membership/App/session takes effect for subsequent authorization snapshots.
- App tokens fail on management routes; cookies fail on data routes. Code replay, bad verifier, wrong origin/window, changed origin generation and revoked parent sessions fail.
- Synthetic Test never reads live records, bypasses management authority, returns a usable user token or performs a write.

### Storage, concurrency and recovery tests

- Test managed and registered databases through existing driver/configuration. Cross-Space unique/reference constraints, bounded connection reuse and credential rotation behave correctly.
- Atomic bulk rollback, optimistic revisions, duplicate IDs, simultaneous references/deletion and deterministic lock ordering; deadlocks are bounded/retryable, never partial success.
- Inject failure before/after each publication step: DDL success with lost worker, partial preparation, registry activation failure, process restart before gate opening, changed authority/dependency, stale attempt and newer edited draft.
- Verify no mismatched rule/storage version executes, failed publication never silently activates, and recovery never drops existing data. Exercise lock/statement timeouts and cancellation.
- Optional shares: stale security revision, permission removal, recipient state changes, grantor revocation, owner transfer, record deletion and concurrent grants/revokes.

### Browser and complete-flow tests

Superadmin creates Tenant/admin → admin creates Space → author defines Company and Deal → adds team reviewer access and the four rule categories → tests synthetic cases → reviews/publishes → two Apps inherit the same entity → real users see permitted data and receive useful validation messages. Repeat with an App ceiling restricting one of the Apps.

Cover keyboard navigation, errors, unsaved edits, concurrent revision conflict, reload during publishing, failed-publication retry, scoped pickers and no hidden-data persistence in browser storage. Re-run existing provisioning/account/password-policy/role/UI regression tests; this feature must not regress the current foundation.

Run Go unit/integration/race checks, schema parity, frontend type/build checks and the project's browser suite as appropriate to implementation. Record actual commands/results in `VERIFICATION.md`. Executed implementation checks are recorded in `VERIFICATION.md`; this checklist is not a substitute for those results.

## 15 Completion and review boundary

The core is complete when a Tenant author can describe an entity, configure its access/rules, test and publish it, and use its secured API from two Apps with one shared definition. Storage stays behind the scenes, policy/rule behavior is identical across single/bulk requests, and supported publication failures recover without data loss or accidental authorization.

This document records the approved implementation stage. The implemented scope is described in `FOUNDATION.md` and `ENTITY_RUNTIME.md`. This plan governs entity/ACL/rule implementation and refines the earlier entity-language/publication details in `SPACE_AUTHORIZATION_PLAN.md` and `CONCEPT.md`; their hierarchy and global identity/role principles remain intact. Deferred features remain separate proposals. Published definitions and current App ceilings determine data access.
