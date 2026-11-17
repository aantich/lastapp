package control

import (
	"net/http"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/delivery"
	"github.com/aantich/lastapp/internal/httpapi"
)

func (s *Server) workspaceRoutes() {
	s.api("GET /v1/spaces/{space}/workspaces", s.workspaceList)
	s.api("POST /v1/spaces/{space}/workspaces", s.workspaceCreate)
	s.api("GET /v1/workspaces/{workspace}", s.workspaceGet)
	s.api("POST /v1/workspaces/{workspace}/archive", s.workspaceArchive)
	s.api("POST /v1/workspaces/{workspace}/retry", s.workspaceRetry)
	s.api("POST /v1/apps/{app}/development-launch", s.developmentLaunch)
}

func (s *Server) workspaceList(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.delivery.Workspaces(r.Context(), httpapi.Actor(r, p), r.PathValue("space"), httpapi.PageRequest(r.URL.Query()))
	return httpapi.Reply(w, 200, v, e)
}

func (s *Server) workspaceGet(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.delivery.Workspace(r.Context(), httpapi.Actor(r, p), r.PathValue("workspace"))
	return httpapi.Reply(w, 200, v, e)
}

func (s *Server) workspaceCreate(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in delivery.WorkspaceCreateInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.delivery.CreateWorkspace(r.Context(), httpapi.Actor(r, p), r.PathValue("space"), in)
	return httpapi.Reply(w, 200, v, e)
}

func (s *Server) workspaceArchive(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.delivery.ChangeWorkspace(r.Context(), httpapi.Actor(r, p), r.PathValue("workspace"), false, httpapi.IfMatch(r))
	return httpapi.Reply(w, 200, v, e)
}

func (s *Server) workspaceRetry(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.delivery.ChangeWorkspace(r.Context(), httpapi.Actor(r, p), r.PathValue("workspace"), true, httpapi.IfMatch(r))
	return httpapi.Reply(w, 200, v, e)
}

func (s *Server) developmentLaunch(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in delivery.DevelopmentLaunchInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.delivery.Launch(r.Context(), httpapi.Actor(r, p), r.PathValue("app"), in)
	return httpapi.Reply(w, 200, v, e)
}
