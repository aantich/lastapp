package control

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	mariadb "github.com/aantich/go-mariadb"
)

func registryTransport(cfg *mariadb.Config, development bool) error {
	if development || cfg.Net == "unix" {
		return nil
	}
	if cfg.Net != "tcp" || cfg.TLS == nil || cfg.TLS.InsecureSkipVerify || cfg.AllowFallbackToPlaintext || cfg.TLS.ServerName == "" {
		return fmt.Errorf("production TCP registry connections require verified TLS without plaintext fallback")
	}
	cfg.TLS.MinVersion = max(cfg.TLS.MinVersion, tls.VersionTLS12)
	return nil
}

func validOrigin(raw string, loopback bool) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	if loopback {
		ip := net.ParseIP(u.Hostname())
		return ip != nil && ip.IsLoopback()
	}
	return true
}

func secureEndpoint(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == ""
}

type secureTransport struct{ base http.RoundTripper }

func (t secureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if !secureEndpoint(r.URL.String()) {
		return nil, fmt.Errorf("OIDC requests require HTTPS")
	}
	return t.base.RoundTrip(r)
}
func (s *Server) oidcClient() *http.Client {
	c := &http.Client{Timeout: 10 * time.Second}
	if !s.Config.Development {
		c.Transport = secureTransport{http.DefaultTransport}
		c.CheckRedirect = func(r *http.Request, via []*http.Request) error {
			if len(via) >= 5 || !secureEndpoint(r.URL.String()) {
				return fmt.Errorf("unsafe OIDC redirect")
			}
			return nil
		}
	}
	return c
}
func (s *Server) allowedOrigin(origin string) bool {
	return origin == s.Config.PublicURL || (s.Config.Development && s.Config.DevelopmentUIOrigin != "" && origin == s.Config.DevelopmentUIOrigin)
}
func (s *Server) cookieName(name string) string {
	if strings.HasPrefix(s.Config.PublicURL, "https://") {
		return "__Host-" + name
	}
	return name
}
