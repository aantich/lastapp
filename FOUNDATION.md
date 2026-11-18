# Lastapp identity roles and scopes foundation

**Implemented; current guide updated 18 November 2026.** The foundation provides centralized identity, memberships, teams, protected administrative roles, custom Space/App business roles and scoped assignments, with matching management APIs and React console screens. Business-object ACLs, SQL filtering, record sharing and the App SDK are also implemented; see [the entity runtime guide](ENTITY_RUNTIME.md). The [MCP guide](MCP.md) covers the implemented agent delegation and management transport.

The Tenant workspace now includes email/password onboarding, forced initial-password replacement, Apps, Spaces, People, Activity and Settings. Apps are managed inside Spaces; micro-app registrations inside Apps. The Platform workspace is operational and cannot author Tenant resources.

[Business entities, ACLs and rules](ENTITY_DESIGN_IMPLEMENTATION_PLAN.md) now build on this foundation. The [runtime guide](ENTITY_RUNTIME.md) describes the implemented designer, publishing, data APIs and App boundary.

## Shared model

DRY is the primary engineering principle: each rule and policy has one authoritative source. Password policy lives in Go and is published through `GET /v1/config`; every account-creation and password-change form consumes that contract through shared UI controls. The console refreshes the contract when it regains focus and after sign-in. No browser password-limit constants or fallback policy are maintained.

One registry stores users, Tenant memberships, teams, role definitions and assignments. Tenant creation adds rows, not another authorization database. Spaces retain their database binding; Apps retain their Space ancestry. Those bindings cannot be changed by an ordinary update.

| Role category | Definitions | Recipients | Assignment scope |
| --- | --- | --- | --- |
| Platform | Fixed | User | Platform |
| Tenant | Fixed | Active Tenant member | Tenant |
| Team | Fixed | Active member of that team | Team |
| Space | Protected manager and custom business roles | Manager: user; business: user or team | Owning Space |
| App | Protected manager and custom business roles | Manager: user; business: user or team | Owning App |

The fixed catalog is `platform.superadmin`, `tenant.admin`, `team.admin`, `team.reviewer`, `space.manager` and `app.manager`. Its IDs and allowed control actions have one source in `internal/authz/catalog.go`; trusted schema initialization persists the catalog in the registry. APIs cannot change fixed definitions. Control permissions have stable UUIDs, and role-permission joins enforce the control/data distinction.

Custom business roles define named, scoped relationships. They cannot contain control permissions. Published entity definitions attach Space data permissions to them; App-role gates can narrow those same Space permissions. A role named “Tenant administrator” inside a Space remains a business role. Names never confer authority.

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

These HTTP management endpoints use local-password or OIDC console sessions. Mutations require exact Origin and CSRF checks. App bearer authentication and business-data endpoints are documented separately in ENTITY_RUNTIME.md. MCP authenticates an explicitly delegated user and invokes these same validated Go operations in process; it does not use console cookies or bypass policy. See MCP.md and the API boundary map in DEVELOPMENT.md.

`POST /auth/login/password` accepts email/password with an exact allowed Origin, generic invalid-credential responses and bounded per-client/per-email throttling. `trusted_proxies` defaults to empty; only explicitly configured proxy peers may supply `X-Forwarded-For`, traversed from right to left to the first untrusted hop. Local credential rows are global, reference `users`, and store salted password verifiers only. Local and OIDC identities are never implicitly linked by email. Initial-password sessions can call only `/v1/me`, password replacement and logout; ordinary management APIs reject them. `POST /auth/password` requires the current password and CSRF, locks the user/credential revision, revokes existing sessions and agent delegations, and issues a new session/CSRF pair. Console sessions, password/OIDC login, CSRF/Origin checks, logout and password change are implemented by `session.Manager` (`internal/session`); the password policy and the single bound on concurrent password derivations, shared by login, password change and every account-creation path, are in `internal/membership/accounts.go`.

`POST /admin/v1/tenants` accepts either `initial_admin_user_id` or `initial_admin: {display_name,email,password}`. Account, Tenant, membership and initial role commit atomically. For another Tenant, an existing local account may be referenced with `initial_admin: {email}`; supplying a password for an existing email returns a conflict and never resets it. `POST /admin/v1/users` creates a global email/password account without granting memberships or roles. Both creation paths require platform administration. Until verified invitations are implemented, Tenant administrators cannot create global credentials: membership creation accepts an existing `user_id` or `account: {email}` only. Platform administrators may also supply a new account while creating a membership; account and membership commit atomically. Account-creation authority is rechecked inside the transaction. Trusted local platform bootstrap supports `-email` and `-password-file`; OIDC is optional.

