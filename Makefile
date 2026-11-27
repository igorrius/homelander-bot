# Homelander build targets. Run `make help` for the list.
SHELL := /bin/bash
BINARY := homelander

.PHONY: help web build test lint run docker clean

help: ## List targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-8s %s\n", $$1, $$2}'

web/node_modules: web/package-lock.json
	cd web && npm ci
	@touch web/node_modules

web: web/node_modules ## Build the web app into web/dist
	cd web && npm run build

build: web ## Build the binary with the web app embedded
	CGO_ENABLED=0 go build -trimpath -o $(BINARY) ./cmd/homelander

test: web/node_modules ## Run Go and web tests
	go test -race ./...
	cd web && npm test

lint: web/node_modules ## Run linters and the type check
	go vet ./...
	golangci-lint run
	cd web && npm run lint && npm run typecheck

run: build ## Build and run with variables from .env
	set -a; [ -f .env ] && . ./.env; set +a; ./$(BINARY)

docker: ## Build the container image
	docker build -t homelander:dev .

clean: ## Remove build output
	rm -f $(BINARY)
	find web/dist -mindepth 1 ! -name .gitkeep -delete
