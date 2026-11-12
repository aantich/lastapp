module github.com/aantich/lastapp

go 1.25.0

require (
	github.com/aantich/go-mariadb v0.0.0
	github.com/coreos/go-oidc/v3 v3.17.0
	github.com/google/uuid v1.6.0
	github.com/pressly/goose/v3 v3.26.0
	golang.org/x/oauth2 v0.32.0
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.3 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/sethvargo/go-retry v0.3.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
)

replace github.com/aantich/go-mariadb => ./third_party/go-mariadb
