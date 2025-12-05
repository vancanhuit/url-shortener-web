SHELL := /bin/bash
BINARY_PATH ?= /tmp/url-shortener-web
VERSION ?= $(shell git describe --tags --always --dirty 2> /dev/null || echo unknown)
COMPOSE_FILE ?= compose.yaml
export COMPOSE_FILE

## help: Show this help message
.PHONY: help
help:
	@echo "Usage:"
	@sed -n 's/^##//p' "${MAKEFILE_LIST}" | column -t -s ':' | sed -e 's/^/ /'

## deps: Update Go and Node dependencies
.PHONY: deps
deps:
	go mod tidy
	npm install

## build: Build the Go application binary
.PHONY: build
build:
	go build -ldflags='-s -X main.version=$(VERSION)' -o $(BINARY_PATH) ./cmd/web


## css: Build the CSS assets using Tailwind CSS
.PHONY: css
css:
	npm run build:css

## test: Run Go tests with coverage
.PHONY: test
test:
	source ./scripts/docker-run-db.sh test_postgres test && go test -cover -v ./...

## lint: Run golangci-lint on the Go codebase
.PHONY: lint
lint:
	go tool golangci-lint run -v ./...


## govulncheck: Run Go vulnerability check
.PHONY: govulncheck
govulncheck:
	go tool govulncheck ./...

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
	docker compose build --build-arg=VERSION=$(VERSION)

## cert: Create locally-trusted development TLS certificates
.PHONY: cert
cert:
	@echo "Creating locally-trusted development certificates..."
	mkdir -pv ./tls
	mkcert -key-file ./tls/key.pem -cert-file ./tls/cert.pem localhost
