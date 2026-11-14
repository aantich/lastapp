package control

import (
	"context"
	"encoding/json"
)

type agentWorkspaceInput struct {
	Action     string                  `json:"action" jsonschema:"create, archive, retry or launch. Inspect workspace status with lastapp_get; list workspaces under the production Space with lastapp_list."`
	Space      string                  `json:"space_id,omitempty" jsonschema:"Production Space ID for create. Never use a backing workspace Space here."`
	Workspace  string                  `json:"workspace_id,omitempty"`
	App        string                  `json:"app_id,omitempty" jsonschema:"Owned workspace App for launch; never a production App."`
	Name       string                  `json:"display_name,omitempty"`
	Launch     *developmentLaunchInput `json:"launch,omitempty" jsonschema:"launch: exact approved origin, locally generated S256 challenge and nonce. Keep the verifier locally; never include it here."`
	Revision   int64                   `json:"expected_revision,omitempty"`
	RequestKey string                  `json:"request_key"`
}
type agentReleaseInput struct {
	Action     string              `json:"action" jsonschema:"upload_ticket, capture, preview or deploy. All builds compile locally. Upload tickets accept one bounded immutable file tree; they cannot execute code."`
	App        string              `json:"app_id,omitempty"`
	Release    string              `json:"release_id,omitempty"`
	Capture    *releaseCreateInput `json:"capture,omitempty" jsonschema:"capture: immutable build ID, release name, exact versions keyed by every active micro-app slug, plus shared dependencies. Inspect lastapp_get release before production deployment."`
	Revision   int64               `json:"expected_revision,omitempty" jsonschema:"Current development App revision for capture/preview; current production App revision for deploy (1 if it does not exist)."`
	RequestKey string              `json:"request_key"`
}

func (s *Server) toolAgentWorkspace(ctx context.Context, p Principal, raw json.RawMessage) (any, error) {
	var in agentWorkspaceInput
	if e := strictAgentInput(raw, &in); e != nil {
		return nil, e
	}
	var fn endpoint
	var body any
	paths := map[string]string{}
	permission := "workspaces.write"
	switch in.Action {
	case "create":
		if id(in.Space) != nil {
			return nil, bad("create needs production space_id.")
		}
		paths["space"] = in.Space
		body = workspaceCreateInput{ID: stableAgentID(p, in.RequestKey), Name: in.Name}
		fn = s.workspaceCreate
	case "archive", "retry":
		if id(in.Workspace) != nil || in.Revision < 1 {
			return nil, bad("workspace_id and expected_revision are required.")
		}
		paths["workspace"] = in.Workspace
		fn = s.workspaceArchive
		if in.Action == "retry" {
			fn = s.workspaceRetry
		}
	case "launch":
		if id(in.App) != nil || in.Launch == nil {
			return nil, bad("launch needs a workspace app_id and launch parameters.")
		}
		paths["app"] = in.App
		body = in.Launch
		permission = "workspaces.test"
		fn = s.developmentLaunch
	default:
		return nil, bad("action is create, archive, retry or launch.")
	}
	ctx, e := s.beginAgentWrite(ctx, p, in.RequestKey, Row{"id": stableAgentID(p, in.RequestKey), "operation": "workspace." + in.Action}, in, permission)
	if e != nil {
		return nil, e
	}
	v, e := s.executeManagement(ctx, p, fn, "POST", "/v1/development/"+in.Action, paths, body, in.Revision)
	return agentWriteResult(ctx, v, e)
}
func (s *Server) toolAgentRelease(ctx context.Context, p Principal, raw json.RawMessage) (any, error) {
	var in agentReleaseInput
	if e := strictAgentInput(raw, &in); e != nil {
		return nil, e
	}
	var fn endpoint
	var body any
	paths := map[string]string{}
	permission := "releases.write"
	switch in.Action {
	case "upload_ticket":
		fn = s.artifactTicket
		paths["app"] = in.App
	case "capture":
		fn = s.releaseCreate
		paths["app"] = in.App
		if in.Capture == nil {
			return nil, bad("capture parameters required.")
		}
		body = in.Capture
	case "preview":
		fn = s.releasePreview
		paths["release"] = in.Release
	case "deploy":
		fn = s.releaseDeploy
		paths["release"] = in.Release
		permission = "production.deploy"
	default:
		return nil, bad("action is upload_ticket, capture, preview or deploy.")
	}
	if in.Action != "upload_ticket" && in.Revision < 1 {
		return nil, bad("expected_revision is required.")
	}
	if in.Action == "upload_ticket" || in.Action == "capture" {
		if e := id(in.App); e != nil {
			return nil, e
		}
	} else {
		if e := id(in.Release); e != nil {
			return nil, e
		}
	}
	ctx, e := s.beginAgentWrite(ctx, p, in.RequestKey, Row{"id": stableAgentID(p, in.RequestKey), "operation": "release." + in.Action}, in, permission)
	if e != nil {
		return nil, e
	}
	v, e := s.executeManagement(ctx, p, fn, "POST", "/v1/releases/"+in.Action, paths, body, in.Revision)
	return agentWriteResult(ctx, v, e)
}
