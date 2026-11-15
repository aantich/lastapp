// A loopback-only OIDC issuer for local development, not a production identity service.
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type login struct {
	Subject, Nonce, Challenge string
	Expiry                    time.Time
}

func random() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func encode(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }

func main() {
	address := flag.String("address", "127.0.0.1:8788", "loopback address")
	origin := flag.String("app-origin", "http://127.0.0.1:8787", "allowed Lastapp origin")
	devUI := flag.String("development-ui-origin", "http://127.0.0.1:5173", "allowed loopback development UI origin")
	password := flag.String("password", "lastapp-local", "local-only demo password")
	flag.Parse()
	host, _, e := net.SplitHostPort(*address)
	if e != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("development issuer must bind to loopback")
	}
	for _, raw := range []string{*origin, *devUI} {
		u, e := url.Parse(raw)
		if e != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Scheme != "http" || net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback() {
			log.Fatal("development origins must be HTTP loopback origins")
		}
	}
	issuer := "http://" + *address
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		log.Fatal(e)
	}
	var mu sync.Mutex
	codes := map[string]login{}
	reply := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v) // client disconnects are not actionable
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		reply(w, map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/jwks", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}, "token_endpoint_auth_methods_supported": []string{"none"}, "code_challenge_methods_supported": []string{"S256"}})
	})
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, r *http.Request) {
		reply(w, map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "local", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
	})
	page := template.Must(template.New("login").Parse(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Sign in · Lastapp</title><style>body{background:#f5f6f9;font:15px system-ui;color:#192234;display:grid;place-items:center;min-height:95vh}main{background:white;padding:44px;width:340px;border:1px solid #e2e5ed;border-radius:18px;box-shadow:0 16px 50px #15284709}small{color:#7b65ca;letter-spacing:.1em}h1{font-size:26px;margin-bottom:8px}p{color:#748095;line-height:1.6}label{display:block;margin-top:22px;font-size:13px}input,button{box-sizing:border-box;width:100%;padding:12px;margin-top:8px;border:1px solid #d9dfe9;border-radius:8px;font:inherit}fieldset{border:0;padding:0;margin:22px 0 0}legend{font-size:13px;padding:0}label.acct{display:flex;align-items:center;gap:10px;margin-top:8px;padding:10px 12px;border:1px solid #d9dfe9;border-radius:8px;font-size:15px;cursor:pointer}label.acct:has(:checked){border-color:#6551d9;background:#f3f1fd}label.acct:has(:focus-visible){outline:2px solid #6551d9;outline-offset:2px}label.acct input{width:auto;margin:0;accent-color:#6551d9}button{background:#6551d9;color:white;border:0;cursor:pointer;margin-top:26px}footer{margin-top:26px;font-size:12px;color:#8390a4}</style></head><body><main><small>LASTAPP / DEVELOPMENT</small><h1>Welcome back.</h1><p>Sign in to your local control plane.</p>{{if .Error}}<p style="color:#b42318">{{.Error}}</p>{{end}}<form method="post"><fieldset><legend>Account</legend><label class="acct"><input type="radio" name="subject" value="operator" checked>Platform operator</label><label class="acct"><input type="radio" name="subject" value="tenant-admin">Tenant administrator</label><label class="acct"><input type="radio" name="subject" value="outsider">Outside user</label><label class="acct"><input type="radio" name="subject" value="joe">Joe (Marketing Reviewer)</label></fieldset><label>Password<input type="password" name="password" required autocomplete="current-password"></label><button>Sign in</button></form><footer>Local development identity provider · loopback only</footer></main></body></html>`))
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("client_id") != "lastapp-local" || q.Get("redirect_uri") != *origin+"/auth/callback" || q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" || len(q.Get("code_challenge")) != 43 || q.Get("state") == "" || q.Get("nonce") == "" {
			http.Error(w, "Invalid authorization request", 400)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self' "+*origin+" "+*devUI+"; frame-ancestors 'none'")
		if r.Method == "GET" {
			_ = page.Execute(w, nil) // static template; only write errors are possible
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Method", 405)
			return
		}
		if r.Header.Get("Origin") != issuer {
			http.Error(w, "Origin", 403)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		if r.ParseForm() != nil {
			http.Error(w, "Invalid form", 400)
			return
		}
		sub := r.PostForm.Get("subject")
		if (sub != "operator" && sub != "tenant-admin" && sub != "outsider" && sub != "joe") || subtle.ConstantTimeCompare([]byte(r.PostForm.Get("password")), []byte(*password)) != 1 {
			_ = page.Execute(w, map[string]string{"Error": "Invalid credentials."}) // only write errors are possible
			return
		}
		code := random()
		mu.Lock()
		for c, v := range codes {
			if time.Now().After(v.Expiry) {
				delete(codes, c)
			}
		}
		codes[code] = login{sub, q.Get("nonce"), q.Get("code_challenge"), time.Now().Add(time.Minute)}
		mu.Unlock()
		dest, _ := url.Parse(q.Get("redirect_uri"))
		params := dest.Query()
		params.Set("code", code)
		params.Set("state", q.Get("state"))
		dest.RawQuery = params.Encode()
		http.Redirect(w, r, dest.String(), 302)
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		if r.ParseForm() != nil {
			http.Error(w, "Invalid form", 400)
			return
		}
		client := r.Form.Get("client_id")
		if client == "" {
			client, _, _ = r.BasicAuth()
		}
		mu.Lock()
		flow, ok := codes[r.Form.Get("code")]
		delete(codes, r.Form.Get("code"))
		mu.Unlock()
		challenge := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || time.Now().After(flow.Expiry) || client != "lastapp-local" || r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("redirect_uri") != *origin+"/auth/callback" || subtle.ConstantTimeCompare([]byte(flow.Challenge), []byte(base64.RawURLEncoding.EncodeToString(challenge[:]))) != 1 {
			w.WriteHeader(400)
			reply(w, map[string]string{"error": "invalid_grant"})
			return
		}
		names := map[string]string{"operator": "Platform operator", "tenant-admin": "Tenant administrator", "outsider": "Outside user", "joe": "Joe (Marketing Reviewer)"}
		payload := map[string]any{"iss": issuer, "sub": flow.Subject, "aud": "lastapp-local", "iat": time.Now().Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(), "nonce": flow.Nonce, "name": names[flow.Subject], "email": flow.Subject + "@lastapp.local"}
		unsigned := encode(map[string]string{"alg": "RS256", "kid": "local", "typ": "JWT"}) + "." + encode(payload)
		digest := sha256.Sum256([]byte(unsigned))
		signature, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
		w.Header().Set("Cache-Control", "no-store")
		reply(w, map[string]any{"access_token": random(), "token_type": "Bearer", "expires_in": 300, "id_token": unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)})
	})
	log.Printf("Local OIDC issuer at %s", issuer)
	server := &http.Server{Addr: *address, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	if e = server.ListenAndServe(); e != nil && !strings.Contains(e.Error(), "closed") {
		log.Fatal(fmt.Sprint(e))
	}
}
