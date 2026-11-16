package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	mariadb "github.com/aantich/go-mariadb"

	"github.com/aantich/lastapp/internal/entity"
	"github.com/aantich/lastapp/internal/registry"
)

// Transport-independent operation primitives. Console handlers and MCP tools
// call the same operations with an explicit Actor; neither transport fakes the
// other. These types become internal/op when the package is split.

// Actor is the caller of one operation: the live principal (authority is always
// rechecked against the registry), the request ID recorded in audit events and,
// for an MCP tool call, the delegated agent call that owns its receipt.
type Actor struct {
	Principal Principal
	RequestID string
	Agent     *AgentCall
}

// AgentCall is one delegated MCP tool execution: the grant, the delegated
// permissions it requires, the receipt identity (request key and input hash),
// the compact result reference stored in the receipt and whether it committed.
type AgentCall struct {
	Grant                *agentGrant
	Permission           []string
	Operation, Key, Hash string
	ResultRef            registry.Row
	Committed            bool
}

type agentContextKey struct{}

func agentCallFrom(ctx context.Context) *AgentCall {
	a, _ := ctx.Value(agentContextKey{}).(*AgentCall)
	return a
}

// errActorAgentMismatch rejects an operation whose Actor does not carry the
// delegated call already in its context.
var errActorAgentMismatch = errors.New("operation actor does not carry the agent call of its context")

// withAgentCall exposes a to the live authorization checks, which still read
// the delegated call from ctx until they take the Actor themselves. The Actor
// is the only source: ctx is never a fallback, and an Actor that disagrees
// with an agent call already in ctx fails rather than skipping its receipt.
func withAgentCall(ctx context.Context, a *AgentCall) (context.Context, error) {
	switch current := agentCallFrom(ctx); {
	case current == a:
		return ctx, nil
	case current != nil:
		return ctx, errActorAgentMismatch
	}
	return context.WithValue(ctx, agentContextKey{}, a), nil
}

// actorFrom is the shim for code not yet migrated to explicit actors: it reads
// the agent call that an MCP adapter placed in ctx.
func actorFrom(ctx context.Context, p Principal, requestID string) Actor {
	return Actor{Principal: p, RequestID: requestID, Agent: agentCallFrom(ctx)}
}

// agentActor is the actor of one MCP tool call. Every call gets its own
// operation request ID, as an HTTP request would.
func agentActor(ctx context.Context, p Principal) Actor {
	return actorFrom(ctx, p, registry.NewID())
}

// Ref is the compact result of a mutation: the record and its new revision.
type Ref struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

// revisionRequired rejects an update without an expected revision.
func revisionRequired() error {
	return registry.Problem(428, "revision_required", "Send If-Match with the current resource revision.")
}

// opError is the one transport-independent classification of an operation
// error. HTTP writes payload() under "error"; MCP returns it in its tool result.
type opError struct {
	Status             int
	Code, Message      string
	Field, FieldOrRule string // empty when not applicable
	Rule               *string
	Retryable          bool
	// Authority carries the missing delegated scopes of needs_authorization.
	Authority *agentAuthorityError
	// Unmapped marks an unexpected failure: log diagnostic() server-side and
	// never return its details.
	Unmapped bool
}

func classify(err error) opError {
	var ae *registry.APIError
	var ed entity.Diagnostic
	var me *mariadb.MySQLError
	if errors.As(err, &ae) {
	} else if errors.As(err, &ed) {
		ae = &registry.APIError{Status: 400, Code: ed.Code, Message: ed.Message}
	} else if errors.As(err, &me) {
		switch me.Number {
		case 1062:
			ae = &registry.APIError{Status: 409, Code: "already_exists", Message: "This ID or name already exists. Read the existing resource before retrying."}
		case 1451, 1452:
			ae = &registry.APIError{Status: 409, Code: "invalid_reference", Message: "A referenced resource is missing, belongs to another Tenant, or is still in use."}
		case 4025:
			ae = &registry.APIError{Status: 400, Code: "invalid_request", Message: "The request violates a resource constraint."}
		case 1205, 1213:
			ae = &registry.APIError{Status: 409, Code: "retry_transaction", Message: "A concurrent change occurred; refresh and retry."}
		}
	}
	o := opError{}
	if ae == nil {
		ae = &registry.APIError{Status: 500, Code: "internal_error", Message: "The operation failed. See the server log with this request ID."}
		o.Unmapped = true
	}
	o.Status, o.Code, o.Message = ae.Status, ae.Code, ae.Message
	o.Retryable = ae.Code == "retry_transaction" || ae.Code == "publication_in_progress" || ae.Status == 429
	if ed.Path != "" {
		o.FieldOrRule = ed.Path
		if ed.Code == "invalid_value" {
			o.Field = ed.Path
		}
	}
	var rf entity.RuleFailure
	if errors.As(err, &rf) {
		if rf.Field != "" {
			o.Field = rf.Field
		}
		o.Rule = &rf.Rule
	}
	if ae.Code == "needs_authorization" {
		var authority *agentAuthorityError
		if errors.As(err, &authority) {
			o.Authority = authority
		}
	}
	return o
}

