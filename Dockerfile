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
COPY --from=web-builder /web/dist/ ./cmd/eazy-gateway/dist/
RUN CGO_ENABLED=0 go build -tags embed -o /eazy-gateway ./cmd/eazy-gateway/

# Stage 3: Minimal runtime image
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go-builder /eazy-gateway /usr/local/bin/eazy-gateway
COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 8022
ENTRYPOINT ["/usr/local/bin/eazy-gateway"]
