<p align="center">
  <img src="logo.png" alt="Forge" width="200">
</p>

# Forge

A CLI tool for managing local development projects with Docker Compose and Traefik. Forge gives every project clean local domains like `https://my-project.test`, handles service discovery on a shared Docker network, and optionally exposes services publicly via Cloudflare Tunnels.

## Features

- **Stays out of your way** — Run `docker compose up` however you like (foreground, watch, detached); then `forge project attach` wires containers into the shared network.
- **Local domains** — `forge project bind` writes `/etc/hosts` entries and Traefik routing so `https://my-project.test` just works.
- **HTTPS by default** — Wildcard TLS via [mkcert](https://github.com/FiloSottile/mkcert) with automatic HTTP-to-HTTPS redirect.
- **No changes to your compose file** — Services are connected to the shared network at runtime.
- **Path-based routing** — Route `/api` to a backend and `/` to a frontend on the same domain.
- **Cloudflare Tunnel support** — Expose services publicly with zero extra compose configuration.
- **Subdirectory support** — All commands auto-discover `.forgerc.json` by walking up the directory tree, so you can run `forge project attach` from any subdirectory.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) with Compose V2
- [mkcert](https://github.com/FiloSottile/mkcert) (optional, for HTTPS)
- [Go](https://go.dev/) 1.25+ (to build from source)

## Installation

```sh
git clone https://github.com/ivancerovina/forge.git
cd forge
go build -o forge
```

Move the binary somewhere in your `$PATH`:

```sh
sudo mv forge /usr/local/bin/
```

## Setup

Initialize forge once to create the Docker network and start Traefik:

```sh
forge setup
```

This is idempotent — safe to run again if something gets out of sync.

## New Project

```sh
cd ~/projects/my-app

# Create .forgerc.json (interactive)
forge project init

# Or non-interactive
forge project init -t "My App" -c my-app

# Add service aliases (auto-binds domains)
forge project alias add frontend --port 5173
forge project alias add backend --port 3000 --alias api --path /api

# Start your containers (forge does not do this for you)
docker compose up -d

# Wire them into forge-network so Traefik can route to them
forge project attach

# Open https://my-app.test
```

## Existing Project

If a project already has a `.forgerc.json` (e.g. cloned from a repo):

```sh
cd ~/projects/existing-app
forge project register
forge project bind             # once, to register routes + hosts entries
docker compose up -d           # start containers however you like
forge project attach           # connect them to forge-network
```

## Commands

All project commands work from any subdirectory within the project.

### System

| Command | Description |
|---------|-------------|
| `forge setup` | Create Docker network, start Traefik (one-time setup). `forge init` is a hidden alias |

### Project Management

| Command | Description |
|---------|-------------|
| `forge project init` | Create `.forgerc.json` in current directory |
| `forge project attach` (alias `link`) | Connect the project's running containers to `forge-network`. Run after `docker compose up`. Idempotent. |
| `forge project info` | Show project details, service states, and alias overview |
| `forge project bind` | Write `/etc/hosts` entries and Traefik routing config |
| `forge project unbind` | Remove `/etc/hosts` entries and Traefik config |
| `forge project register [path]` | Add project to the global registry |
| `forge project unregister [path]` | Remove project from the global registry |
| `forge project list` | List all registered projects |

Forge does not run `docker compose up` / `down` / `stop` — you do that yourself. After containers are running, `forge project attach` connects them to `forge-network` so Traefik can reach them.

**`project init` flags:**

| Flag | Description |
|------|-------------|
| `-t, --title` | Project name (required with `-c`) |
| `-c, --code` | Project code — lowercase, hyphens allowed (required with `-t`) |
| `-d, --description` | Project description |
| `-p, --path` | Directory to initialize in |
| `-r, --remote` | Git remote URL (initializes git repo and sets origin) |
| `--register` | Register project after init |
| `--no-register` | Skip registration prompt |
| `--force` | Overwrite existing `.forgerc.json` |

### Aliases

Aliases map compose service names to Traefik routing rules. Manage them with:

| Command | Description |
|---------|-------------|
| `forge project alias add <service> --port <port>` | Add a service alias |
| `forge project alias remove <service>` | Remove a service alias |
| `forge project alias info [service]` | Show alias details |

All alias commands also work interactively (run without arguments). `alias add` and `alias remove` automatically bind/unbind domains after changes.

**`alias add` flags:**

| Flag | Description |
|------|-------------|
| `--port, -P` | Service port (required) |
| `--alias, -a` | Subdomain (`api` becomes `api.<code>.test`). Omit for root domain |
| `--path` | Path prefix (e.g. `/api`) with automatic StripPrefix |
| `--http` | HTTP only (default is HTTPS) |
| `--cloudflare` | Also bind via Cloudflare tunnel |
| `--force` | Overwrite existing alias |

### Tunnel (Cloudflare)

| Command | Description |
|---------|-------------|
| `forge tunnel init` | Start cloudflared container (requires `$CLOUDFLARE_TUNNEL_TOKEN`) |
| `forge tunnel stop` | Stop and remove cloudflared container |
| `forge tunnel set-domain <domain>` | Set base domain (e.g. `dev.example.com`) |
| `forge tunnel info` | Show tunnel config and container state |

To expose a service publicly:

```sh
export CLOUDFLARE_TUNNEL_TOKEN="eyJ..."  # add to ~/.zshrc
forge tunnel set-domain dev.example.com
forge tunnel init

forge project alias add frontend --port 5173 --cloudflare --force
# Domains are auto-bound — access at https://my-app.dev.example.com
```

## Configuration

### `.forgerc.json`

Created by `forge project init` in the project root:

```json
{
  "name": "My Project",
  "description": "A web application",
  "code": "my-project",
  "environment": {
    "compose_file": "docker-compose.yml",
    "alias": [
      { "container": "frontend", "port": 5173, "alias": null, "cloudflare": true },
      { "container": "backend", "port": 3000, "alias": null, "path": "/api", "cloudflare": true },
      { "container": "docs", "port": 8080, "alias": "docs", "https": false }
    ]
  }
}
```

### Compose file

The `compose_file` field is optional. If omitted, forge auto-detects: `compose.yaml` > `compose.yml` > `docker-compose.yml` > `docker-compose.yaml`.

### Alias routing

Alias keys are Docker Compose service names. Given project code `my-project`:

| `alias` value | `path` | Resulting domain |
|---------------|--------|------------------|
| `null` (omit) | — | `my-project.test` |
| `null` | `/api` | `my-project.test/api` |
| `"docs"` | — | `docs.my-project.test` |

With `cloudflare: true` and domain `dev.example.com`, the same patterns apply to `my-project.dev.example.com`.

You do **not** need to add `forge-network` to your compose file. Forge connects services at runtime via `forge project attach` and registers DNS aliases so Traefik can reach them by service name.

## Development

```sh
go run .           # Run from source
go build -o forge  # Build binary
go mod tidy        # Sync dependencies
```

## License

MIT
