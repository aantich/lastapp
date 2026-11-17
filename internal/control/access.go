package control

import (
	"net/http"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/membership"
	"github.com/aantich/lastapp/internal/op"
)

func (s *Server) memberships(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.members.Memberships(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), httpapi.PageRequest(r.URL.Query()))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) addMembership(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in membership.AccountChoice
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.AddMembership(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), in)
	if e == nil {
		httpapi.Send(w, 201, v)
	}
	return e
}

func (s *Server) updateMembership(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in struct {
		State string `json:"state"`
	}
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.UpdateMembership(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.PathValue("user"), in.State, httpapi.IfMatch(r))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) groups(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.members.Teams(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), httpapi.PageRequest(r.URL.Query()))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) getGroup(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.members.Team(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.PathValue("id"))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in op.NamedInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.CreateTeam(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), in)
	if e == nil {
		httpapi.SendRef(w, 201, v)
	}
	return e
}

func (s *Server) updateGroup(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in membership.TeamPatch
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.UpdateTeam(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.PathValue("id"), in, httpapi.IfMatch(r))
	if e == nil {
		httpapi.SendRef(w, 200, v)
	}
	return e
}

func (s *Server) addGroupMember(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in struct {
		User string `json:"user_id"`
	}
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.AddTeamMember(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.PathValue("id"), in.User, httpapi.IfMatch(r))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) removeGroupMember(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.members.RemoveTeamMember(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.PathValue("id"), r.PathValue("user"), httpapi.IfMatch(r))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) updateGroupMember(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in struct {
		State string `json:"state"`
	}
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.SetTeamMemberState(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.PathValue("id"), r.PathValue("user"), in.State, httpapi.IfMatch(r))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) groupMembers(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.members.TeamMembers(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.PathValue("id"), httpapi.PageRequest(r.URL.Query()))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}
