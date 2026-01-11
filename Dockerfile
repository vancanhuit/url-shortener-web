# syntax=docker/dockerfile:1

ARG NODE_VERSION=24.12.0
ARG GO_VERSION=1.25.5

ARG BUILDPLATFORM
FROM --platform=${BUILDPLATFORM} node:${NODE_VERSION} AS tailwind
WORKDIR /src
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build:css

ARG BUILDPLATFORM
FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION} AS go
WORKDIR /go/src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=tailwind /src/assets/css/ ./assets/css/

ARG TARGETOS
ARG TARGETARCH
ARG APP_VERSION=unknown
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X main.version=${APP_VERSION}" -o /go/bin/web ./cmd/web

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=go /go/bin/web /web
USER nonroot:nonroot

ENTRYPOINT ["/web"]
