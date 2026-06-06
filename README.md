# tun-console

A web-based SSH tunnel manager built with Go and Vue 3.

## Features

- **SSH Host Management** — centrally manage SSH server configs (host, port, user, key) independently from tunnels
- **Tunnel Types** — local port forwarding, remote port forwarding, and dynamic SOCKS5 proxy
- **Traffic Statistics** — real-time byte counters and per-tunnel traffic trend charts
- **SSH Keys** — Ed25519 key pairs stored on filesystem; auto-generates a default key on first run
- **Admin Authentication** — session-based login with cookie or Bearer token support
- **CLI Interface** — control tunnels from the terminal without opening the browser (`list`, `start`, `stop`, `restart`, `status`, `reset-password`)
- **Auto-Restore** — automatically reconnects tunnels that were running before the last shutdown
- **Graceful Shutdown** — cleans up SSH connections and saves traffic counters on SIGTERM/SIGINT
- **Single Static Binary** — embeds the Vue SPA into the Go binary for easy distribution
- **One-Command Install** — `./build/tun-console --install` sets up a user-level systemd service
- **Docker Support** — multi-stage Dockerfile with distroless runtime image
- **Multi-Language UI** — Chinese and English (Vue I18n)

## Architecture

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────┐
│   Vue 3 SPA     │────▶│  Go HTTP API │────▶│  bbolt DB   │
│  (embedded)     │     │   + Engine   │     │  (embedded) │
└─────────────────┘     └──────────────┘     └─────────────┘
                               │
                               ▼
                        ┌──────────────┐
                        │  SSH Client  │
                        │ (crypto/ssh) │
                        └──────────────┘
```

**Data Model** — `Host` (SSH server config) and `Tunnel` (forwarding rule) are managed
separately. A tunnel references a host by ID. Keys are stored as files on disk and
referenced by path in the database.

## Build

Requires [just](https://github.com/casey/just), Node.js 22+, and Go 1.26+.

```bash
just build        # build web + Go binary → build/tun-console
just build-web    # build Vue SPA only
just dev          # backend hot-reload via air
just docker-build # build Docker image
just clean        # remove build artifacts
```

> **Note:** The project migrated from `make` to `just`. `just` accepts the same invocation style (`just build`).

## Run

### Quick Start

```bash
./build/tun-console              # default: port 3100, data ./data
./build/tun-console --port 8080  # custom port
./build/tun-console --data /var/lib/tun-console  # custom data directory
```

### CLI Commands

When a tun-console server is already running, the binary acts as a CLI client via a local Unix socket:

```bash
# List all tunnels
tun-console list

# Start / stop / restart a tunnel
tun-console start <tunnel-id>
tun-console stop <tunnel-id>
tun-console restart <tunnel-id>

# Check tunnel status
tun-console status <tunnel-id>

# Reset admin password (generates a new random password)
tun-console reset-password
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `3100` | HTTP server port |
| `--data` | `./data` | Persistent data directory (DB, keys, logs) |
| `--serve` | `false` | Run in background (daemon mode) |
| `--install` | `false` | Install as a user-level systemd service and start it |
| `--secure-cookies` | `false` | Set `Secure` flag on session cookies (use when behind TLS proxy) |

### systemd Install

The recommended way to run persistently on Linux:

```bash
sudo cp build/tun-console /usr/local/bin/tun-console
tun-console --install
```

This creates a user service at `~/.config/systemd/user/tun-console.service`, enables it,
and starts it immediately.

Alternatively, manually:

```bash
cp build/tun-console /usr/local/bin/tun-console
sudo cp tun-console.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now tun-console
```

### Docker

```bash
# Build
docker build -t tun-console .

# Run
docker run -d \
  -p 3100:3100 \
  -v tun-console-data:/data \
  tun-console \
  --port 3100 --data /data
```

## Development

### Backend + Frontend

```bash
cd web && npm install && npm run build   # build SPA once
go run ./cmd/tun-console                  # serves web/dist from disk
```

Run Go tests:

```bash
go test ./internal/...
```

### Frontend Only (Vite HMR)

