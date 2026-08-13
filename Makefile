.DEFAULT_GOAL := help

COMPOSE := docker compose
GO      := go
PY_DIR  := services/ner-sentiment
PY_VENV := $(CURDIR)/$(PY_DIR)/.venv
PY      := $(PY_VENV)/bin/python

## ---- build (Go) ----------------------------------------------------------

.PHONY: build build-core build-ui build-rollup
build: build-core build-ui build-rollup ## build all three Go binaries into bin/

build-core: ## build cmd/core into bin/core
	$(GO) build -o bin/core ./cmd/core

build-ui: ## build cmd/ui into bin/ui
	$(GO) build -o bin/ui ./cmd/ui

build-rollup: ## build cmd/rollup into bin/rollup
	$(GO) build -o bin/rollup ./cmd/rollup

## ---- test / vet / fmt (Go) -------------------------------------------------
## test-core/test-ui/test-rollup mirror .drone.yml's three Go pipelines
## exactly, so a local failure here means CI will fail the same way.

.PHONY: test test-core test-ui test-rollup test-db
test: test-core test-ui test-rollup ## run all Go tests, service by service (matches CI)

test-core: ## build + vet + test cmd/core and internal/core
	$(GO) build ./cmd/core/... ./internal/core/...
	$(GO) vet ./cmd/core/... ./internal/core/...
	$(GO) test ./internal/core/...

test-ui: ## build + vet + test cmd/ui and internal/ui
	$(GO) build ./cmd/ui/... ./internal/ui/...
	$(GO) vet ./cmd/ui/... ./internal/ui/...
	$(GO) test ./internal/ui/...

test-rollup: ## build + vet + test cmd/rollup and internal/core
	$(GO) build ./cmd/rollup/... ./internal/core/...
	$(GO) vet ./cmd/rollup/... ./internal/core/...
	$(GO) test ./internal/core/...

test-db: ## run the full Go suite against a real Postgres/Meilisearch (run 'make up-deps' first)
	TEST_DATABASE_URL="postgres://whatisgoing:whatisgoing@localhost:5432/whatisgoing?sslmode=disable" \
	TEST_MEILISEARCH_URL="http://localhost:7700" \
	TEST_MEILISEARCH_KEY="dev-master-key" \
	$(GO) test ./...

.PHONY: fmt fmt-check vet lint
fmt: ## gofmt -w every Go file
	gofmt -l -w .

fmt-check: ## fail if any Go file isn't gofmt'd (what CI should run before merging)
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "$$unformatted"; \
		echo "not gofmt'd — run 'make fmt'"; \
		exit 1; \
	fi

vet: ## go vet ./... (all three services at once)
	$(GO) vet ./...

lint: ## golangci-lint run ./... — local convenience, not wired into .drone.yml yet
	golangci-lint run ./...

## ---- test (Python / ner-sentiment) -----------------------------------------
## Mirrors .drone.yml's ner-sentiment pipeline (torch CPU wheel installed
## separately from requirements-dev.txt, same as there).

.PHONY: py-install test-py
py-install: ## create services/ner-sentiment/.venv and install its deps + spaCy model
	python3 -m venv $(PY_VENV)
	$(PY) -m pip install --upgrade pip
	$(PY) -m pip install torch --index-url https://download.pytorch.org/whl/cpu
	$(PY) -m pip install -r $(PY_DIR)/requirements-dev.txt
	$(PY) -m spacy download en_core_web_sm

test-py: ## run services/ner-sentiment's test suite (run 'make py-install' first)
	cd $(PY_DIR) && $(PY) -m pytest -q

## ---- local stack (docker compose) ------------------------------------------

.PHONY: up up-deps down logs run-rollup
up: ## docker compose up --build (core, ui, ner-sentiment, postgres, meilisearch)
	$(COMPOSE) up --build

up-deps: ## start just postgres + meilisearch, for 'make test-db' or running a binary by hand
	$(COMPOSE) up -d postgres meilisearch

down: ## docker compose down
	$(COMPOSE) down

logs: ## tail logs from the running compose stack
	$(COMPOSE) logs -f

run-rollup: ## run cmd/rollup once against the compose stack, then exit
	$(COMPOSE) run --rm rollup

## ---- misc -------------------------------------------------------------------

.PHONY: clean help
clean: ## remove build artifacts, the ner-sentiment venv, and Python caches
	rm -rf bin/
	rm -rf $(PY_VENV)
	find . -name '__pycache__' -type d -prune -exec rm -rf {} +

help: ## show this help
	@grep -hE '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*##"}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
