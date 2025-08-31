SHELL := /bin/bash
BINARY_PATH ?= /tmp/url-shortener-web

.PHONY: build test clean lint govulncheck css deps
deps:
	go mod tidy
	npm install

build:
	go build -ldflags='-s' -o $(BINARY_PATH) ./cmd/web

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
	docker container rm -f dev_postgres 2> /dev/null || true
	docker image prune -f