```bash
cd web && npm install && npm run dev
```

The dev server proxies API requests to `http://localhost:3100` by default (see `web/vite.config.js`).

### Backend Hot-Reload

```bash
just dev
```

Uses [air](https://github.com/air-verse/air) to auto-rebuild the Go binary on file changes.

## Configuration

Configuration is controlled via command-line flags. Example:

```bash
./build/tun-console --port 8080 --data ./data --secure-cookies
```

A `config.example.yaml` exists for reference but is **not loaded at runtime** — copy values
from it into your deployment scripts or systemd unit as needed.

First boot behavior:
- Creates `data/tun-console.db` (bbolt) for hosts, tunnels, settings, and admin hash.
- Generates Ed25519 SSH key pair at `data/keys/default` if no keys exist.
- Prints the auto-generated admin password to stdout and saves it to `data/initial-password.txt`.

## API Endpoints

### Public

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/api/settings` | Read app settings (appName, trafficTrendHours) |

### Auth

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/login` | Admin login (returns session cookie) |
| POST | `/api/logout` | Destroy session |
| GET | `/api/me` | Check current session |

### Admin

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/admin/change-password` | Change admin password (invalidates all sessions) |

### Hosts

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/hosts` | List all SSH host configs |
| POST | `/api/hosts` | Create a host config |
| GET | `/api/hosts/{id}` | Get a host config |
| PUT | `/api/hosts/{id}` | Update a host config |
| DELETE | `/api/hosts/{id}` | Delete a host config (409 if referenced by a tunnel) |
| POST | `/api/hosts/test` | Test SSH connectivity with the given host config |

### Tunnels

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/tunnels` | List tunnels with runtime status and traffic totals |
| POST | `/api/tunnels` | Create a tunnel config |
| GET | `/api/tunnels/{id}` | Get a tunnel config with status and traffic |
| PUT | `/api/tunnels/{id}` | Update a tunnel config |
| DELETE | `/api/tunnels/{id}` | Delete a tunnel config (stops if running) |
| POST | `/api/tunnels/{id}/start` | Start a tunnel |
| POST | `/api/tunnels/{id}/stop` | Stop a tunnel and persist traffic |
| GET | `/api/tunnels/{id}/status` | Get tunnel status (also exposed on CLI socket) |
| GET | `/api/tunnels/{id}/traffic/trend?hours=N` | Traffic delta points for charting (N = 1..48) |

### Keys

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/keys` | List SSH keys (id, name, publicKey) |
| DELETE | `/api/keys/{id}` | Delete a key (409 if referenced by a host) |

> Keys are auto-generated on first run. There is no HTTP upload endpoint; new keys can be
> added by placing PEM files in the `data/keys/` directory and registering them in the DB.

### Settings

| Method | Path | Description |
|--------|------|-------------|
| PUT | `/api/settings` | Update appName and trafficTrendHours (1–24) |

All `/api/*` routes (except `/api/login`, `/api/settings` GET, and `/health`) require an
authenticated session via cookie or `Authorization: Bearer <token>` header.

## Project Structure

```
.
├── cmd/tun-console/         # Go entry point + embed/noembed build tags
├── internal/
│   ├── api/                 # HTTP handlers (auth, hosts, tunnels, keys, settings, admin)
│   ├── crypto/              # Password hashing, key generation, key encryption
│   ├── db/                  # bbolt persistence layer
│   ├── ssh/                 # SSH client, tunnel engine, forwarding logic
│   └── bbolt_shim/          # bbolt compatibility module
├── web/                     # Vue 3 SPA
│   ├── src/
│   │   ├── views/           # Page components
│   │   ├── components/      # Shared UI components
│   │   ├── stores/          # Pinia stores
│   │   ├── router/          # Vue Router config
│   │   └── i18n/            # Translation files (zh, en)
│   └── vite.config.js
├── Dockerfile               # Multi-stage build (node → golang → distroless)
├── justfile                 # Build automation
├── tun-console.service      # Example systemd unit file
└── config.example.yaml      # Example configuration reference
```

## License

MIT
