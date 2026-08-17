.DEFAULT_GOAL := help
SHELL := /bin/bash

SERVER_DIR   := server
CLIENT_DIR   := client
DATABASE_URL ?= postgres://vocalis:vocalis@localhost:5432/vocalis?sslmode=disable
MIGRATIONS   := internal/db/migrations

GOOSE := cd $(SERVER_DIR) && GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(DATABASE_URL)" go tool goose -dir $(MIGRATIONS)

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: db-up
db-up: ## Start Postgres and wait until it accepts connections
	docker compose up -d postgres
	@until [ "$$(docker inspect -f '{{.State.Health.Status}}' vocalis-postgres 2>/dev/null)" = "healthy" ]; do \
		sleep 1; \
	done
	@echo "postgres ready"

.PHONY: db-down
db-down: ## Stop containers (data is preserved)
	docker compose down

.PHONY: db-reset
db-reset: ## Drop the volume and rebuild the schema from scratch
	docker compose down -v
	$(MAKE) db-up migrate

.PHONY: migrate
migrate: ## Apply all pending migrations
	$(GOOSE) up

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	$(GOOSE) down

.PHONY: migrate-status
migrate-status: ## Show which migrations have run
	$(GOOSE) status

.PHONY: migration
migration: ## Create a migration: make migration name=add_reactions
	@test -n "$(name)" || (echo "usage: make migration name=add_reactions" && exit 1)
	$(GOOSE) create $(name) sql

.PHONY: sqlc
sqlc: ## Regenerate the type-safe query layer from SQL
	cd $(SERVER_DIR) && sqlc generate

.PHONY: types
types: ## Regenerate client TypeScript types from pkg/events
	cd $(SERVER_DIR) && go tool tygo generate

.PHONY: generate
generate: sqlc types ## Run every code generator

.PHONY: verify-generated
verify-generated: generate ## Fail if generated code is stale (for CI)
	@git diff --exit-code -- $(SERVER_DIR)/internal/db/gen $(CLIENT_DIR)/src/types \
		|| (echo "generated code is out of date; run 'make generate' and commit" && exit 1)

.PHONY: up
up: db-up migrate $(CLIENT_DIR)/node_modules ## Run everything: Postgres, API and client
	@echo ""
	@echo "  API   http://localhost:8080"
	@echo "  App   http://localhost:1420"
	@echo "  Ctrl-C stops both."
	@echo ""
	@trap 'kill 0' EXIT INT TERM; 	( cd $(SERVER_DIR) && go run ./cmd/api ) & 	( cd $(CLIENT_DIR) && npm run dev ) & 	wait

$(CLIENT_DIR)/node_modules: $(CLIENT_DIR)/package.json
	cd $(CLIENT_DIR) && npm install
	@touch $@

.PHONY: share
share: db-up migrate .env $(CLIENT_DIR)/node_modules ## Serve the app and print a public link for friends
	cd $(CLIENT_DIR) && npm run build
	@command -v cloudflared >/dev/null || { echo "cloudflared not found. brew install cloudflared"; exit 1; }
	@trap 'kill 0' EXIT INT TERM; \
	set -a; . ./.env; set +a; \
	( cd $(SERVER_DIR) && UI_DIR=../$(CLIENT_DIR)/dist go run ./cmd/api ) & \
	printf "starting server"; \
	until curl -sf http://localhost:8080/healthz >/dev/null 2>&1; do printf "."; sleep 1; done; \
	echo; \
	cloudflared tunnel --url http://localhost:8080 > .tunnel.log 2>&1 & \
	printf "opening tunnel"; \
	for i in $$(seq 1 40); do \
	  url=$$(grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' .tunnel.log 2>/dev/null | head -1); \
	  if [ -n "$$url" ]; then break; fi; printf "."; sleep 1; \
	done; \
	echo; echo; \
	if [ -n "$$url" ]; then \
	  echo "  Send this to your friends:"; \
	  echo "      $$url"; \
	else \
	  echo "  Tunnel did not start. See .tunnel.log"; \
	fi; \
	echo; echo "  Local: http://localhost:8080    Ctrl-C stops everything."; echo; \
	wait

.PHONY: serve
serve: db-up migrate .env $(CLIENT_DIR)/node_modules ## Serve the app on :8080 without a public link
	cd $(CLIENT_DIR) && npm run build
	set -a; . ./.env; set +a; \
	cd $(SERVER_DIR) && UI_DIR=../$(CLIENT_DIR)/dist go run ./cmd/api

# A generated secret, because the development default is committed to this
# public repository and would let anyone forge a token for any account.
.env:
	@sed 's|^JWT_SECRET=.*|JWT_SECRET='"$$(openssl rand -base64 48 | tr -d '\n')"'|' .env.example > .env
	@echo "created .env with a freshly generated JWT_SECRET"

.PHONY: dev
dev: db-up migrate ## Start Postgres, migrate, then run only the server
	cd $(SERVER_DIR) && go run ./cmd/api

.PHONY: client-install
client-install: ## Install client dependencies
	cd $(CLIENT_DIR) && npm install

.PHONY: dev-client
dev-client: ## Run the Tauri desktop client (needs Rust)
	cd $(CLIENT_DIR) && npm run tauri dev

.PHONY: dev-web
dev-web: ## Run the client in a browser at :1420 (no Rust needed)
	cd $(CLIENT_DIR) && npm run dev

.PHONY: client-check
client-check: ## Typecheck and build the client
	cd $(CLIENT_DIR) && npm run build

.PHONY: client-build
client-build: ## Bundle the desktop app (needs Rust)
	cd $(CLIENT_DIR) && npm run tauri build

.PHONY: build
build: ## Build the server binary into server/bin/
	cd $(SERVER_DIR) && go build -o bin/vocalis-server ./cmd/api

.PHONY: test
test: ## Run the unit test suite with the race detector
	cd $(SERVER_DIR) && go test -race ./...

.PHONY: e2e
e2e: db-up migrate ## Drive the real API and gateway against Postgres
	cd $(SERVER_DIR) && go test -tags=e2e -race -count=1 -v ./internal/app/

.PHONY: lint
lint: ## Vet and check formatting
	cd $(SERVER_DIR) && go vet ./...
	@test -z "$$(cd $(SERVER_DIR) && gofmt -l .)" || \
		(echo "unformatted files:" && cd $(SERVER_DIR) && gofmt -l . && exit 1)

.PHONY: check
check: lint test ## Everything CI runs
