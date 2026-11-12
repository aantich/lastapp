# Lastapp identity roles and scopes foundation

**Implemented 12 November 2026.** This stage provides centralized identity, memberships, teams, protected administrative roles, custom Space/App business roles and scoped assignments, with matching management APIs and React console screens. Business-object ACL definitions, SQL filtering, record sharing and the App SDK follow after their entity/API contract is designed.

The Tenant workspace now includes email/password onboarding, forced initial-password replacement, Home, Spaces, People, Activity and Settings. Apps are managed inside Spaces; micro-app registrations inside Apps. The Platform workspace is operational and cannot author Tenant resources.

## Shared model

One registry stores users, Tenant memberships, teams, role definitions and assignments. Tenant creation adds rows, not another authorization database. Spaces retain their database binding; Apps retain their Space ancestry. Those bindings cannot be changed by an ordinary update.

| Role category | Definitions | Recipients | Assignment scope |
| --- | --- | --- | --- |
| Platform | Fixed | User | Platform |
| Tenant | Fixed | Active Tenant member | Tenant |
| Team | Fixed | Active member of that team | Team |
| Space | Protected manager and custom business roles | Manager: user; business: user or team | Owning Space |
| App | Protected manager and custom business roles | Manager: user; business: user or team | Owning App |

The fixed catalog is `platform.superadmin`, `tenant.admin`, `team.admin`, `team.reviewer`, `space.manager` and `app.manager`. Its IDs and allowed control actions have one source in `internal/authz/catalog.go`; trusted schema initialization persists the catalog in the registry. APIs cannot change fixed definitions. Control permissions have stable UUIDs, and role-permission joins enforce the control/data distinction.

Custom business roles currently define named, scoped relationships. They cannot contain control permissions, and do not yet authorize entity operations. A role named “Tenant administrator” inside a Space remains a business role. Names never confer authority.

Each assignment has exactly one scope. Joe → Team Reviewer → Marketing is different from Marketing team → Sales reviewer → CRM Space. The first describes Joe’s role within one team. The second makes active Marketing members recipients of a Space business role. Teams are Tenant structures, not parents of Spaces.

## Assignment authority

| Caller | May assign or revoke |
| --- | --- |
| Platform superadmin | Platform roles and Tenant administration, including recovery |
| Tenant administrator | Tenant/team roles and Space/App managers within the Tenant |
| Team administrator | Fixed team roles within their own team; manage that team’s members |
| Space manager | Their Space business roles and App managers within that Space |
| App manager | Their App business roles |

Protected administrative roles require individual users. A team cannot hold a manager role. This prevents a team membership editor from indirectly appointing themselves a Space/App manager.

Platform accounts and Tenant/team administrative accounts are separate. Creating a Tenant or assigning an initial Space/App manager rejects a platform account. Granting platform administration to a user with Tenant/team control roles is rejected until those roles are removed. Recipient user locks serialize these checks. Platform administrators manage Tenant lifecycle, initial accounts and Tenant-admin recovery, and inspect operational metadata; they cannot create/update Spaces, Apps, micro-apps, teams or custom business roles. This restriction is enforced by the API as well as the console. The last-Tenant-admin check counts accounts that can actually administer the Tenant.

Space/App creation requires `initial_manager_user_id`, an active member of the Tenant. The manager assignment and creation audit commit together. Creating or managing a scope grants no business data role.

## APIs

These management endpoints use local-password or OIDC console sessions. Mutations require exact Origin and CSRF checks. Generated App bearer authentication and business-data endpoints are outside this stage.

`POST /auth/login/password` accepts email/password with an exact allowed Origin, generic invalid-credential responses and bounded per-client/per-email throttling. `trusted_proxies` defaults to empty; only explicitly configured proxy peers may supply `X-Forwarded-For`, traversed from right to left to the first untrusted hop. Local credential rows are global, reference `users`, and store salted password verifiers only. Local and OIDC identities are never implicitly linked by email. Initial-password sessions can call only `/v1/me`, password replacement and logout; ordinary management APIs reject them. `POST /auth/password` requires the current password and CSRF, locks the user/credential revision, revokes existing sessions and issues a new session/CSRF pair.

