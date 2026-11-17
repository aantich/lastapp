package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

// MCPHandler serves the management MCP server at /mcp for a bearer token.
func (s *Server) MCPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := op.WithRequestDeadline(r.Context())
		defer cancel()
		r = r.WithContext(ctx)
		if origin := r.Header.Get("Origin"); origin != "" && !httpapi.AllowedOrigin(s.Config, origin) {
			httpapi.WriteError(w, r, registry.Problem(403, "invalid_origin", "Unapproved MCP browser Origin."))
			return
		}
		if !s.admitAgent(w, r, &s.validationLimiter, s.ValidationSlots, "mcp-auth", 60, 10) {
			return
		}
		p, e := s.Principal(r)
		<-s.ValidationSlots
		if e != nil {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+s.Config.PublicURL+`/.well-known/oauth-protected-resource/mcp"`)
			httpapi.WriteError(w, r, e)
			return
		}
		r.SetPathValue("tenant", p.Agent.Tenant)
		if e = s.Limiter.LimitRequest(w, r, s.Checker, s.DB, p); e != nil {
			httpapi.WriteError(w, r, e)
			return
		}
		select {
		case s.Slots <- struct{}{}:
			defer func() { <-s.Slots }()
		default:
			httpapi.WriteError(w, r, httpapi.RateLimited(w))
			return
		}
		if r.Method == http.MethodPost {
			// One HTTP admission represents one RPC. Reject legacy batches even
			// without a protocol header, before the SDK can fan out tool calls.
			body := http.MaxBytesReader(w, r.Body, 1<<20)
			defer body.Close()
			payload, err := io.ReadAll(body)
			if err != nil {
				var tooLarge *http.MaxBytesError
				if errors.As(err, &tooLarge) {
					err = registry.Problem(413, "request_limit", "MCP request exceeds its size limit.")
				}
				httpapi.WriteError(w, r, err)
				return
			}
			trimmed := bytes.TrimSpace(payload)
			if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
				httpapi.WriteError(w, r, registry.Bad("Send exactly one MCP message object per HTTP request; batching is not supported."))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(payload))
		}
		server := mcp.NewServer(&mcp.Implementation{Name: "lastapp", Version: "0.1.0"}, &mcp.ServerOptions{Instructions: Instructions})
		for _, spec := range s.Tools() {
			if !ToolVisible(spec, p.Agent) {
				continue
			}
			schema, e := InputSchema(spec.Input)
			if e != nil {
				httpapi.WriteError(w, r, e)
				return
			}
			cached, _ := agentSchemaContracts.Load(spec.Input)
			resolved := cached.(*agentSchemaContract).Resolved
			current := spec
			server.AddTool(&mcp.Tool{Name: spec.Name, Description: spec.Description, InputSchema: schema, OutputSchema: map[string]any{"type": "object", "properties": map[string]any{"ok": map[string]any{"type": "boolean"}, "data": map[string]any{}, "error": map[string]any{}}, "required": []string{"ok"}}, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: spec.Read, IdempotentHint: true, OpenWorldHint: ptrBool(false)}}, func(call context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				var args any
				decoder := json.NewDecoder(bytes.NewReader(req.Params.Arguments))
				e := decoder.Decode(&args)
				if e == nil {
					e = resolved.Validate(args)
				}
				var data any
				a := &access.AgentCall{Grant: p.Agent, Permission: current.Permissions, Operation: current.Name}
				call = context.WithValue(call, access.AgentContextKey{}, a)
				if e != nil {
					e = registry.Bad("Tool arguments do not match its published schema: " + e.Error())
				} else if e = s.CheckAgent(call, s.DB, p, access.Resource{Tenant: p.Agent.Tenant}, false); e == nil {
					data, e = current.Run(call, p, req.Params.Arguments)
				}
				result := registry.Row{"ok": e == nil}
				if e == nil {
					result["data"] = data
				} else {
					o := op.Classify(e)
					rid := registry.NewID()
					if o.Unmapped {
						fields := []any{"operation", "mcp " + current.Name, "operation_request_id", rid}
						httpapi.RequestLogger(call).Error("request failed", append(fields, httpapi.Diagnostic(e)...)...)
					}
					result["error"] = o.Payload(rid)
					if o.Code == "needs_authorization" {
						if o.Authority != nil {
							result["required_scopes"] = o.Authority.Missing
						}
						result["authorization_url"] = s.Config.PublicURL + "/console/account"
						result["guidance"] = "The user must explicitly expand Agent access in the console. Refresh/reconnect, reread revisions, and retry. Nothing has executed."
					}
				}
				b, _ := json.Marshal(result)
				if len(b) > MaxResponse {
					result = registry.Row{"ok": false, "error": registry.Row{"code": "response_limit", "message": "Inspect a smaller scope or page; no content was silently truncated."}}
					if a.Committed {
						result["committed"], result["resource"] = true, a.ResultRef
						result["guidance"] = "The mutation committed. Inspect the resource using a smaller section/page; reuse the same request_key for unchanged retries."
					}
					b, _ = json.Marshal(result)
				}
				return &mcp.CallToolResult{StructuredContent: result, Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}, IsError: e != nil || result["ok"] == false}, nil
			})
		}
		if access.AgentHas(p.Agent, "read") {
			for name, guide := range agentGuides {
				topic, text := name, guide
				server.AddResource(&mcp.Resource{URI: "lastapp://docs/" + topic, Name: topic, MIMEType: "text/markdown", Description: "Authoritative Lastapp agent guide"}, func(call context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
					if e := s.CheckAgent(call, s.DB, p, access.Resource{Tenant: p.Agent.Tenant}, false); e != nil {
						return nil, e
					}
					return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: "lastapp://docs/" + topic, MIMEType: "text/markdown", Text: text}}}, nil
				})
			}
		}
		mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}).ServeHTTP(w, r)
	})
}