The Tenant workspace uses `GET /v1/tenants/{t}/overview` for scoped resource counts, defaults, provisioning attention and plan limits. Its main Apps collection comes from the paginated `/apps` endpoint. `PATCH /v1/tenants/{t}` lets a Tenant administrator update the company name with `If-Match`; subscription/lifecycle changes remain platform operations. Space/App detail endpoints load their stored ancestry and scoped metadata.

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

Disabling a user, Tenant membership, team, team membership or custom role removes the relevant effective assignments. Re-enabling preserves them. Removing team membership deletes its team-scoped user assignments in the same transaction; rejoining does not restore those removed assignments. Team-recipient business roles apply again to any current active member, as intended. Tenant membership is disable-only. Authorized administrators can revoke existing roles while the recipient account, membership or team is disabled, without first restoring access. New assignments still require active recipients; re-enabling access preserves only grants that remain.

Agent delegations are a separate ceiling over those roles. User or Tenant-membership disable explicitly revokes the affected agent grants; re-enabling the account/membership does not restore those revoked connections. Password replacement also revokes delegations. Team/role changes are checked through current ordinary authority, not a copied agent role list. Ordinary browser logout does not revoke agent connections. See MCP.md for consent, token rotation and queued-job behavior.

The last active Tenant administrator and platform superadmin are protected under serialized control mutations. Global user disable also checks affected Tenant administrators and deletes the user’s sessions. Recipient state checks lock user rows so concurrent user disable cannot race an assignment/membership creation into an invalid last-admin state. Deadlocks produce a retryable transaction error rather than partial changes.

Composite foreign keys and explicit non-null scope checks protect ancestry, recipients, team membership and role ownership. Role/permission domains prevent data roles acquiring control actions through the SQL join table. Equivalent assignments have a canonical unique key. Security mutations include safe before/after audit details; credential values are excluded. Runtime registry credentials cannot update/delete audit history.

## Console and local demonstration

Tenant navigation is **Home, Apps, Entities, Access, Agents, Activity** plus the sidebar's **Spaces** list, filtered by the capabilities the backend reports and defined once in `tenant/navigation.ts` (sidebar, page tabs and command palette). Home is the landing page; Apps is the complete, scoped directory. Space tabs are **Apps, Development** (for workspace managers, on production Spaces), **Entities, Access, Settings**; App tabs are **Micro-apps, Releases** (for release managers), **Access, Settings**. Access (Tenant people) tabs are **Members, Teams**; Tenant Settings (account menu → Tenant settings) tabs are **General, Databases, Agent access**; Entities opens the current or first Space's entities and Agents the **Agent connections** page (the Tenant's for a security manager, otherwise the person's own). Space database details are in Settings. The sidebar holds the Tenant switcher (its section label), **Search or run…** (⌘K / Ctrl+K command palette) and the account menu (**Tenant settings, My account, Appearance, Sign out**); the top bar holds breadcrumbs, search and **Connect agent**; the sidebar collapses to an icon rail and becomes a drawer on narrow screens. Access combines Tenant administrator checkboxes with membership rows; Teams opens members and their fixed Administrator/Reviewer roles together. Space Access is headed as the Space's access and shows a read-only role matrix for one entity (roles and relationships named by its permission paths × operations; published state, draft changes marked; a cell opens the entity), then **Assignments** and **Roles**; App Access shows Assignments and Roles. **Remove** asks for confirmation. Searchable controls use names and internal paging, without user-entered UUIDs. Forms generate slugs/codes and default managers/database choices; Advanced contains technical overrides. Space/App Access lists share the workspace refresh counter, including the background poll, while preserving open editors and their captured revisions. App compute preferences are displayed under Advanced and editable through the same field definition used at creation. Existing dedicated preferences can be retained after a plan change, or changed to shared; new dedicated preferences require plan support. Shared forms, themes, collections, shell and URL navigation serve both workspaces. The entity/rule designer and App access settings are implemented; generated App hosting, connectors and the shared micro-app design library are not.

The shared API client distinguishes an expired/revoked session (`401 unauthenticated`) from incorrect login/current-password credentials (`401 invalid_credentials`). Expiry clears workspace state and shows login; already-expired logout also succeeds locally. Request generations prevent old-session failures from invalidating a newly authenticated session.

