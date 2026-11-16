// Package config is the server configuration that internal/control loads and
// validates, shared by the domain packages that read it.
package config

import (
	"net"
	"net/url"
)

type Config struct {
	WorkspaceDatabaseTarget string        `json:"workspace_database_target_id"`
	ArtifactDirectory       string        `json:"artifact_directory"`
	AgentClients            []AgentClient `json:"agent_clients"`
	DevelopmentUIOrigin     string        `json:"development_ui_origin"`
	AppOrigin               string        `json:"app_origin"`
	Address                 string        `json:"address"`
	PublicURL               string        `json:"public_url"`
	RegistryDSN             string        `json:"registry_dsn"`
	EncryptionKey           string        `json:"encryption_key"`
	EncryptionKeyID         string        `json:"encryption_key_id"`
	OIDCIssuer              string        `json:"oidc_issuer"`
	OIDCClientID            string        `json:"oidc_client_id"`
	OIDCClientSecret        string        `json:"oidc_client_secret"`
	Development             bool          `json:"development"`
	AllowedNetworks         []string      `json:"allowed_networks"`
	TrustedProxies          []string      `json:"trusted_proxies"`
	RuntimeAccountHost      string        `json:"runtime_account_host"`
}

type AgentClient struct {
	ID        string   `json:"client_id"`
	Name      string   `json:"client_name"`
	Redirects []string `json:"redirect_uris"`
	Scope     string   `json:"scope,omitempty"`
}

func ValidOrigin(raw string, loopback bool) bool {
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
