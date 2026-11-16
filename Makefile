.PHONY: dev stop test integration web build lint fmt
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
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
lint:
	$(GOLANGCI_LINT) run ./...
	cd web/admin && npm run lint && npm run typecheck
fmt:
	$(GOLANGCI_LINT) fmt ./...
integration:
	LASTAPP_SECURITY_CONFIG=$(CURDIR)/.local/config.json LASTAPP_TEST_CONFIG=$(CURDIR)/.local/config.json go test -race -count=1 -v ./internal/control
	cd web/admin && npm run test:e2e

.PHONY: mcp-test
mcp-test:
	LASTAPP_TEST_CONFIG=$(CURDIR)/.local/config.json go test -race -count=1 -v ./internal/control -run 'TestAgent'

.PHONY: client
client:
	web/admin/node_modules/.bin/tsc -p packages/client/tsconfig.json
	cp packages/client/dist/index.js examples/data-app/public/client.js
