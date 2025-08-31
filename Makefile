SHELL := /bin/bash

.PHONY: test clean lint govulncheck run deps css
deps:
	@go mod tidy
	@npm install

css:
	@npm run build:css

test:
	@source ./scripts/docker-run-db.sh test_postgres test && go test -cover -v ./...

lint:
	@go tool golangci-lint run -v ./...

govulncheck:
	@go tool govulncheck ./...

clean:
	@docker container rm -f test_postgres 2> /dev/null || true
	@docker container rm -f dev_postgres 2> /dev/null || true
	@docker image prune -f
