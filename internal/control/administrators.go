package control

import (
	"net/http"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/membership"
)

func (s *Server) addTenantAdministrator(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in membership.AccountChoice
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.members.AddAdministrator(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), in)
	if e == nil {
		httpapi.Send(w, 201, v)
	}
	return e
}

func (s *Server) tenantAdministrators(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.members.Administrators(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), httpapi.PageRequest(r.URL.Query()))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}
