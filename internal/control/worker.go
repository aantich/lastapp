package control

import "context"

// RunWorker runs the background worker leader (internal/worker) until ctx ends.
func (s *Server) RunWorker(ctx context.Context) { s.worker.Run(ctx) }
