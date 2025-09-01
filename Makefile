SHELL := /bin/bash
BINARY_PATH ?= /tmp/url-shortener-web
VERSION ?= $(shell git describe --tags --always --dirty 2> /dev/null || echo unknown)
COMPOSE_FILE ?= compose.yaml
export COMPOSE_FILE

.PHONY: deps
deps:
	go mod tidy
	npm install

.PHONY: build
build:
	go build -ldflags='-s -X main.version=$(VERSION)' -o $(BINARY_PATH) ./cmd/web

.PHONY: css
css:
	npm run build:css

.PHONY: test
test:
	source ./scripts/docker-run-db.sh test_postgres test && go test -cover -v ./...

.PHONY: lint
lint:
	go tool golangci-lint run -v ./...

.PHONY: govulncheck
govulncheck:
	go tool govulncheck ./...

.PHONY: clean
clean:
	docker container rm -f test_postgres 2> /dev/null || true
	docker system prune -f
	docker volume prune -f

.PHONY: compose/down
compose/down:
	docker compose down -v

.PHONY: compose/up
compose/up:
	docker compose up -d

.PHONY: compose/build
compose/build:
	docker compose build --build-arg=VERSION=$(VERSION)

.PHONY: cert
cert:
	@echo "Creating locally-trusted development certificates..."
	mkdir -pv ./tls
	mkcert -key-file ./tls/key.pem -cert-file ./tls/cert.pem localhost
