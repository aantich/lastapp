# Isolated development and static releases

**Implemented and locally verified, 14 November 2026.** Current contracts are in [WORKSPACES.md](WORKSPACES.md); executed checks are in [VERIFICATION.md](VERIFICATION.md).

Approved scope: authors use local files, terminal and browser tools; each author/agent has an isolated Space workspace; built files upload to Lastapp's filesystem. Git and remote build execution are deferred.

## Boundaries

- A workspace is an owned development copy of a Space configuration with a newly provisioned database. No production records, credentials, assignments or browser origins are copied.
- Reuse the existing Space entity, publication, ACL and App API implementations. A private backing Space is an implementation detail, not another public hierarchy level.
- The owner and, for an agent workspace, its owning delegation are checked live. An agent's production Space grant may reach only its own workspace. Development authority does not authorize production deployment.
- Browser test sessions are bound to an exact workspace App/origin, PKCE, user and live delegation. They confer ordinary business ACLs, never administrator access to records.
- An immutable App release pins its static build, micro-app versions and shared dependencies. Uploading does not activate production. Source and compilation remain local.
- Production promotion checks the captured production base, maps stable entity/role/App identities, uses the existing safe publication engine, and activates static files after backend preparation. Database DDL and client activation are not one transaction. A failed backend step leaves the old client release selected and reports the durable job's state.
- Filesystem storage is a bounded local implementation behind a small artifact-store boundary. Deployed replicas require a shared persistent volume until object storage is implemented.

## Implementation and acceptance

1. Registry/workspace lifecycle, isolated allocation, baseline copying, owner/delegation guards and context inventories.
2. Development App authentication, downloadable local starter with the authoritative SDK, fixtures through the ordinary workspace App APIs.
3. Bounded immutable artifact uploads, App release manifests, isolated static hosting and production deployment jobs.
4. Console workspace/release flows and concise MCP tools using the same validated operations.
5. Integration tests for isolation, ACLs, revocation, replay/stale edits, upload confinement and the complete local build → workspace API → release flow; browser QA; current documentation.

No production data copying, Git service, remote shell/build farm, arbitrary backend code or automatic destructive schema rollback is included.
