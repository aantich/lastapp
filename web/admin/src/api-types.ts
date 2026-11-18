// Console response shapes. The backend writes most responses as untyped Go
// maps (control.Row), so there is no generated TypeScript for them: the
// generated TypeScript in internal/entity/contracts.go covers per-entity App
// record data only. Each shape below names the Go source it mirrors; change
// both together.

// A registry row or form value whose columns vary by endpoint. Its values are
// unknown: narrow them where they are used (the helpers below, typeof checks)
// or prefer a named shape below when a screen depends on specific fields.
export type Item = Record<string, unknown>;

// Display text of a loose value, as React would render it: strings and
// numbers as written, nothing for null, undefined, booleans or objects.
export const text = (v: unknown): string =>
  typeof v === "string" ? v : typeof v === "number" ? String(v) : "";

// A loose value that is a number, or undefined.
export const num = (v: unknown): number | undefined =>
  typeof v === "number" ? v : undefined;

// A loose value that is a JSON object.
export const isItem = (v: unknown): v is Item =>
  typeof v === "object" && v !== null && !Array.isArray(v);

// A loose value that is an array of JSON objects (other entries dropped).
export const items = (v: unknown): Item[] =>
  Array.isArray(v) ? v.filter(isItem) : [];

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

// internal/dataapi/sessions.go: LaunchInfo, App launch code exchange.
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

// internal/agent/oauth.go: agent consent decision.
export type Redirect = { redirect_url: string };

// A loose value usable as a React key: strings and numbers as written,
// undefined otherwise (so React falls back to the index, as before).
export const keyOf = (v: unknown): string | number | undefined =>
  typeof v === "string" || typeof v === "number" ? v : undefined;

// A loose value that is a string, or undefined (keeps "absent" distinct
// from "empty" where an optional string parameter is expected).
export const optText = (v: unknown): string | undefined =>
  typeof v === "string" ? v : undefined;

// internal/control/artifacts.go: upload ticket; internal/artifact/store.go: Limits.
export type UploadTicket = {
  upload_url: string;
  upload_token: string;
  expires_in: number;
  limits: Record<
    "max_bytes" | "max_file_bytes" | "max_files" | "max_body_bytes",
    number
  >;
};

// internal/control/artifacts.go: Build.
export type Build = {
  id: string;
  content_hash: string;
  size_bytes: number;
  created_at: string;
  entrypoint: string;
};

// internal/inventory/inventory.go: DeploymentStatus, an App list item's
// latest deployment (present only where the reader may manage its
// releases). failed_state is the step a failed deployment had reached.
export type DeploymentSummary = {
  id: string;
  state: string;
  failed_state: string | null;
  error_message: string | null;
  release_id: string;
  release_name: string;
  deployed_by: string;
  created_at: string;
  finished_at: string | null;
};

// internal/delivery/releases.go: Deployment. deployed_by is the person who
// started it (an agent acts for them); release_name the release's name;
// attempts counts its runs and retried_by names who last retried it.
export type Deployment = DeploymentSummary & {
  url: string;
  revision: number;
  attempts: number;
  retried_by: string | null;
};

// internal/entities/entities.go: EntityItem (GET /v1/spaces/{id}/entities).
// has_draft_changes: the saved draft differs from the active version (or
// nothing is published yet).
export type EntityItem = {
  id: string;
  entity_key: string;
  label: string;
  state: string;
  revision: number;
  active_version_id: string | null;
  pending_version_id: string | null;
  has_draft_changes: boolean;
};

// internal/delivery/workspaces.go: WorkspaceItem (GET
// /v1/spaces/{id}/workspaces). preparation_step is the WorkspaceSteps step a
// preparing workspace has reached (or a failed one failed at);
// agent_client_name names the agent client that owns an agent workspace.
export type WorkspaceItem = {
  id: string;
  display_name: string;
  backing_space_id: string;
  production_space_id: string;
  owner_agent_grant_id: string | null;
  agent_client_name: string | null;
  preparation_step: string | null;
  error_message: string | null;
  state: string;
  revision: number;
  created_at: string;
};

// internal/delivery/releases.go: ReleaseItem (GET /v1/apps/{id}/releases):
// who captured it, and the agent client that owns its workspace, if any.
export type ReleaseItem = {
  id: string;
  display_name: string;
  build_id: string;
  created_at: string;
  created_by_name: string;
  agent_client_name: string | null;
  snapshot_hash: string;
};

// internal/control/releases.go: Release (GET /v1/releases/{id}). The manifest
// pins micro-app and package versions; the snapshot is
// internal/control/development_workspaces.go: spaceSnapshot, whose entity
// definitions are internal/entity/model.go: Definition (fields read loosely).
export type ReleaseManifest = {
  microapps?: Record<string, string>;
  dependencies?: Record<string, string>;
};
export type Release = {
  id: string;
  display_name: string;
  build_id: string;
  created_at: string;
  production_app_id: string;
  manifest_json: ReleaseManifest | null;
  snapshot_json: {
    entities: {
      id: string;
      definition: { label: string; fields: Item[]; permissions: unknown[] };
    }[];
    roles: unknown[];
    apps: unknown[];
  };
};

// internal/access/delegation.go: AgentLimits.
export type AgentLimits = { target_ids: string[]; max_databases: number };

// internal/agent/server.go: GET /v1/agent-access/config; permissions
// and presets are internal/agent/permissions.go: agentPermissionCatalog,
// agentPresets.
export type AgentPermission = {
  code: string;
  label: string;
  description: string;
  scope_kinds: string[];
};
export type AgentConfig = {
  mcp_url: string;
  permissions: AgentPermission[];
  presets: { label: string; description: string; permissions: string[] }[];
  default_permissions: string[];
  default_limits: AgentLimits;
  default_days: number;
  max_days: number;
  // The expiry choices the consent form offers, within 1..max_days.
  day_choices: number[];
  // Active, unexpired connections one person may hold at a time.
  max_connections: number;
  max_databases: number;
  max_targets: number;
  access_token_seconds: number;
};

// internal/agent/oauth.go: pending connection request.
export type AgentRequest = {
  id: string;
  client_name: string;
  client_id: string;
  redirect_uri: string;
  scopes: string;
  expires_at: string;
  resource_uri: string;
};

// internal/agent/oauth.go: GrantItem.
export type AgentGrant = {
  client_id: string;
  client_name: string;
  created_at: string;
  expires_at: string;
  id: string;
  last_used_at: string | null;
  limits_json: AgentLimits;
  owner_name: string;
  permissions_json: string[];
  revision: number;
  scope_id: string | null;
  scope_kind: string;
  scope_name: string;
  state: string;
  tenant_id: string | null;
  tenant_name: string | null;
  user_id: string;
};
