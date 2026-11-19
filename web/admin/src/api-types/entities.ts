import type { Item } from "./loose";

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
