package access

import (
	"context"
	"time"

	"github.com/aantich/lastapp/internal/registry"
)

const (
	// UnapprovedClientLifetime bounds a registered agent OAuth client that no
	// grant has approved yet.
	UnapprovedClientLifetime = 10 * time.Minute
	// ClientIdleLifetime is the retention window of an approved client identity.
	ClientIdleLifetime = 30 * 24 * time.Hour
)

// CleanupAgentSessions deletes token sessions of revoked or expired grants and
// expired OAuth authorization requests, at most 1000 rows each.
func CleanupAgentSessions(ctx context.Context, q registry.Queryer) error {
	if _, e := q.ExecContext(ctx, "DELETE FROM agent_token_sessions WHERE grant_id IN (SELECT id FROM agent_grants WHERE state='revoked' OR expires_at<=UTC_TIMESTAMP(6)) LIMIT 1000"); e != nil {
		return e
	}
	_, e := q.ExecContext(ctx, "DELETE FROM agent_oauth_requests WHERE expires_at<=UTC_TIMESTAMP(6) LIMIT 1000")
	return e
}

// Unapproved registrations have a short, bounded lifetime. Approved client
// identities use the longer retention window and do not consume anonymous quota.
// Never evict a client during consent/code exchange or while a grant is live.
func CleanupClients(ctx context.Context, q registry.Queryer) error {
	_, e := q.ExecContext(ctx, `DELETE FROM agent_oauth_clients WHERE
 ((created_at<? AND NOT EXISTS (SELECT 1 FROM agent_grants g WHERE g.client_id=agent_oauth_clients.id)) OR last_used_at<?)
 AND NOT EXISTS (SELECT 1 FROM agent_grants g WHERE g.client_id=agent_oauth_clients.id AND g.state='active' AND g.expires_at>UTC_TIMESTAMP(6))
 AND NOT EXISTS (SELECT 1 FROM agent_oauth_requests r WHERE r.client_id=agent_oauth_clients.id AND r.consumed=0 AND r.expires_at>UTC_TIMESTAMP(6))
 LIMIT 1000`, time.Now().UTC().Add(-UnapprovedClientLifetime), time.Now().UTC().Add(-ClientIdleLifetime))
	return e
}
