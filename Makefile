# ITAM developer Makefile (PowerShell)
# Requires: docker (+ compose), go 1.25+, node 20+, PowerShell 5+.

SHELL          := powershell.exe
.SHELLFLAGS    := -NoProfile -Command

COMPOSE        := docker compose -f deploy/docker-compose.yml
MIGRATIONS_DIR := backend/migrations
# Migrations run as the superuser (needed for extensions + Vault). The
# supabase/postgres image's superuser is supabase_admin, not postgres.
DATABASE_URL   ?= postgres://supabase_admin:postgres@localhost:5600/postgres?sslmode=disable
# In-docker URL: service name `db`, internal port 5432 (5600 is only the host mapping).
GOOSE_DB_URL   ?= postgres://supabase_admin:postgres@db:5432/postgres?sslmode=disable
GOOSE_IMAGE    ?= itam-migrate
GOOSE_NETWORK  ?= itam_default
GOOSE_CMD      := powershell -NoProfile -ExecutionPolicy Bypass -File scripts/goose-docker.ps1

# Backing services only (all pulled images, never built). GoTrue auto-migrates
# its own auth schema, so it belongs here too.
INFRA_SERVICES := db valkey nats rustfs auth
INFRA_VOLUMES  := itam_db_data itam_valkey_data itam_nats_data itam_rustfs_data
PSQL           := $(COMPOSE) exec db psql -U supabase_admin -d postgres

.PHONY: help
help: ## Show this help
	@Select-String -Path 'Makefile' -Pattern '^[a-zA-Z_-]+:.*?##' | ForEach-Object { if ($$_.Line -match '^([^:]+):.*?## (.+)$$') { Write-Host ("  {0,-20} {1}" -f $$Matches[1], $$Matches[2]) } }

## ---- Full stack (docker) ----
.PHONY: up
up: ## Build and start the whole stack
	$(COMPOSE) up -d --build

.PHONY: down
down: ## Stop the stack
	$(COMPOSE) down

.PHONY: reset
reset: ## Stop the stack and wipe all volumes (DESTRUCTIVE)
	$(COMPOSE) down -v

.PHONY: logs
logs: ## Tail logs
	$(COMPOSE) logs -f

.PHONY: ps
ps: ## Show running services
	$(COMPOSE) ps

## ---- Infra only: backing services, NO image builds (for local dev) ----
## After `make infra-up`, run `make migrate-up` to create the app schema.
.PHONY: infra-up
infra-up: ## Start infra only (db, valkey, nats, rustfs, auth) - no builds
	$(COMPOSE) up -d --no-build $(INFRA_SERVICES)

.PHONY: infra-down
infra-down: ## Stop and remove infra containers (keeps data volumes)
	$(COMPOSE) rm -fs $(INFRA_SERVICES)

.PHONY: infra-clean
infra-clean: ## Stop infra and wipe its data volumes (DESTRUCTIVE)
	$(COMPOSE) rm -fsv $(INFRA_SERVICES)
	-docker volume rm $(INFRA_VOLUMES)

.PHONY: infra-logs
infra-logs: ## Tail infra logs
	$(COMPOSE) logs -f $(INFRA_SERVICES)

.PHONY: infra-ps
infra-ps: ## Show infra service status
	$(COMPOSE) ps $(INFRA_SERVICES)

## ---- Migrations (goose) ----
.PHONY: migrate-build
migrate-build: ## Build the goose runner image (once, or after Dockerfile changes)
	$(COMPOSE) build migrate

.PHONY: migrate-up
migrate-up: migrate-build ## Apply all pending migrations
	$(GOOSE_CMD) up

.PHONY: migrate-down
migrate-down: ## Roll back the last migration
	$(GOOSE_CMD) down

.PHONY: migrate-status
migrate-status: ## Show migration status
	$(GOOSE_CMD) status

.PHONY: migrate-reset
migrate-reset: ## Roll back everything (DESTRUCTIVE)
	$(GOOSE_CMD) reset

.PHONY: migrate-create
migrate-create: migrate-build ## Create a new migration: make migrate-create name=add_widgets
	$(GOOSE_CMD) create $(name)

.PHONY: psql
psql: ## Open psql in the db container (requires infra-up)
	$(PSQL)

## ---- Local app: run natively against `infra-up`, NO docker build ----
## Typical flow:  make infra-up  &&  make migrate-up
## then in two terminals:  make dev-api   and   make dev-web
.PHONY: dev-api
dev-api: ## Run the Go API natively (hot env defaults target infra ports)
	cd backend; go run ./cmd/api

.PHONY: dev-web
dev-web: ## Run the SvelteKit dev server natively (http://localhost:5608)
	cd frontend; npm run dev

.PHONY: dev-web-install
dev-web-install: ## Install frontend deps (run once before dev-web)
	cd frontend; npm install

## ---- Backend (build/test helpers) ----
.PHONY: tidy
tidy: ## go mod tidy
	cd backend; go mod tidy

.PHONY: build
build: ## Build the API binary
	cd backend; go build ./...

.PHONY: run
run: dev-api ## Alias for dev-api

.PHONY: test
test: ## Run backend tests
	cd backend; go test ./...

## ---- Frontend (aliases) ----
.PHONY: web-install
web-install: dev-web-install ## Alias for dev-web-install

.PHONY: web
web: dev-web ## Alias for dev-web
