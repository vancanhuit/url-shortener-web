FROM node:24-bookworm AS tailwind
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build:css

FROM golang:1.25.0-bookworm AS go
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=tailwind /app/assets/css/tailwind.css ./assets/css/
ARG VERSION=unknown
RUN make build BINARY_PATH=/tmp/url-shortener-web VERSION=${VERSION}

FROM gcr.io/distroless/base-debian12:latest
COPY --from=go /tmp/url-shortener-web /
USER nonroot:nonroot

ENTRYPOINT ["/url-shortener-web"]
