// Console response shapes. The backend writes most responses as untyped Go
// maps (control.Row), so there is no generated TypeScript for them: the
// generated TypeScript in internal/entity/contracts.go covers per-entity App
// record data only. Each shape below names the Go source it mirrors; change
// both together.

// A registry row or form value whose columns vary by endpoint. The one place
// the console accepts loosely typed JSON; prefer a named shape below when a
// screen depends on specific fields.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type Item = Record<string, any>;

// internal/control/pagination.go: pageInfo, sendPage.
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

// internal/control/passwords.go: publicPasswordPolicy.
export type PasswordPolicy = {
  min_length: number;
  max_length: number;
  length_hint: string;
};

// internal/control/http.go: GET /v1/config (fields the console reads).
export type ConsoleConfig = {
  development_policy: { max_release_microapps: number };
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

// internal/control/workspace.go: tenantOverview. Every field is optional
// because screens render before it loads; tenant is the raw tenants/plans row.
export type TenantOverview = {
  tenant?: Item;
  spaces?: number;
  apps?: number;
  microapps?: number;
  databases?: number;
  recent_apps?: Item[];
  attention?: Item[];
};

// internal/entity/expression.go: Operator, ContextProvider.
export type Operator = {
  key: string;
  min_args: number;
  max_args: number;
  kind: string;
};
export type ContextProvider = { key: string; label: string; type: string };

// internal/entity/model.go: Capabilities (GET /v1/entity-capabilities).
// Templates and field defaults are entity Definition/Field documents.
export type EntityCatalog = {
  format_version: number;
  types: string[];
  operations: string[];
  operators: Operator[];
  relationships: string[];
  limits: Record<
    | "Fields"
    | "Rules"
    | "Permissions"
    | "Depth"
    | "Nodes"
    | "Page"
    | "MaxPage"
    | "Bulk"
    | "ReadPaths"
    | "DataRequests",
    number
  >;
  templates: Item[];
  field_defaults: Record<string, Item>;
  field_constraints: Record<string, number>; // internal/entity/limits.go
  context_providers: ContextProvider[];
};

// internal/control/app_sessions.go: appLaunchInfo, App launch code exchange.
export type LaunchInfo = {
  id: string;
  name: string;
  space: string;
  origin: string;
};
export type LaunchCode = {
  code: string;
  origin: string;
  nonce: string;
  expires_in: number;
};

// internal/control/agent_oauth.go: agent consent decision.
export type Redirect = { redirect_url: string };
