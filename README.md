# Simple URL Shortener

[![Go](https://github.com/vancanhuit/url-shortener-web/actions/workflows/go.yaml/badge.svg)](https://github.com/vancanhuit/url-shortener-web/actions/workflows/go.yaml)

- Install [Go](https://go.dev).
- Install [Docker](https://docs.docker.com).
- Install [Docker Compose](https://docs.docker.com/compose/).
- Install [Node.js](https://nodejs.org/).

```bash
make deps # Install go and node dependencies
make css # Build Tailwind CSS
make test # Run Go test
make lint # Run golangci-lint
make govulncheck # Run vulnerability check
make build # Build Go binary
make clean # Clean up
```

Run the web locally using Docker Compose:
```bash
make compose/build # Build docker image
make compose/up # Run services
make compose/down # Shutdown services
```

Alternatively with `go run` command:
```bash
source ./scripts/docker-run-db.sh
go run ./cmd/web
```
