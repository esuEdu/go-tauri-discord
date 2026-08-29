.DEFAULT_GOAL := help
SHELL := /bin/bash

SERVER_DIR   := server
CLIENT_DIR   := client
DATABASE_URL ?= postgres://vocalis:vocalis@localhost:5432/vocalis?sslmode=disable
MIGRATIONS   := internal/db/migrations
IMAGE        ?= vocalis:latest
COMPOSE_PROD := docker compose -f docker-compose.prod.yml
COMPOSE_OBSERVE := docker compose -f docker-compose.observe.yml

ifneq ($(wildcard $(HOME)/.cargo/bin/cargo),)
export PATH := $(HOME)/.cargo/bin:$(PATH)
endif

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
dev-client: require-rust ## Run the Tauri desktop client (needs Rust)
	cd $(CLIENT_DIR) && npm run tauri dev

.PHONY: dev-web
dev-web: ## Run the client in a browser at :1420 (no Rust needed)
	cd $(CLIENT_DIR) && npm run dev

.PHONY: client-check
client-check: ## Typecheck and build the client
	cd $(CLIENT_DIR) && npm run build

.PHONY: client-build
client-build: require-rust ## Bundle the desktop app (needs Rust)
	cd $(CLIENT_DIR) && npm run tauri build

.PHONY: desktop-check
desktop-check: require-rust ## Compile and bundle the desktop app unoptimised (for CI)
	cd $(CLIENT_DIR) && npm run tauri -- build --debug --bundles app

.PHONY: desktop-smoke
desktop-smoke: ## Launch the built desktop app and fail if it dies (catches WebKit key renames)
	@cd $(CLIENT_DIR) && { ./src-tauri/target/debug/vocalis & pid=$$!; \
	sleep 12; \
	if kill -0 $$pid 2>/dev/null; then \
		kill $$pid 2>/dev/null; wait $$pid 2>/dev/null; \
		echo "desktop app survived launch"; \
	else \
		echo "desktop app died on launch; check the WebKit preference keys in $(CLIENT_DIR)/src-tauri/src/lib.rs"; \
		exit 1; \
	fi; }

.PHONY: require-rust
require-rust:
	@command -v cargo >/dev/null || { \
		echo "cargo not found."; \
		echo "Install: curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"; \
		echo "Then open a new terminal, or run: . \"$$HOME/.cargo/env\""; \
		exit 1; \
	}

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

.PHONY: image
image: ## Build the deployable image: server binary plus the built client
	docker build -t $(IMAGE) .

.PHONY: image-smoke
image-smoke: ## Boot the image against a throwaway Postgres and fail unless it serves
	@set -e; \
	cleanup() { \
		docker rm -f vocalis-smoke vocalis-smoke-db >/dev/null 2>&1 || true; \
		docker network rm vocalis-smoke >/dev/null 2>&1 || true; \
	}; \
	cleanup; trap cleanup EXIT; \
	docker network create vocalis-smoke >/dev/null; \
	docker run -d --name vocalis-smoke-db --network vocalis-smoke \
		-e POSTGRES_USER=vocalis -e POSTGRES_PASSWORD=vocalis -e POSTGRES_DB=vocalis \
		postgres:17-alpine >/dev/null; \
	printf "waiting for postgres"; \
	for i in $$(seq 1 60); do \
		docker exec vocalis-smoke-db pg_isready -h 127.0.0.1 -U vocalis -d vocalis >/dev/null 2>&1 && break; \
		printf "."; sleep 1; \
	done; echo; \
	docker run -d --name vocalis-smoke --network vocalis-smoke -p 18080:8080 \
		-e DATABASE_URL=postgres://vocalis:vocalis@vocalis-smoke-db:5432/vocalis?sslmode=disable \
		-e JWT_SECRET=$$(openssl rand -base64 48 | tr -d '\n') \
		-e WEBRTC_PUBLIC_IP=127.0.0.1 \
		$(IMAGE) >/dev/null; \
	printf "waiting for the server"; \
	ok=; \
	for i in $$(seq 1 60); do \
		if curl -sf http://localhost:18080/healthz >/dev/null 2>&1; then ok=1; break; fi; \
		docker inspect -f '{{.State.Running}}' vocalis-smoke | grep -q true || break; \
		printf "."; sleep 1; \
	done; echo; \
	if [ -z "$$ok" ]; then echo "the server never answered /healthz:"; docker logs vocalis-smoke; exit 1; fi; \
	curl -sf http://localhost:18080/ | grep -q 'id="root"' \
		|| { echo "the container serves no client at /"; exit 1; }; \
	docker exec vocalis-smoke-db psql -U vocalis -d vocalis -tAc \
		"select to_regclass('public.users')" | grep -q users \
		|| { echo "migrations did not run at boot"; docker logs vocalis-smoke; exit 1; }; \
	echo "image serves the client, answers /healthz and migrated an empty database"

