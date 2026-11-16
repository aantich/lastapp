package control

import (
	"net/http"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/authz"
	"github.com/aantich/lastapp/internal/roles"
)

func (s *Server) roles(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	v, e := s.authority.Roles(r.Context(), httpActor(r, p), scope, res, pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createRole(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	var in roles.RoleInput
	if e = decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authority.CreateRole(r.Context(), httpActor(r, p), scope, res, in)
	if e == nil {
		sendRef(w, 201, v)
	}
	return e
}

func (s *Server) updateRole(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	var in roles.RolePatch
	if e = decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authority.UpdateRole(r.Context(), httpActor(r, p), scope, res, r.PathValue("id"), in, ifMatch(r))
	if e == nil {
		sendRef(w, 200, v)
	}
	return e
}

func (s *Server) grants(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	v, e := s.authority.Assignments(r.Context(), httpActor(r, p), scope, res, pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createGrant(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	var in roles.AssignmentInput
	if e = decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.authority.Grant(r.Context(), httpActor(r, p), scope, res, in)
	if e == nil {
		sendRef(w, 201, v)
	}
	return e
}

func (s *Server) deleteGrant(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	v, e := s.authority.Revoke(r.Context(), httpActor(r, p), scope, res, r.PathValue("id"), ifMatch(r))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) recipientOptions(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	scope, res, e := requestScope(r)
	if e != nil {
		return e
	}
	if r.URL.Query().Get("type") == "team" {
		v, e := s.authority.TeamRecipients(r.Context(), httpActor(r, p), scope, res, pageRequest(r.URL.Query()))
		return reply(w, 200, v, e)
	}
	v, e := s.authority.Recipients(r.Context(), httpActor(r, p), scope, res, pageRequest(r.URL.Query()))
	return reply(w, 200, v, e)
}

// requestScope reads a role/assignment scope from the console route.
func requestScope(r *http.Request) (authz.Scope, access.Resource, error) {
	return roles.ScopeOf(r.PathValue("tenant"), r.PathValue("team"), r.PathValue("space"), r.PathValue("app"))
}
