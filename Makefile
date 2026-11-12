.PHONY: dev stop test integration web build
NODE_BIN ?= $(HOME)/.nvm/versions/node/v24.20.0/bin
export PATH := $(NODE_BIN):$(PATH)
export GOCACHE := $(CURDIR)/.local/go-cache

dev:
	python3 scripts/dev.py
stop:
	python3 scripts/dev.py --stop
web:
	cd web/admin && npm ci && npm run build
build: web
	go build -o .local/lastapp ./cmd/lastapp
	go build -o .local/devoidc ./cmd/devoidc
test:
	go test ./...
integration:
	LASTAPP_SECURITY_CONFIG=$(CURDIR)/.local/config.json LASTAPP_TEST_CONFIG=$(CURDIR)/.local/config.json go test -race -count=1 -v ./internal/control
	cd web/admin && npm run test:e2e
