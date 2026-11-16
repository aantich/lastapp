package control

import (
	"net/http"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/membership"
	"github.com/aantich/lastapp/internal/op"
)

func (s *Server) memberships(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.members.Memberships(r.Context(), httpActor(r, p), r.PathValue("tenant"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) addMembership(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in membership.AccountChoice
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.AddMembership(r.Context(), httpActor(r, p), r.PathValue("tenant"), in)
	if e == nil {
		send(w, 201, v)
	}
	return e
}

func (s *Server) updateMembership(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in struct {
		State string `json:"state"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.UpdateMembership(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("user"), in.State, ifMatch(r))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) groups(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.members.Teams(r.Context(), httpActor(r, p), r.PathValue("tenant"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) getGroup(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.members.Team(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in op.NamedInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.CreateTeam(r.Context(), httpActor(r, p), r.PathValue("tenant"), in)
	if e == nil {
		sendRef(w, 201, v)
	}
	return e
}

func (s *Server) updateGroup(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in membership.TeamPatch
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.UpdateTeam(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), in, ifMatch(r))
	if e == nil {
		sendRef(w, 200, v)
	}
	return e
}

func (s *Server) addGroupMember(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in struct {
		User string `json:"user_id"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.AddTeamMember(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), in.User, ifMatch(r))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) removeGroupMember(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.members.RemoveTeamMember(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), r.PathValue("user"), ifMatch(r))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) updateGroupMember(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in struct {
		State string `json:"state"`
	}
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.SetTeamMemberState(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), r.PathValue("user"), in.State, ifMatch(r))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) groupMembers(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.members.TeamMembers(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}
