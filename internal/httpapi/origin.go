package httpapi

import (
	"strings"

	"github.com/aantich/lastapp/internal/config"
)

// AllowedOrigin reports whether origin is the console origin (or, in
// development, the configured UI origin).
func AllowedOrigin(c *config.Config, origin string) bool {
	return origin == c.PublicURL || (c.Development && c.DevelopmentUIOrigin != "" && origin == c.DevelopmentUIOrigin)
}

// Console session cookie names, before CookieName's prefix. internal/session
// sets them; the data API's console launch reads the session cookie and
// GET /v1/config publishes the CSRF cookie name to the console.
const (
	SessionCookie = "lastapp_session"
	CSRFCookie    = "lastapp_csrf"
)

// CookieName adds the __Host- prefix to name on an HTTPS public URL.
func CookieName(publicURL, name string) string {
	if strings.HasPrefix(publicURL, "https://") {
		return "__Host-" + name
	}
	return name
}
