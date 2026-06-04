# Stage 1: Build frontend assets
FROM node:22-alpine AS web-builder
WORKDIR /web
COPY web/ ./
RUN npm install && npm run build

# Stage 2: Build Go binary
FROM golang:1.26-alpine AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
COPY internal/bbolt_shim/ ./internal/bbolt_shim/
RUN go mod download
COPY . .
COPY --from=web-builder /web/dist/ ./cmd/tun-console/dist/
RUN CGO_ENABLED=0 go build -tags embed -o /tun-console ./cmd/tun-console/

# Stage 3: Minimal runtime image
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go-builder /tun-console /usr/local/bin/tun-console
COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/usr/local/bin/tun-console"]
