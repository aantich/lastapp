package control

import (
	"net/http"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/membership"
)

func (s *Server) addTenantAdministrator(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in membership.AccountChoice
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.AddAdministrator(r.Context(), httpActor(r, p), r.PathValue("tenant"), in)
	if e == nil {
		send(w, 201, v)
	}
	return e
}

func (s *Server) tenantAdministrators(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.members.Administrators(r.Context(), httpActor(r, p), r.PathValue("tenant"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}
