# Historical provisioning schema validation

The results below apply to the pre-foundation schema at checkpoint `fa31720`. They do not describe the current role model. Current schema checks are maintained in the Go foundation/entity/MCP/integration suites; see [VERIFICATION.md](../VERIFICATION.md) and [DEVELOPMENT.md](../DEVELOPMENT.md). The ordered migrations are authoritative. `python3 design/validate_schema.py --write` regenerates the SQL document, `--check` checks parity offline, and the default invocation also runs the current Go foundation checks on the existing local development server. That focused test does not replace entity/MCP integration tests. The historical results below remain unchanged.

# Control Plane Schema Validation

**PASS** — 2026-11-11T23:28:17+00:00

Engine: 12.3.3-MariaDB.

The validator started its own temporary MariaDB with TCP disabled, applied the design DDL,
and removed its data directory after stopping the private server. No existing server was used.

These checks cover the DDL, authorization relationship constraints, conditional writes and SQL advisory lock primitives. They do not validate the future
Go authorization decisions, handlers, OIDC flows, encryption, policy/quota enforcement or provisioning side effects.

65 completed checks:

- DDL applies to an empty database and creates all 17 tables
- OIDC subjects compare case-sensitively
- Duplicate provider identity is rejected
- A plan cannot default to a forbidden layout
- Managed targets require provisioning credentials
- All four layout and management-method combinations are representable
- A resource requires an existing Tenant
- Physical names differing only by case cannot be registered twice
- Managed resources require an ownership token
- Unknown provisioning states are rejected
- Two Spaces can share one tenant database
- A Space cannot bind to another tenant's database
- A dedicated database cannot bind to a second Space
- A Space cannot mislabel a shared database as dedicated
- The tenant default can point to its own shared database
- The tenant default cannot point to a dedicated database
- The tenant default cannot point to another tenant's database
- The same App slug is allowed in different Spaces
- An App cannot claim another tenant's Space
- Compute preference rejects unknown values
- A duplicate create ID cannot allocate another App
- An App slug cannot be duplicated within its Space
- The same pinned artifact can be referenced from different Apps
- An installation can be inventoried before an artifact exists
- A micro-app cannot claim another tenant's App
- A revision-conditional UPDATE rejects a stale writer
- Sessions cannot reference an unknown user
- Session expiry must follow creation
- Audit details must be a JSON object
- Shared built-in roles are seeded with separate platform and Tenant actions
- Different Tenants can own teams with the same slug
- A team cannot include a user outside its Tenant membership
- A team membership cannot reference another Tenant's team
- Different Tenants can define roles with the same code in shared tables
- A custom Tenant role cannot claim platform assignment kind
- A custom role cannot falsify its owner namespace
- A Tenant role cannot include platform permissions
- Changing the permission mapping kind cannot elevate a Tenant role
- A global user can receive the protected platform role without a Tenant binding
- One shared Tenant administrator definition can be assigned in multiple Tenants
- A team can receive its Tenant's custom role within one Space
- A user can receive a direct App permission through the same grants table
- A team can receive a direct Space permission through the same grants table
- A Tenant grant cannot include a user outside that Tenant
- A Tenant grant cannot include another Tenant's team
- A Tenant grant cannot include another Tenant's Space
- A Tenant grant cannot include another Tenant's App
- A custom role cannot be assigned outside its owner Tenant
- Falsifying the role owner cannot bypass the custom-role boundary
- A platform role cannot be assigned as a Tenant role
- A direct Tenant grant cannot contain a platform action
- Changing a grant to platform kind cannot retain Tenant scope
- A grant must select exactly one user or team
- A grant cannot omit its user or team
- A grant cannot assign both a role and a direct permission
- A grant cannot omit both role and permission
- A grant cannot specify both Space and App scope
- Platform grants cannot use Tenant teams
- An unknown action code cannot enter a grant
- The worker leadership lock survives COMMIT and excludes a second connection
- Worker leadership becomes available when its owning connection ends
- The resource advisory lock excludes overlap and releases on disconnect
- A bound database cannot be deleted from inventory
- A referenced target cannot be deleted
- Deleting a Tenant cannot cascade through retained resources
