# Aurelia: Beastbound — task runner.
#   make            list targets
#   make quick      Node-only checks (no Go toolchain needed)
#   make test       full Go suite (race + coverage)
#   make all        quick + test + demo
# Works in WSL / git-bash / macOS / Linux / CI.

GO        ?= go
NODE      ?= node
COVERFILE := cover.out
MIN_COV   := 70

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n",$$1,$$2}'

## ---- Node-only checks (no Go required) -----------------------------------
.PHONY: quick
quick: verify-engine verify-session verify-backend gen ## Run all Node reference checks

.PHONY: verify-engine
verify-engine: ## Engine math parity (damage, stats, type matrix)
	$(NODE) test/reference/verify_engine.js

.PHONY: verify-session
verify-session: ## Full turn-loop parity (status, faint->switch, victory)
	$(NODE) test/reference/verify_session.mjs

.PHONY: verify-backend
verify-backend: ## Auth (JWT) + trade + AI + save parity
	$(NODE) test/reference/verify_backend.mjs

.PHONY: gen
gen: ## Regenerate the 300-creature seed + balance audit
	$(NODE) tools/creaturegen/generate.mjs --seed 42 --count 300 --out data/creatures/seed.json

## ---- Go ------------------------------------------------------------------
.PHONY: tidy
tidy: ## Resolve deps + write go.sum (run once)
	$(GO) mod tidy

.PHONY: vet
vet: ## go vet
	$(GO) vet ./...

.PHONY: test
test: ## Full Go suite with race detector + coverage
	$(GO) test -race -covermode=atomic -coverprofile=$(COVERFILE) ./...

.PHONY: cover
cover: test ## Show coverage and enforce the minimum
	@$(GO) tool cover -func=$(COVERFILE) | tail -1
	@pct=$$($(GO) tool cover -func=$(COVERFILE) | tail -1 | awk '{print $$3}' | tr -d '%'); \
	  echo "coverage: $${pct}% (min $(MIN_COV)%)"; \
	  awk -v p="$$pct" -v m=$(MIN_COV) 'BEGIN{exit (p<m)}'

.PHONY: demo
demo: ## Play a full two-bot PvP match end-to-end
	$(GO) run ./services/battle/cmd/demo

.PHONY: build
build: ## Build all service binaries into ./bin
	@mkdir -p bin
	$(GO) build -o bin/ ./services/...

## ---- Compose / aggregate -------------------------------------------------
.PHONY: up
up: ## Local full stack (Postgres + Redis + services)
	docker compose -f deploy/docker/docker-compose.yml up --build

.PHONY: ci
ci: vet test cover quick ## What CI runs

.PHONY: all
all: quick vet test demo ## Everything sensible in one go

.PHONY: clean
clean: ## Remove build/coverage artifacts
	rm -rf bin $(COVERFILE)
