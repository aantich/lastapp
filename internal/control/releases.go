package control

import (
	"net/http"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/delivery"
	"github.com/aantich/lastapp/internal/httpapi"
)

func (s *Server) releaseCreate(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in delivery.ReleaseCreateInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.delivery.CreateRelease(r.Context(), httpapi.Actor(r, p), r.PathValue("app"), in, httpapi.IfMatch(r))
	return httpapi.Reply(w, 200, v, e)
}

func (s *Server) releaseList(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.delivery.Releases(r.Context(), httpapi.Actor(r, p), r.PathValue("app"), httpapi.PageRequest(r.URL.Query()))
	return httpapi.Reply(w, 200, v, e)
}

func (s *Server) releaseGet(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.delivery.Release(r.Context(), httpapi.Actor(r, p), r.PathValue("release"))
	return httpapi.Reply(w, 200, v, e)
}

func (s *Server) releasePreview(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.delivery.PreviewRelease(r.Context(), httpapi.Actor(r, p), r.PathValue("release"), httpapi.IfMatch(r))
	return httpapi.Reply(w, 200, v, e)
}

func (s *Server) releaseDeploy(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.delivery.DeployRelease(r.Context(), httpapi.Actor(r, p), r.PathValue("release"), httpapi.IfMatch(r))
	return httpapi.Reply(w, 202, v, e)
}

func (s *Server) deploymentGet(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.delivery.Deployment(r.Context(), httpapi.Actor(r, p), r.PathValue("deployment"))
	return httpapi.Reply(w, 200, v, e)
}

func (s *Server) appDeployment(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.delivery.AppDeployment(r.Context(), httpapi.Actor(r, p), r.PathValue("app"))
	return httpapi.Reply(w, 200, v, e)
}
