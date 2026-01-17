# Simple URL Shortener

[![Go Build Status](https://github.com/vancanhuit/url-shortener-web/actions/workflows/go.yaml/badge.svg)](https://github.com/vancanhuit/url-shortener-web/actions/workflows/go.yaml)

A minimal URL shortener web application built with Go and Tailwind CSS.

## Prerequisites

- [Go](https://go.dev)
- [Node.js](https://nodejs.org/)
- [Docker Engine](https://docs.docker.com/engine/install/)
- [Docker Compose](https://docs.docker.com/compose/)
- [`mkcert`](https://github.com/FiloSottile/mkcert) (for local HTTPS)
- [`make`](https://makefiletutorial.com/)
- [`Dagger`](https://dagger.io/)

## Setup

Install a local development CA for HTTPS:

```bash
mkcert -install
```

Install dependencies and build assets locally:

```bash
make deps
make css
```

Generate a development TLS certificate:
```bash
make cert
```

Development tasks:
```bash
make test
make golangci-lint
make govulncheck
make build-binary GOARCH=amd64
make build-binary GOARCH=arm64
make export-oci-tarball
make load-image-from-oci-tarball
docker container run --rm url-shortener-web:latest --version
```

## Running Locally

### Using Docker Compose

Build and start the services:

```bash
make compose/build

# Run HTTP server at http://localhost:8080
make compose/up/http
make compose/down

# Run HTTPS server at https://localhost:8080
make compose/up/https
make compose/down
```

### Using Go Directly

Start the database and run the web server:

```bash
# Run HTTP server at http://localhost:8080
go run ./cmd/web

# Run HTTPS server at https://localhost:8080
go run ./cmd/web -tls -port 8080 -base-url https://localhost:8080
```