`POST /admin/v1/tenants` accepts either `initial_admin_user_id` or `initial_admin: {display_name,email,password}`. Account, Tenant, membership and initial role commit atomically. For another Tenant, an existing local account may be referenced with `initial_admin: {email}`; supplying a password for an existing email returns a conflict and never resets it. `POST /admin/v1/users` creates a global email/password account without granting memberships or roles. Both creation paths require platform administration. Until verified invitations are implemented, Tenant administrators cannot create global credentials: membership creation accepts an existing `user_id` or `account: {email}` only. Platform administrators may also supply a new account while creating a membership; account and membership commit atomically. Account-creation authority is rechecked inside the transaction. Trusted local platform bootstrap supports `-email` and `-password-file`; OIDC is optional.

Tenant Home uses `GET /v1/tenants/{t}/overview` for scoped resource counts, recent Apps, provisioning attention and plan limits. `PATCH /v1/tenants/{t}` lets a Tenant administrator update the company name with `If-Match`; subscription/lifecycle changes remain platform operations. Space/App detail endpoints load their stored ancestry and scoped metadata.

| Endpoint | Behavior |
| --- | --- |
| `GET /v1/roles/catalog?scope_kind=team` | Fixed catalog, optionally filtered by category |
| `GET /v1/tenants/{t}/capabilities` | Current control actions with Tenant/team/Space/App scope information |
| `GET /v1/tenants/{t}/my-role-assignments` | Paginated effective assignments for the current active Tenant member |
| `/v1/tenants/{t}/memberships` | List/add Tenant members; PATCH a user’s membership to enable/disable it |
| `/v1/tenants/{t}/teams` | List/create teams; GET/PATCH a team; list/add/PATCH/remove its members |
| `{scope}/roles` | List definitions; create/edit custom business roles only at Space/App scope |
| `{scope}/role-assignments` | List/create assignments; DELETE an assignment to revoke it |

The assignment/definition scope prefixes are:

```text
/admin/v1                                      platform assignments
/v1/tenants/{t}                                Tenant
/v1/tenants/{t}/teams/{team}                    team
/v1/tenants/{t}/spaces/{space}                  Space
/v1/tenants/{t}/apps/{app}                      App
```

The route selects the scope; assignment payloads cannot provide a different scope. App ancestry is loaded from the registry. Team/Tenant role definitions are fixed, so their creation/edit routes do not exist. Platform role definitions use the catalog endpoint.

Create a Space business role:

```json
{
  "id": "70000000-0000-4000-8000-000000000010",
  "code": "reviewer",
  "display_name": "Reviewer",
  "state": "active"
}
```

POST it to `/v1/tenants/{t}/spaces/{s}/roles`. Then POST an assignment to that same scope’s `/role-assignments`:

```json
{
  "id": "80000000-0000-4000-8000-000000000010",
  "group_id": "60000000-0000-4000-8000-000000000001",
  "role_id": "70000000-0000-4000-8000-000000000010"
}
```

Use `user_id` instead of `group_id` for a user recipient. To assign Joe Team Reviewer, use the team’s route, Joe’s `user_id`, and fixed role ID `00000000-0000-4000-8000-000000000004`. SQL and service validation reject a wrong Tenant, role owner/category, missing team membership, combined scope or team recipient for an administrative role.

List endpoints support `page`, `limit` and `q`; the default limit is 100 and maximum is 200. Authoritative scope predicates apply before pagination/search. Scope role authoring is bounded to 200 custom roles per Space/App, with additional Tenant metadata quotas. The caller assignment endpoint is paged too; its results are relationships, not a business-data permission decision.

Custom role/team/user changes and assignment deletion require `If-Match` for that resource’s current revision. Tenant membership PATCH uses its membership revision. Team member add/change/removal uses the containing team’s revision, obtained from GET team; membership changes increment it. Stale revisions return 412 and missing required revisions return 428. Unknown request fields, including direct `permission_code` grants and administrative `permissions` on custom roles, are rejected. The former `/access-grants` and `/groups` API routes are removed.

