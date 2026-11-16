package control

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"slices"
	"strings"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/registry"
	clientsdk "github.com/aantich/lastapp/packages/client"
	"github.com/aantich/lastapp/packages/starter"
	admin "github.com/aantich/lastapp/web/admin"
)

func starterVersions() (map[string]string, error) {
	var lock struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
	}
	if e := json.Unmarshal(admin.PackageLock, &lock); e != nil {
		return nil, e
	}
	out := map[string]string{}
	for _, name := range []string{"react", "react-dom", "typescript", "vite", "@types/react", "@types/react-dom"} {
		out[name] = lock.Packages["node_modules/"+name].Version
		if out[name] == "" {
			return nil, registry.Bad("Starter dependency version is unavailable.")
		}
	}
	return out, nil
}

func (s *Server) Starter(ctx context.Context, actor Actor, app string) (registry.Row, error) {
	p := actor.Principal
	a, ws, e := s.ownedDevelopmentApp(ctx, s.DB, p, app)
	if e != nil {
		return nil, e
	}
	versions, e := starterVersions()
	if e != nil {
		return nil, e
	}
	files := map[string]string{}
	e = fs.WalkDir(starter.Files, "template", func(name string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if entry.IsDir() {
			return nil
		}
		b, e := starter.Files.ReadFile(name)
		if e == nil {
			files[strings.TrimPrefix(name, "template/")] = string(b)
		}
		return e
	})
	if e != nil {
		return nil, e
	}
	files["src/lastapp.ts"] = clientsdk.Source
	config, _ := json.MarshalIndent(registry.Row{"apiOrigin": s.Config.PublicURL, "appID": a.ID, "environment": "development"}, "", "  ")
	files["lastapp.local.json"] = string(config) + "\n"
	pkg, _ := json.MarshalIndent(registry.Row{"name": "lastapp-client", "private": true, "version": "0.0.0", "type": "module", "scripts": registry.Row{"dev": "vite", "build": "tsc --noEmit && vite build", "preview": "vite preview", "upload": "node scripts/upload.mjs"}, "dependencies": registry.Row{"react": versions["react"], "react-dom": versions["react-dom"]}, "devDependencies": registry.Row{"typescript": versions["typescript"], "vite": versions["vite"], "@types/react": versions["@types/react"], "@types/react-dom": versions["@types/react-dom"]}}, "", "  ")
	files["package.json"] = string(pkg) + "\n"
	files[".gitignore"] = "node_modules/\ndist/\n.env*\nlastapp.local.json\n"
	return registry.Row{"app_id": a.ID, "space_id": a.SpaceID, "workspace_id": ws.ID, "files": files, "development_origin": "http://127.0.0.1:5173", "instructions": []string{"Write these files locally; run npm install and npm run dev. Commit your generated lockfile in your own source repository if you use one.", "Inspect workspace entity inventory, entity definitions and App contracts. Reuse entities, this SDK and shared UI across micro-apps.", "Approve development_origin with app_access_update. Use ordinary Connect, or create a development-launch PKCE code with lastapp_workspace and call window.lastapp.connectDevelopment({code,verifier,nonce}) from your browser tools.", "Create reusable fixtures through the ordinary workspace App data API. No production records are copied and no administrator ACL bypass exists.", "Build using npm run build. Get an upload ticket, then run node scripts/upload.mjs dist <upload_url> with its token in LASTAPP_UPLOAD_TOKEN. Do not bake secrets into the bundle.", "Create a release with the uploaded build ID, exact micro-app versions and shared dependency versions. Preview it, fix errors locally, upload a new build and repeat.", "Production activation requires an explicit production.deploy grant and a current production base. Poll the deployment result."}}, nil
}

func (s *Server) appStarterArchive(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	data, e := s.Starter(r.Context(), httpActor(r, p), r.PathValue("app"))
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
	v, e := s.Starter(r.Context(), httpActor(r, p), r.PathValue("app"))
	return reply(w, 200, v, e)
}
