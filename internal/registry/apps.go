package registry

import (
	"encoding/json"
	"time"

	"github.com/aantich/lastapp/internal/entity"
)

// AppRecord is an App's registry row.
type AppRecord struct {
	ActiveRelease    *string   `json:"active_release_id"`
	Origin           *string   `json:"approved_origin"`
	Compute          string    `json:"compute_mode"`
	CreatedAt        time.Time `json:"created_at"`
	Access           JSONValue `json:"data_access_json"`
	Name             string    `json:"display_name"`
	ID               string    `json:"id"`
	OriginGeneration int64     `json:"origin_generation"`
	OriginHost       *string   `json:"origin_host"`
	Revision         int64     `json:"revision"`
	Slug             string    `json:"slug"`
	SpaceID          string    `json:"space_id"`
	State            string    `json:"state"`
	TenantID         string    `json:"tenant_id"`
}

const AppRecordColumns = "active_release_id,approved_origin,compute_mode,created_at,data_access_json,display_name,id,origin_generation,origin_host,revision,slug,space_id,state,tenant_id"

func (v *AppRecord) ScanFields() []any {
	return []any{&v.ActiveRelease, &v.Origin, &v.Compute, &v.CreatedAt, &v.Access, &v.Name, &v.ID, &v.OriginGeneration, &v.OriginHost, &v.Revision, &v.Slug, &v.SpaceID, &v.State, &v.TenantID}
}

// AccessOf decodes an App's data_access_json policy.
func AccessOf(v JSONValue) (entity.AppAccess, error) {
	var a entity.AppAccess
	b, _ := json.Marshal(v)
	e := json.Unmarshal(b, &a)
	return a, e
}
