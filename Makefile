SHELL := /bin/bash
COMPOSE_FILE ?= compose.yaml
export COMPOSE_FILE

GO_VERSION ?= 1.25.5
NODE_VERSION ?= 24.12.0
GOLANGCI_LINT_VERSION ?= v2.8.0

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

GIT_REPO ?= $(shell git config --get remote.origin.url | \
  sed -E 's#(git@|https://)(.+[:/])([^/]+/[^/.]+)(\.git)?#\3#')

APP_VERSION ?= $(shell git describe --tags --always --dirty=-dev 2> /dev/null || echo unknown)
LDFLAGS ?= "-s -w -X main.version=$(APP_VERSION)"
DIST ?= dist
BINARY_NAME ?= $(shell basename $(GIT_REPO))
BINARY_PATH ?= $(DIST)/$(BINARY_NAME)-$(GOOS)-$(GOARCH)-$(APP_VERSION)
OCI_TARBALL_PATH ?= $(DIST)/$(BINARY_NAME)-$(APP_VERSION).tar

IMAGE_REGISTRY ?= ghcr.io
IMAGE_REPO ?= $(IMAGE_REGISTRY)/${GIT_REPO}
IMAGE_TAGS ?= latest

## help: Show this help message
.PHONY: help
help:
	@echo "Usage:"
	@sed -n 's/^##//p' "${MAKEFILE_LIST}" | column -t -s ':' | sed -e 's/^/ /'

$(DIST):
	mkdir -pv $@

## deps: Update Go and Node dependencies
.PHONY: deps
deps:
	go mod tidy
	npm ci

## build: Build the Go application binary
.PHONY: build-binary
build-binary: $(DIST)
	dagger call build-binary \
			--src=. \
			--go-version=$(GO_VERSION) \
			--node-version=$(NODE_VERSION) \
			--goos=$(GOOS) \
			--goarch=$(GOARCH) \
			--ldflags=$(LDFLAGS) \
			--output=$(BINARY_PATH)

## export-oci: Export image as an OCI tarball
.PHONY: export-oci-tarball
export-oci-tarball: $(DIST)
	dagger call export-oci-tarball \
			--src=. \
			--go-version=$(GO_VERSION) \
			--node-version=$(NODE_VERSION) \
			--ldflags=$(LDFLAGS) \
			--output=$(OCI_TARBALL_PATH)

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
	dagger call lint \
			--src=. \
			--go-version=$(GO_VERSION) \
			--node-version=$(NODE_VERSION) \
			--golangci-lint-version=$(GOLANGCI_LINT_VERSION)

## govulncheck: Run Go vulnerability check
.PHONY: govulncheck
govulncheck:
	dagger call govulncheck --src=. --go-version=$(GO_VERSION)

.PHONY: push-image
push-image:
	@test -n "$(REGISTRY_USER)" || (echo "REGISTRY_USER is required"; exit 1)
	@test -n "$(REGISTRY_TOKEN)" || (echo "REGISTRY_TOKEN is required"; exit 1)
	dagger call push-image \
					--src=. \
					--node-version=$(NODE_VERSION) \
					--go-version=$(GO_VERSION) \
					--ldflags=$(LDFLAGS) \
					--repo=$(IMAGE_REPO) \
					--tags=$(IMAGE_TAGS) \
					--username=$(REGISTRY_USER) \
					--token=env://REGISTRY_TOKEN

## clean: Clean up Docker containers and prune unused resources
.PHONY: clean
clean:
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
