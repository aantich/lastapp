# Business entities, access and rules

The entity stage extends the existing Go service and React console. A Space owns
one published definition per entity; Apps consume it with their own approved
access ceiling. Authors describe entities and rules. Lastapp creates and manages
their storage. This implementation includes the optional record-sharing milestone.

## Try the designer

Sign in as a Tenant administrator or Space manager, open a Space, and select
**Entities**. Create a blank entity or use the Company, Contact or Deal template.
Templates start without access grants. Add fields, choose the default read
projection, and configure Access. Use a Space role or an owner/team relationship;
Tenant administration itself grants no business-record access.

**Rules** configures defaults, stored calculations, resulting-record validation
and allowed changes. **Test** evaluates synthetic values using the real backend
evaluator and current same-Tenant relationships. It never fetches or writes a
business record. Live uniqueness, referenced-record existence and actual shares
are explicitly outside that simulation.

Save incomplete drafts, then use **Review changes → Validate → Publish**. The
published version remains separate from the draft. The UI preserves the selected
entity across reloads and warns before abandoning unsaved edits. Publication
status stays pending until the generated storage is ready to serve that version.
Archive preserves data and disables the entity API; references can prevent it.

Managed databases use their target's schema credentials. For a registered database,
a Tenant administrator supplies separate **Schema credentials** under
**Settings → Databases**. Entity authors never enter SQL, table mappings or database
credentials. Runtime connections retain their separate CRUD credentials.

## Connect a browser App

Open an App's **Settings → App data access**. A Space manager approves its HTTPS
origin and either full inheritance or explicit permission gates. App managers can
add their own narrowing conditions and App-role requirements. Each App uses a
separate host from the console and other Apps; loopback HTTP origins are accepted
in development. New Apps start without approved data access.

The small TypeScript SDK is in [packages/client](packages/client). Build it with
`make client`. The [isolated example](examples/data-app/README.md) shows the full
flow using real data. It uses a static server, an approval popup, PKCE and an
opaque App token kept in memory. No App token is stored in localStorage. The
backend rechecks the parent console session, membership, App state, origin
approval and current permissions. App tokens cannot call management APIs.

```ts
import { Lastapp } from '@lastapp/client';

const client = new Lastapp('https://api.example.com', appID);
await client.connect();
const schema = await client.schema();
const deals = await client.query('deal', {
  select: ['name', 'amount'],
  filter: { op: 'eq', args: [{ field: 'stage' }, { literal: 'qualified' }] },
  sort: [{ field: 'amount', direction: 'desc' }],
});
```

Apps in the same Space resolve the same entity definitions and database binding.
Their origins, ceilings, restrictions and sessions remain App-specific. The SDK
also supports create/get/update/delete, bounded atomic bulk requests, advisory
record capabilities, scoped user/team reference options, sharing/revocation and
authorized share-permission/recipient discovery. Errors retain rule identity and
optional field placement.
Updates and deletes use the current record revision. A successful write returns
IDs/revisions; reading values requires a separate authorized read.

## Contracts and enforcement

`GET /v1/entity-capabilities` publishes the authoritative type/operator/limit
catalog and templates. Entity drafts use one typed JSON document; generated SQL,
JSON Schema, TypeScript declarations and permission links are projections. The
console also consumes the fixed role catalog instead of maintaining role UUIDs.

The authoritative format is `entity.Definition` in `internal/entity/model.go`,
with expression/policy semantics and limits in the same package. JSON Schema is
generated from that model; it is not a separate authoring language or independent
source of rules. Inspect capabilities and generated contracts rather than copying
type/operator lists or inventing storage mappings.

Management routes are in `internal/control/entities.go`. Published data routes
start at `/data/v1`; their context comes exclusively from the App token.
`GET /data/v1/schema` returns only the actor/App-visible contracts, including
stable entity IDs, versions, input/output schemas and generated client types.
App authors have a separate ceiling-scoped contract endpoint. Neither surface
returns executable policy bodies or hidden calculation/default expressions.

### API map

Console-session management endpoints and App bearer endpoints are separate.
Management writes use Origin/CSRF and guarded revisions; App record updates and
deletes use the record revision through the SDK. `{key}` is the published entity
API key, while management `{entity}` is its registry UUID.

