package agent

import (
	"github.com/aantich/lastapp/internal/registry"
)

// Agent permissions are delegation ceilings, never independent user roles.
var agentPermissions = []registry.Row{
	{"code": "workspaces.write", "label": "Create isolated workspaces", "description": "Allocate bounded, empty development databases on the Space target or platform-configured development target and manage your own workspaces. Never copies production records."},
	{"code": "workspaces.test", "label": "Test workspace Apps", "description": "Issue short-lived browser test sessions for your own workspace Apps. Normal business ACLs still apply. No production records."},
	{"code": "releases.write", "label": "Upload and test releases", "description": "Upload bounded static builds and capture immutable development App releases."},
	{"code": "production.deploy", "label": "Deploy to production", "description": "Explicitly authorize production configuration changes and activation of reviewed App releases. Separate from workspace development."},
	{"code": "read", "label": "Inspect configuration", "description": "Read authorized inventories, contracts, roles, documentation and operational audit; no business records."},
	{"code": "resources.write", "label": "Create and edit resources", "description": "Create/edit Spaces, Apps, micro-app registrations and teams. An initial manager defaults to you; choosing another manager also requires access-management authority. Reusing existing shared storage needs no provisioning grant; actual allocation does."},
	{"code": "entities.draft", "label": "Edit entity drafts", "description": "Create/edit full entity drafts, including proposed rules and ACLs; never activates them."},
	{"code": "entities.check", "label": "Validate and test drafts", "description": "Validate definitions and simulate synthetic cases; no business-record access."},
	{"code": "entities.publish", "label": "Publish entities, rules and ACLs", "description": "Activate the captured entity definition, including its schema, business rules and permissions. This can change data access."},
	{"code": "access.manage", "label": "Manage access", "description": "Manage custom roles, role assignments, memberships and App origin/access settings within your existing authority."},
	{"code": "databases.provision", "label": "Provision databases", "description": "Create managed databases on explicitly approved targets, including allocation during Space creation; retry existing jobs."},
	{"code": "lifecycle.manage", "label": "Change resource lifecycle", "description": "Archive, disable, restore or retire supported resources. Also requires access-management authority; changing entity lifecycle requires publication authority."},
	{"code": "platform.manage", "label": "Manage platform inventory", "description": "Manage Tenants and plans only with an explicitly approved platform grant and current platform authority. Does not permit Tenant authoring."},
}

var (
	agentDefaultPermissions = []string{"read"}
	agentPresets            = []registry.Row{
		{"label": "Workspace author", "description": "Own development copy: build, test, release and publish there", "permissions": []string{"read", "workspaces.write", "workspaces.test", "releases.write", "resources.write", "entities.draft", "entities.check", "entities.publish", "access.manage", "lifecycle.manage"}},
		{"label": "Inspect only", "description": "Read configuration, change nothing", "permissions": []string{"read"}},
		{"label": "App author", "description": "Create and edit Spaces, Apps and micro-apps", "permissions": []string{"read", "resources.write"}},
		{"label": "Entity author", "description": "Draft and check entities; never publish", "permissions": []string{"read", "entities.draft", "entities.check"}},
	}
	// The expiry choices the consent form offers; each lies within
	// 1..agentMaxGrantDays and includes the default.
	agentGrantDayChoices = []int{1, agentGrantDays, agentMaxGrantDays}
)

func agentPermissionKinds(code string) []string {
	switch code {
	case "read", "access.manage", "lifecycle.manage":
		return []string{"platform", "tenant", "team", "space", "app"}
	case "resources.write":
		return []string{"tenant", "team", "space", "app"}
	case "workspaces.write", "workspaces.test", "releases.write", "production.deploy", "entities.draft", "entities.check", "entities.publish":
		return []string{"tenant", "space"}
	case "databases.provision":
		return []string{"tenant"}
	case "platform.manage":
		return []string{"platform"}
	}
	return nil
}

func agentPermissionCatalog() []registry.Row {
	out := []registry.Row{}
	for _, permission := range agentPermissions {
		p := registry.Row{}
		for k, v := range permission {
			p[k] = v
		}
		p["scope_kinds"] = agentPermissionKinds(registry.Str(p["code"]))
		out = append(out, p)
	}
	return out
}

// ScopeCodes are the OAuth scopes: the agent permission codes.
func ScopeCodes() []string {
	out := make([]string, 0, len(agentPermissions))
	for _, permission := range agentPermissions {
		out = append(out, registry.Str(permission["code"]))
	}
	return out
}
