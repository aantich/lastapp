package control

import (
	"context"
	"net/http"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/inventory"
	"github.com/aantich/lastapp/internal/op"
)

func (s *Server) spaces(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Spaces(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), httpapi.PageRequest(r.URL.Query()))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) getSpace(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Space(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.PathValue("space"))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) createSpace(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in inventory.SpaceCreateInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.CreateSpace(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), in)
	if e == nil {
		httpapi.SendRef(w, 201, v)
	}
	return e
}

func (s *Server) apps(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Apps(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.URL.Query().Get("space_id"), httpapi.PageRequest(r.URL.Query()))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) getApp(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.App(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.PathValue("app"))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) createApp(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in inventory.AppCreateInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.CreateApp(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.PathValue("space"), in)
	if e == nil {
		httpapi.SendRef(w, 201, v)
	}
	return e
}

func (s *Server) microapps(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.inv.Microapps(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.URL.Query().Get("app_id"), httpapi.PageRequest(r.URL.Query()))
	if e == nil {
		httpapi.Send(w, 200, v)
	}
	return e
}

func (s *Server) createMicroapp(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in inventory.MicroappCreateInput
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.inv.CreateMicroapp(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.PathValue("app"), in)
	if e == nil {
		httpapi.SendRef(w, 201, v)
	}
	return e
}

func (s *Server) updateSpace(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	return updateInventory(w, r, p, s.inv.UpdateSpace)
}

func (s *Server) updateApp(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	return updateInventory(w, r, p, s.inv.UpdateApp)
}

func (s *Server) updateMicroapp(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	return updateInventory(w, r, p, s.inv.UpdateMicroapp)
}

// updateInventory is the console adapter of the Space, App and micro-app
// metadata and lifecycle operations.
func updateInventory(w http.ResponseWriter, r *http.Request, p access.Principal, update func(context.Context, op.Actor, string, string, inventory.MetadataPatch, int64) (op.Ref, error)) error {
	var in inventory.MetadataPatch
	if e := httpapi.Decode(w, r, &in); e != nil {
		return e
	}
	v, e := update(r.Context(), httpapi.Actor(r, p), r.PathValue("tenant"), r.PathValue("id"), in, httpapi.IfMatch(r))
	if e == nil {
		httpapi.SendRef(w, 200, v)
	}
	return e
}
