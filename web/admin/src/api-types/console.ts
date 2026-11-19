import type { Item } from "./loose";

// internal/op/pagination.go: PageInfo; internal/httpapi/pagination.go: SendPage.
export type PageInfo = { page: number; limit: number; has_more: boolean };
export type Page<T = Item> = { items: T[]; pagination?: PageInfo };

// internal/control/http.go: GET /v1/me.
export type Me = {
  id: string;
  display_name: string;
  email: string;
  platform_admin: boolean;
  auth_method: string;
  password_change_required: boolean;
  tenants: { id: string; slug: string; display_name: string; state: string }[];
  pagination?: PageInfo;
};

// internal/membership/accounts.go: PublicPasswordPolicy.
export type PasswordPolicy = {
  min_length: number;
  max_length: number;
  length_hint: string;
};

// internal/control/http.go: GET /v1/config (fields the console reads).
// internal/delivery/policy.go: Policy. deployment_states lists a production
// deployment's states in worker order (DeploymentSteps) and its failure state;
// workspace_steps a preparing workspace's steps in worker order
// (WorkspaceSteps).
export type DeploymentStates = { steps: string[]; failed: string };
export type ConsoleConfig = {
  development_policy: {
    max_release_microapps: number;
    deployment_states: DeploymentStates;
    workspace_steps: string[];
  };
  oidc_enabled: boolean;
  password_policy: PasswordPolicy;
  role_catalog: { id: string; code: string }[];
};

// internal/control/http.go: capabilities (GET /v1/tenants/{tenant}/capabilities),
// keyed by action code.
export type ScopeCapability = {
  all: boolean;
  available: boolean;
  spaces: string[] | null;
  apps: string[] | null;
  teams: string[] | null;
};
export type TenantCapabilities = Record<string, ScopeCapability | undefined>;
// The same response's control_roles: the account's fixed control roles in
// the Tenant (internal/authz catalog), broadest first.
export type ControlRole = {
  id: string;
  code: string;
  display_name: string;
  scope_kind: string;
};
export type CapabilitiesResponse = {
  permissions: TenantCapabilities;
  control_roles: ControlRole[];
};

// internal/control/workspace.go: tenantOverview. Every field is optional
// because screens render before it loads; tenant is the raw tenants/plans row.
export type TenantOverview = {
  tenant?: Item;
  spaces?: number;
  apps?: number;
  microapps?: number;
  // Production Apps a deployment made live; published entities.
  live_apps?: number;
  entities?: number;
  databases?: number;
  // The newest production Apps, each with its micro-app count (microapps).
  recent_apps?: Item[];
  attention?: Item[];
};