Platform navigation is **Tenants, Users, Activity, Settings**; Settings has **Database servers** and **Plans**. Tenant details contain **Overview, Administrators, Settings**, without Tenant authoring actions. Removing a Tenant administrator and disabling an account ask for confirmation. Sign-in and the first-login password step are one centred card; **My account** shows the profile beside **Change password** (or a single sign-on note), with Agent access below. **Administrators → Add administrator** submits `POST /admin/v1/tenants/{t}/administrators` with either `{user_id}` or `{account:{display_name,email,password}}`. Existing local accounts may also be selected with `{account:{email}}`; credentials cannot be replaced. The operation authorizes platform administration before password hashing and again inside the transaction; account creation, membership creation/reactivation and the fixed administrator grant commit together, with quotas and safe audit events. Duplicate grants are idempotent. Disabled global accounts and platform accounts are rejected. `GET` of the same route provides the paged administrator list. Revocation uses the existing revision-guarded Tenant role-assignment DELETE and last-admin checks.

Users has platform administrator checkboxes and **Add to Tenant** for active non-platform accounts. This task selects a Tenant by name and posts the selected user ID to the existing membership API, adding an ordinary member without an administrator grant. It supports local and SSO identities without linking accounts by email or exposing the global directory to Tenant administrators. Database servers and plans are under Settings. Scope-authorized `/recipients` endpoints at Tenant/team/Space/App prefixes provide eligible active Tenant users, or active teams with `?type=team`, for assignment pickers. They require `authorization.manage` at the requested scope, expose no credentials, exclude platform users and grant no broader directory-management authority. User-recipient and Tenant-administrator lists search email as well as name/ID; team-recipient searches use their own response columns. Membership and team-member list responses include their fixed role assignment IDs/revisions so UI toggles use the existing guarded assignment APIs.

Space managed creation accepts an omitted `target_id`: it reuses an existing default shared resource or chooses the first active managed server in stable creation order when a resource must be allocated. Explicit target selection and plan/quota checks are preserved. With no eligible server it returns an actionable setup conflict without creating a partial Space. The overview's default layout reflects the allowed Tenant preference or plan fallback. No database schema change is required by the console simplification.

`make dev` initializes a fresh development registry named `lastapp_foundation`. The previous `lastapp_control` database is retained, and the previous configuration is saved privately before switching registries. No deployed installation or legacy grant conversion is involved.

The development-only **Northstar demo** contains Marketing/Engineering teams, a CRM Space backed by a real managed MariaDB database, and Deal review/Forecasting Apps. Joe has Team Reviewer on Marketing and inherits Sales reviewer through Marketing’s Space assignment. The Tenant administrator manages the example; the platform operator can inspect it. Choose **Joe (Marketing Reviewer)** in the local issuer to inspect his own role context through the API. All development accounts use the existing local password.

The seed then fills the demo the way an author would, as the Tenant administrator and through the same validated operations as the console (`internal/control/development_demo.go`):

- published CRM entities **Account** (name, industry, website, annual revenue, owner), **Contact** (name, email, job title, phone, account, owner) and **Deal** (name, account, primary contact, amount, stage, probability, expected close, owner, and the forecast category promoted by Forecasting's release). Sales reviewers read every field; members create records they then own, and owners read and update them;
- the development workspace **crm-pricing-exp**, owned by the Tenant administrator;
- for each App a small static build uploaded through an upload ticket, release **1.0.0** captured in the workspace, and an **active** production deployment. Deal review ships first; Forecasting's release also promotes the workspace's new Deal field, so its deployment publishes a backend change. Both Apps inherit the Space's permission paths for their data.

Queued work (storage provisioning, publications, workspace preparation, deployments) runs in the seed through the worker's own steps (`worker.Leader.Settle`), so it finishes before the server starts. If a running server already leads the worker, the seed skips this content with a warning; stop the server and seed again. A reviewer's own unpublished draft or pending change in CRM also stops it with a warning instead of being overridden. There is no failed deployment and no agent grant in the demo: a deployment fails only on a broken check or lost authority, which the seed does not fake, and agent grants are created only through the OAuth consent flow.

The demo is created once. Each piece is created only while it is missing, so restarting changes nothing: it does not restore a role an administrator revoked, an entity they changed, or redeploy an App that has any deployment. The seed preserves its existing assignments. Publishing entities and attaching their data permissions enables row access through the [entity runtime](ENTITY_RUNTIME.md). The [broader Space authorization plan](SPACE_AUTHORIZATION_PLAN.md) records the hierarchy; [VERIFICATION.md](VERIFICATION.md) records executed checks.
