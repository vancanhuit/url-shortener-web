FROM node:22-bookworm AS tailwind
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build:css

FROM golang:1.25.0-trixie AS go
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=tailwind /app/assets/css/tailwind.css ./app/assets/css/
RUN go build -o bin/url-shortener ./cmd/web

FROM gcr.io/distroless/base-debian12:latest
COPY --from=go /app/bin/url-shortener /
USER nonroot:nonroot

ENTRYPOINT ["/url-shortener"]
