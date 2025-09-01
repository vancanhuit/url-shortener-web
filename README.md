# Simple URL Shortener

[![Go](https://github.com/vancanhuit/url-shortener-web/actions/workflows/go.yaml/badge.svg)](https://github.com/vancanhuit/url-shortener-web/actions/workflows/go.yaml)

- [Go](https://go.dev).
- [Docker Engine](https://docs.docker.com/engine/install/).
- [Docker Compose](https://docs.docker.com/compose/).
- [Node.js](https://nodejs.org/).
- [`mkcert`](https://github.com/FiloSottile/mkcert).
- [`make`](https://makefiletutorial.com/).

```bash
make deps # Install go and node dependencies
make css # Build Tailwind CSS
make test # Run Go test
make lint # Run golangci-lint
make govulncheck # Run vulnerability check
make build # Build Go binary
make cert # Create locally-trusted development certificates
make clean # Clean up Docker resources
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
go run ./cmd/web -tls -port 8080 -base-url https://localhost:8080
```

Access: [https://localhost:8080](https://localhost:8080).
