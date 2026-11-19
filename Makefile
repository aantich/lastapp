.PHONY: dev stop test integration e2e web build lint fmt
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
# node/npm come from PATH; set NODE_BIN=/path/to/node/bin to prefer another Node.
NODE_BIN ?=
ifneq ($(strip $(NODE_BIN)),)
export PATH := $(NODE_BIN):$(PATH)
endif
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
# Database tests run only on a disposable MariaDB (scripts/testdb.py); never the dev data.
integration:
	python3 scripts/testdb.py -race -count=1 ./...
	$(MAKE) e2e
# Browser regressions on a disposable MariaDB/API (scripts/e2e.py); never the dev data.
# Pass Playwright arguments with ARGS, e.g. make e2e ARGS=tests/security.spec.ts
e2e:
	python3 scripts/e2e.py $(ARGS)

.PHONY: mcp-test
mcp-test:
	python3 scripts/testdb.py -race -count=1 -v ./internal/agent ./internal/control -run 'TestAgent'

.PHONY: client
client:
	web/admin/node_modules/.bin/tsc -p packages/client/tsconfig.json
	cp packages/client/dist/index.js examples/data-app/public/client.js
