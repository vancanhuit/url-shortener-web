# Simple URL Shortener

[![Go Build Status](https://github.com/vancanhuit/url-shortener-web/actions/workflows/go.yaml/badge.svg)](https://github.com/vancanhuit/url-shortener-web/actions/workflows/go.yaml)

A minimal URL shortener web application built with Go and Tailwind CSS.

## Prerequisites

- [Go](https://go.dev)
- [Docker Engine](https://docs.docker.com/engine/install/)
- [Docker Compose](https://docs.docker.com/compose/)
- [Node.js](https://nodejs.org/)
- [`mkcert`](https://github.com/FiloSottile/mkcert) (for local HTTPS)
- [`make`](https://makefiletutorial.com/)

## Setup

Install a local development CA for HTTPS:

```bash
mkcert -install
```

Install dependencies and build assets:

```bash
make deps      # Install Go and Node.js dependencies
make css       # Build Tailwind CSS
make test      # Run Go tests
make lint      # Run golangci-lint
make govulncheck # Run vulnerability check
make build     # Build Go binary
make cert      # Generate development certificates
make clean     # Clean up Docker resources
```

## Running Locally

### Using Docker Compose

Build and start the services:

```bash
make compose/build   # Build Docker images

# Run HTTP server at http://localhost:8080
make compose/up      # Start services
make compose/down    # Stop services

# To run HTTPS server at https://localhost:8080
export COMPOSE_FILE=compose.yaml:compose.https.yaml
make compose/up
make compose/down
```

### Using Go Directly

Start the database and run the web server:

```bash
source ./scripts/docker-run-db.sh

# Run HTTP server at http://localhost:8080
go run ./cmd/web

# Or run HTTPS server at https://localhost:8080
go run ./cmd/web -tls -port 8080 -base-url https://localhost:8080
```
