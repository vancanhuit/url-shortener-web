SHELL := /bin/bash
COMPOSE_FILE ?= compose.yaml
export COMPOSE_FILE

GO_VERSION ?= 1.25.5
NODE_VERSION ?= 24.12.0
GOLANGCI_LINT_VERSION ?= v2.8.0

GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

APP_VERSION ?= $(shell git describe --tags --always --dirty 2> /dev/null || echo unknown)
DIST ?= dist
BINARY_NAME ?= url-shortener-web
BINARY_PATH ?= $(DIST)/$(BINARY_NAME)-$(GOOS)-$(GOARCH)-$(APP_VERSION)
OCI_TARBALL_PATH ?= $(DIST)/$(BINARY_NAME)-$(APP_VERSION).tar

## help: Show this help message
.PHONY: help
help:
	@echo "Usage:"
	@sed -n 's/^##//p' "${MAKEFILE_LIST}" | column -t -s ':' | sed -e 's/^/ /'

$(DIST):
	mkdir -p $@

## deps: Update Go and Node dependencies
.PHONY: deps
deps:
	go mod tidy
	npm ci

## build: Build the Go application binary
.PHONY: build
build: $(DIST)
	dagger call build-go-binary --src=. --go-version=$(GO_VERSION) --node-version=$(NODE_VERSION) --ldflags='-s -w -X main.version=$(APP_VERSION)' --output=$(BINARY_PATH)

## export-oci: Export image as an OCI tarball
.PHONY: export-oci
export-oci: $(DIST)
	dagger call export-oci --src=. --go-version=$(GO_VERSION) --node-version=$(NODE_VERSION) --app-version=$(APP_VERSION) --output=$(OCI_TARBALL_PATH)

## load-image-from-oci-tarball: Load Docker image from OCI tarball
.PHONY: load-image-from-oci-tarball
load-image-from-oci-tarball:
	skopeo copy oci-archive:$(OCI_TARBALL_PATH) docker-daemon:$(BINARY_NAME):latest

## css: Build the CSS assets using Tailwind CSS
.PHONY: css
css:
	npm run build:css

## test: Run Go tests with coverage
.PHONY: test
test:
	dagger call test --src=. --node-version=$(NODE_VERSION) --go-version=$(GO_VERSION)

## lint: Run golangci-lint on the Go codebase
.PHONY: lint
lint:
	dagger call lint --src=. --go-version=$(GO_VERSION) --node-version=$(NODE_VERSION) --golangci-lint-version=$(GOLANGCI_LINT_VERSION)

## govulncheck: Run Go vulnerability check
.PHONY: govulncheck
govulncheck:
	dagger call govulncheck --src=. --go-version=$(GO_VERSION)

## clean: Clean up Docker containers and prune unused resources
.PHONY: clean
clean:
	docker container rm -f test_postgres 2> /dev/null || true
	docker system prune -f
	docker volume prune -f

## compose/down: Stop and remove Docker Compose services
.PHONY: compose/down
compose/down:
	docker compose down -v

## compose/up: Start Docker Compose services in detached mode
.PHONY: compose/up
compose/up:
	docker compose up -d

## compose/build: Build Docker Compose services with version argument
.PHONY: compose/build
compose/build:
	docker compose build --build-arg=APP_VERSION=$(APP_VERSION)

## cert: Create locally-trusted development TLS certificates
.PHONY: cert
cert:
	@echo "Creating locally-trusted development certificates..."
	mkdir -pv ./tls
	mkcert -key-file ./tls/key.pem -cert-file ./tls/cert.pem localhost
