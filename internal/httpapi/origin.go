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

// CookieName adds the __Host- prefix to name on an HTTPS public URL.
func CookieName(publicURL, name string) string {
	if strings.HasPrefix(publicURL, "https://") {
		return "__Host-" + name
	}
	return name
}
