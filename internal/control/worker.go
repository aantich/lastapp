package control

import (
	"context"
	"database/sql"
	"time"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/data"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
)

// A pinned registry connection owns leadership and publishes every worker result.
func (s *Server) RunWorker(ctx context.Context) {
	for ctx.Err() == nil {
		if e := s.workerLeader(ctx); e != nil && ctx.Err() == nil {
			httpapi.Logger(ctx, s.Log).Warn("provisioning leader disconnected", httpapi.Diagnostic(e)...)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (s *Server) workerLeader(ctx context.Context) error {
	conn, e := s.DB.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	lock := "lastapp:" + s.registryName
	var acquired int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", lock).Scan(&acquired); e != nil {
		return e
	}
	if acquired != 1 {
		return nil
	}
	defer data.ReleasePinnedLock(conn, lock)
	if e = s.provisioner.Start(ctx, conn); e != nil {
		return e
	}
	lastPublicationTenant := ""
	cleanedAt := time.Time{}
	for ctx.Err() == nil {
		if time.Since(cleanedAt) > time.Minute {
			if _, e = cleanupSessions(ctx, conn); e != nil {
				return e
			}
			if e = access.CleanupAgentSessions(ctx, conn); e != nil {
				return e
			}
			if _, e = conn.ExecContext(ctx, "DELETE FROM artifact_uploads WHERE expires_at<=UTC_TIMESTAMP(6) LIMIT 1000"); e != nil {
				return e
			}
			if e = access.CleanupClients(ctx, conn); e != nil {
				return e
			}
			cleanedAt = time.Now()
		}
		if e = s.delivery.DeploymentJob(ctx, conn); e != nil {
			return e
		}
		if e = s.delivery.WorkspaceJob(ctx, conn); e != nil {
			return e
		}
		if _, err := s.publisher.Job(ctx, conn, &lastPublicationTenant); err != nil {
			return err
		}
		busy, e := s.provisioner.Run(ctx, conn)
		if e != nil {
			return e
		}
		if !busy {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		}
	}
	return ctx.Err()
}

func cleanupSessions(ctx context.Context, q registry.Queryer) (sql.Result, error) {
	return q.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at<=UTC_TIMESTAMP(6) LIMIT 1000")
}
