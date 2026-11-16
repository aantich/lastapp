package registry

import (
	"time"
)

// Workspace is a development workspace. Read with its backing storage
// (workspaceStorage), it also carries the database resource and its state;
// the workspace view adds its item mappings.
type Workspace struct {
	BackingSpace     string              `json:"backing_space_id"`
	BaseHash         string              `json:"base_hash"`
	CreatedAt        time.Time           `json:"created_at"`
	DatabaseResource *string             `json:"database_resource_id,omitempty"`
	DatabaseState    *string             `json:"database_state,omitempty"`
	Name             string              `json:"display_name"`
	Error            *string             `json:"error_message"`
	ID               string              `json:"id"`
	Mappings         *[]WorkspaceMapping `json:"mappings,omitempty"`
	OwnerGrant       *string             `json:"owner_agent_grant_id"`
	Owner            string              `json:"owner_user_id"`
	ProductionSpace  string              `json:"production_space_id"`
	Revision         int64               `json:"revision"`
	State            string              `json:"state"`
	TenantID         string              `json:"tenant_id"`
}

// WorkspaceColumns selects a Workspace from development_workspaces w.
const WorkspaceColumns = "w.backing_space_id,w.base_hash,w.created_at,w.display_name,w.error_message,w.id,w.owner_agent_grant_id,w.owner_user_id,w.production_space_id,w.revision,w.state,w.tenant_id"

func (v *Workspace) ScanFields() []any {
	return []any{&v.BackingSpace, &v.BaseHash, &v.CreatedAt, &v.Name, &v.Error, &v.ID, &v.OwnerGrant, &v.Owner, &v.ProductionSpace, &v.Revision, &v.State, &v.TenantID}
}

// WorkspaceMapping maps a development record to its production counterpart.
type WorkspaceMapping struct {
	Development string `json:"development_id"`
	Kind        string `json:"kind"`
	Production  string `json:"production_id"`
}

func (v *WorkspaceMapping) ScanFields() []any { return []any{&v.Development, &v.Kind, &v.Production} }