func (o opError) payload(requestID string) registry.Row {
	v := registry.Row{"code": o.Code, "message": o.Message, "request_id": requestID, "retryable": o.Retryable}
	if o.FieldOrRule != "" {
		v["field_or_rule"] = o.FieldOrRule
	}
	if o.Field != "" {
		v["field"] = o.Field
	}
	if o.Rule != nil {
		v["rule"] = *o.Rule
	}
	return v
}

// runner is the mutation core shared by every operation: one READ COMMITTED
// transaction that locks the Tenant (or the platform SuperRole) row first,
// rechecks live authorization and the production guard, then writes the agent
// receipt together with the change and its audit events.
type runner struct {
	db *sql.DB
	// access provides the live authorization checks (authorize, scopes, guards).
	// It is the only *Server dependency and becomes internal/access.
	access *Server
}

func (r *runner) mutate(ctx context.Context, a Actor, action string, res Resource, fn func(*sql.Tx) error) error {
	ctx, e := withAgentCall(ctx, a.Agent)
	if e != nil {
		return e
	}
	tx, e := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if res.Tenant != "" {
		var x string
		if e = tx.QueryRowContext(ctx, "SELECT id FROM tenants WHERE id=? FOR UPDATE", res.Tenant).Scan(&x); e != nil {
			return registry.Missing()
		}
	} else {
		var x string
		if e = tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE id=? FOR UPDATE", SuperRole).Scan(&x); e != nil {
			return e
		}
	}
	if e = r.access.authorize(ctx, tx, a.Principal, action, res, true); e != nil {
		return e
	}
	if e = r.access.productionMutationGuard(ctx, tx, a.Principal, action, res); e != nil {
		return e
	}
	if e = agentMutationBegin(ctx, tx, a.Agent); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		return e
	}
	if e = agentMutationFinish(ctx, tx, a.Agent); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	if a.Agent != nil {
		a.Agent.Committed = true
	}
	return nil
}

// authorize is a read-side live authorization check for a.
func (r *runner) authorize(ctx context.Context, a Actor, action string, res Resource) error {
	return r.check(ctx, r.db, a, action, res, false)
}

// authorizeTx rechecks a's mutating authority inside an operation transaction.
func (r *runner) authorizeTx(ctx context.Context, tx *sql.Tx, a Actor, action string, res Resource) error {
	return r.check(ctx, tx, a, action, res, true)
}

// check is the live authorization check on q. mutating selects the stricter
// membership and platform-control rules of a change, also before its transaction.
func (r *runner) check(ctx context.Context, q registry.Queryer, a Actor, action string, res Resource, mutating bool) error {
	ctx, e := withAgentCall(ctx, a.Agent)
	if e != nil {
		return e
	}
	return r.access.authorize(ctx, q, a.Principal, action, res, mutating)
}

// operationID is the identifier of the record a's operation creates: the
// receipt's reserved result ID for an agent call, a new ID otherwise.
func (a Actor) operationID() string {
	if a.Agent != nil {
		return registry.Str(a.Agent.ResultRef["id"])
	}
	return registry.NewID()
}

// scopes returns a's live read scopes in tenant; apply them before paging.
func (r *runner) scopes(ctx context.Context, a Actor, tenant, action string) (scopeSet, error) {
	ctx, e := withAgentCall(ctx, a.Agent)
	if e != nil {
		return scopeSet{}, e
	}
	return r.access.scopes(ctx, r.db, a.Principal, tenant, action)
}

// ref reads a record's compact result after its mutation committed.
func (r *runner) ref(ctx context.Context, table, tenant, record string) (Ref, error) {
	query := "SELECT id,revision FROM " + table + " WHERE id=?"
	args := []any{record}
	if tenant != "" {
		query += " AND tenant_id=?"
		args = append(args, tenant)
	}
	var v Ref
	if e := r.db.QueryRowContext(ctx, query, args...).Scan(&v.ID, &v.Revision); e != nil {
		if errors.Is(e, sql.ErrNoRows) {
			return v, registry.Missing()
		}
		return v, e
	}
	return v, nil
}

// auditAs records an audit event attributed to a: the human user, the request
// ID and, for delegated calls, the agent grant and client. Workers carry the
// grant identity of their persisted job through agentAuditContext.
func auditAs(ctx context.Context, q registry.Queryer, a Actor, tenant, action, kind, record string, details any) error {
	identity, _ := ctx.Value(agentAuditKey{}).(agentAuditIdentity)
	if a.Agent != nil {
		identity = agentAuditIdentity{Grant: a.Agent.Grant.ID, Client: a.Agent.Grant.Client}
	}
	if identity.Grant != "" {
		details = registry.Row{"agent_grant_id": identity.Grant, "agent_client_id": identity.Client, "details": details}
	}
	b, _ := json.Marshal(details)
	_, e := q.ExecContext(ctx, "INSERT INTO audit_events (tenant_id,actor_user_id,action,resource_type,resource_id,request_id,details_json) VALUES (?,?,?,?,?,?,?)", registry.Nullable(tenant), registry.Nullable(a.Principal.ID), action, kind, registry.Nullable(record), a.RequestID, string(b))
	return e
}
