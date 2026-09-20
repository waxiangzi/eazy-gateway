# eazy-gateway

A web-based SSH tunnel manager built with Go and Vue 3.

## Features

- **SSH Host Management** — centrally manage SSH server configs (host, port, user, key) independently from tunnels
- **Tunnel Types** — local port forwarding, remote port forwarding, dynamic SOCKS5 proxy, and HTTP-to-SOCKS5 proxy
- **Traffic Statistics** — real-time byte counters and per-tunnel traffic trend charts
- **SSH Keys** — Ed25519 key pairs encrypted at rest with the admin password; auto-generates a default key on first run
- **Admin Authentication** — session-based login with cookie or Bearer token support
- **CLI Interface** — control tunnels from the terminal without opening the browser (`list`, `start`, `stop`, `restart`, `status`, `reset-password`)
- **Auto-Restore** — automatically reconnects tunnels that were running before the last shutdown
- **Graceful Shutdown** — cleans up SSH connections and saves traffic counters on SIGTERM/SIGINT
- **Single Static Binary** — embeds the Vue SPA into the Go binary for easy distribution
- **One-Command Install** — `./build/eazy-gateway --install` sets up a user-level systemd service
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
separately. A tunnel references a host by ID. Private keys are stored as files
encrypted at rest and referenced by path in the database.

**Tunnel Types**

| Type | Description |
|------|-------------|
| `local` | Local port forwarding (listen locally, forward through SSH to a remote target) |
| `remote` | Remote port forwarding (listen on the SSH server, forward back to a local target) |
| `dynamic` | Dynamic SOCKS5 proxy over the SSH connection |
| `httpToSocks5` | Local HTTP proxy that relays through an upstream SOCKS5 proxy; supports CONNECT tunnelling, direct HTTP relay, and per-domain routing rules (`*.example.com` wildcards) |

## Build

Requires [just](https://github.com/casey/just), Node.js 22+, and Go 1.26+.

```bash
just build        # build web + Go binary → build/eazy-gateway
just build-web    # build Vue SPA only
just dev          # backend hot-reload via air
just docker-build # build Docker image
just clean        # remove build artifacts
```

## Run

### Quick Start

```bash
./build/eazy-gateway                # default: port 8022, data ./data
./build/eazy-gateway --port 9000    # custom port
PORT=9000 ./build/eazy-gateway      # custom port via environment variable
./build/eazy-gateway --data /var/lib/eazy-gateway  # custom data directory
```

### CLI Commands

When an eazy-gateway server is already running, the binary acts as a CLI client via a local Unix socket:

```bash
# List all tunnels
eazy-gateway list

# Start / stop / restart a tunnel
eazy-gateway start <tunnel-id>
eazy-gateway stop <tunnel-id>
eazy-gateway restart <tunnel-id>

# Check tunnel status
eazy-gateway status <tunnel-id>

# Reset admin password (generates a new random password)
eazy-gateway reset-password
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8022` (env `PORT`) | HTTP server port |
| `--data` | `./data` (env `DATA_DIRECTORY`) | Persistent data directory (DB, keys, logs) |
| `--serve` | `false` | Run in background (daemon mode) |
| `--install` | `false` | Install as a user-level systemd service and start it |
| `--secure-cookies` | `false` | Set `Secure` flag on session cookies (use when behind TLS proxy) |

### systemd Install

The recommended way to run persistently on Linux:

```bash
sudo cp build/eazy-gateway /usr/local/bin/eazy-gateway
eazy-gateway --install
```

This creates a user service at `~/.config/systemd/user/eazy-gateway.service`, enables it,
and starts it immediately.

Alternatively, manually:

```bash
cp build/eazy-gateway /usr/local/bin/eazy-gateway
sudo cp eazy-gateway.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now eazy-gateway
```

### Docker

```bash
# Build
docker build -t eazy-gateway .

# Run
docker run -d \
  -p 8022:8022 \
  -v eazy-gateway-data:/data \
  eazy-gateway \
  --port 8022 --data /data
```

## Development

### Backend + Frontend

```bash
cd web && npm install && npm run build   # build SPA once
go run ./cmd/eazy-gateway                 # serves web/dist from disk
```

Run Go tests:

```bash
go test ./internal/...
```

### Frontend Only (Vite HMR)

```bash
cd web && npm install && npm run dev
```

The dev server proxies API requests to `http://localhost:8022` by default (see `web/vite.config.js`).

### Backend Hot-Reload

```bash
just dev
```

Uses [air](https://github.com/air-verse/air) to auto-rebuild the Go binary on file changes.

## Configuration

Configuration is controlled via command-line flags (and the `PORT` / `DATA_DIRECTORY`
environment variables). Example:

```bash
./build/eazy-gateway --port 9000 --data ./data --secure-cookies
```

A `config.example.yaml` exists for reference but is **not loaded at runtime** — copy values
from it into your deployment scripts or systemd unit as needed.

First boot behavior:
- Creates `data/eazy-gateway.db` (0600) for hosts, tunnels, settings, and admin hash.
- Generates an Ed25519 SSH key pair if no keys exist: the private key is encrypted with
  the admin password at `data/keys/default` (0600) and the public half at `data/keys/default.pub`.
- Generates an admin password and saves it to `data/initial-password.txt` (0600). It is
  never written to the log, so read it from that file — under `--serve` the log is a
  plain file and the password doubles as the key-store encryption password.

### Key store

Private keys are sealed with a key derived from the admin password, so powering the
service off means the key store starts **locked** and tunnels are restored only after
an admin login. Only files inside the key store are ever written, so a key record that
points elsewhere (a hand-added path under `~/.ssh`, say) is read as-is and left
untouched by password changes and legacy migration.
Changing the password re-encrypts every stored key in one pass; the
CLI `reset-password` cannot do that while the store is locked, so it reports a warning
and the stored keys become unusable. To recover, delete each host that references such
a key (and its tunnels), then delete the key: a fresh default key is generated
automatically and can be assigned to new hosts.

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
| DELETE | `/api/keys/{id}` | Delete a key and its files (409 if referenced by a host) |

> Keys are auto-generated on first run and there is no HTTP upload endpoint. Deleting
> the last key immediately generates a new default key so the console stays usable.
> The private key files are encrypted with the admin password; see [Key store](#key-store).

### Settings

| Method | Path | Description |
|--------|------|-------------|
| PUT | `/api/settings` | Update appName and trafficTrendHours (1–24) |

All `/api/*` routes (except `/api/login`, `/api/settings` GET, and `/health`) require an
authenticated session via cookie or `Authorization: Bearer <token>` header.

## Project Structure

```
.
├── cmd/eazy-gateway/        # Go entry point + embed/noembed build tags
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
├── eazy-gateway.service     # Example systemd unit file
└── config.example.yaml      # Example configuration reference
```

## License

MIT