| Endpoint group | Purpose |
| --- | --- |
| `GET /v1/entity-capabilities` | Authoritative types, operators, limits and templates |
| `GET/POST /v1/spaces/{space}/entities` | List/create Space definitions |
| `GET/PATCH /v1/entities/{entity}` | Inspect/replace a draft or change lifecycle |
| `POST /v1/entities/{entity}/validate` and `/test` | Shared validator and synthetic evaluator |
| `POST /v1/entities/{entity}/publications`; `GET .../publications/{version}` | Queue and inspect exact-version publication |
| `GET/PATCH /v1/apps/{app}/data-access` | Approved origins, inheritance, ceilings and restrictions |
| `GET /v1/apps/{app}/entity-contracts` | Author-visible published App contracts |
| `POST /data/v1/session/exchange`; `GET /data/v1/schema` | App PKCE exchange and actor-visible contracts |
| `POST /data/v1/entities/{key}/query` | Structured selection/filter/sort/cursor reads |
| `POST .../records`; `GET/PATCH/DELETE .../records/{id}` | Single-record CRUD |
| `POST .../bulk` and `.../capabilities` | Atomic bounded mutations and advisory per-record capabilities |
| `GET .../reference-options`; `GET/POST/DELETE .../records/{id}/shares` | Authorized references and record sharing |

The exact SDK request/response types live in
[packages/client/src/index.ts](packages/client/src/index.ts); dispatch lives in
[entity_data.go](internal/control/entity_data.go). Do not hand-maintain another
data API client or treat management pagination as the data query contract.

Management MCP exposes these same authoring operations through typed tools;
see [MCP.md](MCP.md). Discover entity inventories, inspect full definitions and
published contracts, then save/check/publish with current revisions and stable
request keys. MCP has no business-record CRUD. Large checks use the shared
`validate?view=diagnostics` path; full/generated contract sections are inspected
separately, without a second validation engine.

Read authorization retains each permission's fields with its own record predicate.
Selected, filtered and sorted fields all contribute to row visibility before count,
sorting or cursor pagination. A multi-field update needs one complete permission
path for the entire submitted field set. Update scope holds on both old and new
records; change conditions explicitly describe transitions. A caller must pass the
complete update permission before learning revision conflicts. Ownership changes
also require access-management authority and clear shares transactionally.

Arithmetic uses exact decimals and stored calculated fields. Conditions compare
stored values; arithmetic that could fail inside a SQL predicate is rejected.
Decimal literal casts preserve the full supported precision, with matching Go/SQL
regressions. Null behavior, case/trailing spaces and cursor ordering have explicit
semantics. Defaults apply only to omitted create values. Calculated/system fields
are never accepted as client inputs. The backend applies the same preparation,
constraints and rules to single and bulk writes.

Record sharing delegates an existing role-activated permission, retaining its
predicate, fields and App ceiling. A grantor needs that permission and an explicit
`acl.manage` delegation grant. Security changes invalidate older shares; label
changes preserve them. Global identities/roles remain in the registry; indexed
record grants and mutation audit live beside the business records. Disabling
memberships/teams or revoking the parent session affects subsequent requests.
Already-authorized bounded requests use their captured authorization snapshot.

## Publication and supported limits

Two registry tables store entity drafts and immutable publication versions/jobs.
The existing worker prepares owned storage under a pinned entity lock and local
write gate, activates registry projections transactionally, then opens the exact
version. Recovery can finish an activation interrupted before opening. Failed
preparation restores the previous gate only when its physical layout is verified
compatible. No recovery path drops business data. Publication attempts are fenced,
Tenant queues are bounded and work rotates across Tenants.

Supported changes include new entities, nullable additions, labels, policies,
defaults and compatible rule/constraint changes. Existing data is checked under
the write gate when resulting-record constraints change, in bounded keyset batches
with a record cap and deadline. Policy, label and default changes do not scan
business records. Backfills, destructive column changes, published storage/index
changes, arbitrary joins, nested JSON queries, cross-Space references, side effects
and generated backend code remain outside this release. Some DDL can require a
retry or a supported alternative; the system does not silently choose an
unbounded rebuild. Externally supplied database credentials can bypass the API,
so direct external writes do not carry Lastapp's enforcement guarantees.

Connection pools and compiled immutable models are bounded. Authorization is
resolved afresh; there is no positive-permission TTL cache. Requests, concurrency,
expressions, SQL permission expansion, pages, bulk operations and JSON values are
bounded by the Go catalogs. Wide projections reduce the default page size;
explicit pages exceeding the response budget are rejected. Tenant concurrency
limits reserve capacity for other Tenants, and opening a slow database does not
hold the shared pool-cache lock. Warm model-cache hits do not reload definition
documents from the registry. The current implementation uses the pinned MariaDB
driver and does not add another database engine or a database-version matrix.
