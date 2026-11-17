package control

import (
	"crypto/tls"
	"fmt"

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
