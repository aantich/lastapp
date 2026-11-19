package control

import (
	"net/http"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/authz"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/roles"
)

func (s *Server) roles(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	v, e := s.authority.Roles(r.Context(), httpapi.Actor(r, p), scope, res, httpapi.PageRequest(r.URL.Query()))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) createRole(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	var in roles.RoleInput
	if e = httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authority.CreateRole(r.Context(), httpapi.Actor(r, p), scope, res, in)
	if e == nil {
		httpapi.SendRef(w, 201, v)
	}
	return e
}

func (s *Server) updateRole(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	var in roles.RolePatch
	if e = httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authority.UpdateRole(r.Context(), httpapi.Actor(r, p), scope, res, r.PathValue("id"), in, httpapi.IfMatch(r))
	if e == nil {
		httpapi.SendRef(w, 200, v)
	}
	return e
}

func (s *Server) grants(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	list := s.authority.Assignments
	if r.URL.Query().Get("include") == "space" {
		list = s.authority.AppAccessAssignments
	}
	v, e := list(r.Context(), httpapi.Actor(r, p), scope, res, httpapi.PageRequest(r.URL.Query()))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) createGrant(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	var in roles.AssignmentInput
	if e = httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authority.Grant(r.Context(), httpapi.Actor(r, p), scope, res, in)
	if e == nil {
		httpapi.SendRef(w, 201, v)
	}
	return e
}

func (s *Server) deleteGrant(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	v, e := s.authority.Revoke(r.Context(), httpapi.Actor(r, p), scope, res, r.PathValue("id"), httpapi.IfMatch(r))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) recipientOptions(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	if r.URL.Query().Get("type") == "team" {
		v, e := s.authority.TeamRecipients(r.Context(), httpapi.Actor(r, p), scope, res, httpapi.PageRequest(r.URL.Query()))
		return httpapi.Reply(w, 200, v, e)
	}
	v, e := s.authority.Recipients(r.Context(), httpapi.Actor(r, p), scope, res, httpapi.PageRequest(r.URL.Query()))
	return httpapi.Reply(w, 200, v, e)
}

// requestScope reads a role/assignment scope from the console route.
func requestScope(r *http.Request) (authz.Scope, access.Resource, error) {
	return roles.ScopeOf(r.PathValue("tenant"), r.PathValue("team"), r.PathValue("space"), r.PathValue("app"))
}
