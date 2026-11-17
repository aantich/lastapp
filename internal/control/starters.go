package control

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"slices"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
)

func (s *Server) appStarterArchive(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	data, e := s.delivery.Starter(r.Context(), httpapi.Actor(r, p), r.PathValue("app"))
	if e != nil {
		return e
	}
	files, ok := data["files"].(map[string]string)
	if !ok {
		return registry.Bad("Starter unavailable.")
	}
	var content bytes.Buffer
	archive := zip.NewWriter(&content)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		file, e := archive.Create(name)
		if e != nil {
			return e
		}
		if _, e = io.WriteString(file, files[name]); e != nil {
			return e
		}
	}
	if e = archive.Close(); e != nil {
		return e
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="lastapp-starter.zip"`)
	w.WriteHeader(200)
	_, e = w.Write(content.Bytes())
	return e
}

func (s *Server) appStarter(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.delivery.Starter(r.Context(), httpapi.Actor(r, p), r.PathValue("app"))
	return httpapi.Reply(w, 200, v, e)
}
