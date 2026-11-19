// internal/agent/oauth.go: agent consent decision.
export type Redirect = { redirect_url: string };

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
