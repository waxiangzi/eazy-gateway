# tun-console

A web-based SSH tunnel manager built with Go and Vue 3.

## Features

- Manage SSH tunnels (local, remote, dynamic/SOCKS5) through a web UI
- Store SSH keys encrypted at rest
- Admin authentication with session cookies
- Graceful shutdown on SIGTERM/SIGINT
- Single static binary with embedded Vue SPA

## Build

Build the frontend and Go binary into a single static binary:

```bash
just build
```

This runs:
1. `cd web && npm run build` — builds the Vue SPA into `web/dist`
2. `go build -tags embed -o build/tun-console ./cmd/tun-console` — embeds `web/dist` into the Go binary

> **注意：** 项目已从 `make` 迁移至 `just`。如有旧习惯，`just` 接受与 `make` 相同的参数风格（如 `just build`）。

## Run

```bash
./build/tun-console
```

Or with flags:

```bash
./build/tun-console -listen :8080 -data ./data
```

Environment variables:
- `LISTEN_ADDRESS` — HTTP listen address (default `:8080`)
- `DATA_DIRECTORY` — persistent data directory (default `./data`)

## Development

Build and run without embedding (serves `web/dist` from disk):

```bash
cd web && npm install && npm run build
go run ./cmd/tun-console
```

Run tests:

```bash
go test ./internal/...
```

## systemd

Copy the binary to `/usr/local/bin/tun-console` and the service file:

```bash
cp build/tun-console /usr/local/bin/tun-console
sudo cp tun-console.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now tun-console
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/api/login` | Admin login |
| POST | `/api/logout` | Logout |
| GET | `/api/me` | Session check |
| POST | `/api/admin/change-password` | Change admin password |
| GET | `/api/tunnels` | List tunnels |
| POST | `/api/tunnels` | Create tunnel |
| GET | `/api/tunnels/{id}` | Get tunnel |
| PUT | `/api/tunnels/{id}` | Update tunnel |
| DELETE | `/api/tunnels/{id}` | Delete tunnel |
| POST | `/api/tunnels/{id}/start` | Start tunnel |
| POST | `/api/tunnels/{id}/stop` | Stop tunnel |
| GET | `/api/keys` | List SSH keys |
| POST | `/api/keys` | Add SSH key |
| DELETE | `/api/keys/{id}` | Remove SSH key |