.PHONY: deploy-env
deploy-env: ## Create .env for a deployment from deploy/env.example, with generated secrets
	@test ! -f .env || { echo ".env already exists. Move it aside first."; exit 1; }
	@pass=$$(openssl rand -hex 24); secret=$$(openssl rand -base64 48 | tr -d '\n'); \
	grafana=$$(openssl rand -hex 16); \
	sed -e "s|CHANGE_ME_PASSWORD|$$pass|g" -e "s|CHANGE_ME_SECRET|$$secret|" \
		-e "s|CHANGE_ME_GRAFANA|$$grafana|" \
		deploy/env.example > .env
	@echo "created .env with a generated database password and JWT_SECRET."
	@echo "Set DOMAIN and CORS_ORIGINS to your own name before starting."

.PHONY: deploy-up
deploy-up: ## Build and start Postgres, the server and Caddy
	@test -f .env || { echo "no .env. Run: make deploy-env"; exit 1; }
	@grep -q '^ENV=production' .env || { \
		echo ".env is a development file: it has no ENV=production line."; \
		echo "The development .env points at a local database with a committed"; \
		echo "password, and this would hand it to the deployment. Move it aside"; \
		echo "and run: make deploy-env"; \
		exit 1; }
	$(COMPOSE_PROD) --profile tls up -d --build

.PHONY: deploy-pull
deploy-pull: ## Deploy the image built by CI, instead of building here
	@test -f .env || { echo "no .env. Run: make deploy-env"; exit 1; }
	@grep -q '^VOCALIS_IMAGE=' .env || { \
		echo "no VOCALIS_IMAGE in .env. Add the published image:"; \
		echo "  echo 'VOCALIS_IMAGE=ghcr.io/esuEdu/vocalis:latest' >> .env"; \
		exit 1; }
	$(COMPOSE_PROD) pull server
	$(COMPOSE_PROD) --profile tls up -d --no-build
	@echo "running $$(grep '^VOCALIS_IMAGE=' .env | cut -d= -f2-)"

.PHONY: deploy-down
deploy-down: ## Stop the deployment; volumes and their data are preserved
	$(COMPOSE_PROD) --profile tls down

.PHONY: deploy-logs
deploy-logs: ## Follow the server log
	$(COMPOSE_PROD) logs -f server

.PHONY: observe-up
observe-up: ## Start Grafana, Prometheus, Loki and the exporters alongside the deployment
	@test -f .env || { echo "no .env. Run: make deploy-env"; exit 1; }
	@grep -q '^GRAFANA_PASSWORD=' .env || { \
		echo "no GRAFANA_PASSWORD in .env. Add one:"; \
		echo "  echo \"GRAFANA_PASSWORD=$$(openssl rand -hex 16)\" >> .env"; \
		exit 1; }
	$(COMPOSE_OBSERVE) up -d
	@echo ""
	@echo "  Grafana is on the host's loopback only. Reach it from your laptop with:"
	@echo "    ssh -N -L 3000:127.0.0.1:3000 <user>@<this-host>"
	@echo "  then open http://localhost:3000 and sign in as admin."
	@echo ""

.PHONY: observe-down
observe-down: ## Stop the observability stack; its data is preserved
	$(COMPOSE_OBSERVE) down

.PHONY: observe-logs
observe-logs: ## Follow the observability stack's own logs
	$(COMPOSE_OBSERVE) logs -f

.PHONY: deploy-backup
deploy-backup: ## Dump the database and the uploaded files into backups/
	@mkdir -p backups
	@ts=$$(date +%Y%m%d-%H%M%S); \
	$(COMPOSE_PROD) exec -T postgres pg_dump -U vocalis vocalis | gzip > backups/db-$$ts.sql.gz; \
	docker run --rm -v vocalis_vocalis-files:/data:ro -v "$$PWD/backups:/out" alpine \
		tar czf /out/files-$$ts.tar.gz -C /data . ; \
	echo "wrote backups/db-$$ts.sql.gz and backups/files-$$ts.tar.gz"