The console retains the revision captured when a team editor opens. Background list refreshes cannot advance that form’s revision and silently overwrite another editor’s changes, including a team disable.

## Revocation and safety

Authorization reads current user, membership, team, role and assignment state; it does not trust a browser role list or retain positive permission decisions. A business role never contributes control actions. Unknown actions fail closed even for platform administrators. Space/App managers can operate only in their authorized scopes; an App manager cannot appoint another App manager or change a sibling App.

Per-user request limits apply before Tenant lookups. Only active Tenant members or platform administrators may allocate or spend that Tenant’s shared request budget; requests from outsiders or disabled members do not affect it. Endpoint authorization still checks each requested action and scope.

Disabling a user, Tenant membership, team, team membership or custom role removes the relevant effective assignments. Re-enabling preserves them. Removing team membership deletes its team-scoped user assignments in the same transaction; rejoining does not restore those removed assignments. Team-recipient business roles apply again to any current active member, as intended. Tenant membership is disable-only.

The last active Tenant administrator and platform superadmin are protected under serialized control mutations. Global user disable also checks affected Tenant administrators and deletes the user’s sessions. Recipient state checks lock user rows so concurrent user disable cannot race an assignment/membership creation into an invalid last-admin state. Deadlocks produce a retryable transaction error rather than partial changes.

Composite foreign keys and explicit non-null scope checks protect ancestry, recipients, team membership and role ownership. Role/permission domains prevent data roles acquiring control actions through the SQL join table. Equivalent assignments have a canonical unique key. Security mutations include safe before/after audit details; credential values are excluded. Runtime registry credentials cannot update/delete audit history.

## Console and local demonstration

The Tenant sidebar is **Home, Spaces, People, Activity, Settings**. Home links to **View all Apps**, a scoped, searchable and paginated directory independent of parent-Space visibility. People adds existing accounts by email or user ID and contains Members, Teams and fixed Tenant roles as permitted by the caller. Platform **Users → Create account** provisions new global identities. A Space contains Overview, Apps, Database, Access and Settings; an App contains Overview, Micro-apps, Access and Settings. Scoped Access panels reuse the same role/assignment editor without a second hierarchy selector. Refreshes preserve open editors and captured revisions. Shared forms, themes, paging/search and mobile navigation serve both workspaces. Unimplemented entity/rule editors, integrations, design libraries and runtime launchers are not exposed.

The shared API client distinguishes an expired/revoked session (`401 unauthenticated`) from incorrect login/current-password credentials (`401 invalid_credentials`). Expiry clears workspace state and shows login; already-expired logout also succeeds locally. Request generations prevent old-session failures from invalidating a newly authenticated session.

Platform **Access control** manages platform grants or a selected Tenant's fixed administrator assignments. Platform Tenant details show operational counts and subscription/status; authoring screens and actions are absent. Team screens distinguish membership from team roles and support disable/enable/removal. Space/App business roles cannot contain administrative actions; there is no custom Tenant role editor or direct permission-grant control.

`make dev` initializes a fresh development registry named `lastapp_foundation`. The previous `lastapp_control` database is retained, and the previous configuration is saved privately before switching registries. No deployed installation or legacy grant conversion is involved.

The development-only **Northstar demo** contains Marketing/Engineering teams, a CRM Space backed by a real managed MariaDB database, and Deal review/Forecasting Apps. Joe has Team Reviewer on Marketing and inherits Sales reviewer through Marketing’s Space assignment. The Tenant administrator manages the example; the platform operator can inspect it. Choose **Joe (Marketing Reviewer)** in the local issuer to inspect his own role context through the API. All development accounts use the existing local password.

The demo is created once. Restarting does not restore a role an administrator revoked. Its business roles do not yet grant CRM row access because generic business entities and their ACL runtime are deferred. The [broader Space authorization plan](SPACE_AUTHORIZATION_PLAN.md) records that next-stage direction; [VERIFICATION.md](VERIFICATION.md) records executed checks.
