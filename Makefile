SHELL := /bin/bash
BINARY_PATH ?= /tmp/url-shortener-web
VERSION ?= $(shell git describe --tags --always --dirty 2> /dev/null || echo unknown)

.PHONY: build test clean lint govulncheck css deps
deps:
	go mod tidy
	npm install

build:
	go build -ldflags='-s -X main.version=$(VERSION)' -o $(BINARY_PATH) ./cmd/web

css:
	npm run build:css

test:
	source ./scripts/docker-run-db.sh test_postgres test && go test -cover -v ./...

lint:
	go tool golangci-lint run -v ./...

govulncheck:
	go tool govulncheck ./...

clean:
	docker container rm -f test_postgres 2> /dev/null || true
	docker image prune -f
