<p align="center">
  <img src="logo.png" alt="Forge" width="200">
</p>

# Forge

A CLI tool for managing local development projects with Docker Compose and Traefik. Forge gives every project clean local domains like `https://my-project.local`, handles service discovery on a shared Docker network, and optionally exposes services publicly via Cloudflare Tunnels.

## Features

- **One command to start** — `forge start` runs Docker Compose, connects services to a shared network, and shows status.
- **Local domains** — `forge project bind` writes `/etc/hosts` entries and Traefik routing so `https://my-project.local` just works.
- **HTTPS by default** — Wildcard TLS via [mkcert](https://github.com/FiloSottile/mkcert) with automatic HTTP-to-HTTPS redirect.
- **No changes to your compose file** — Services are connected to the shared network at runtime.
- **Path-based routing** — Route `/api` to a backend and `/` to a frontend on the same domain.
- **Cloudflare Tunnel support** — Expose services publicly with zero extra compose configuration.
- **Subdirectory support** — All commands auto-discover `.forgerc.json` by walking up the directory tree, so you can run `forge start` from any subdirectory.
- **Lifecycle hooks** — Run commands before/after start, stop, and destroy.

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
forge init
```

This is idempotent — safe to run again if something gets out of sync.

## New Project

```sh
cd ~/projects/my-app

# Create .forgerc.json (interactive)
forge project init

# Or non-interactive
forge project init -t "My App" -c my-app

# Add service aliases (keys are your compose service names)
forge project alias add frontend --port 5173
forge project alias add backend --port 3000 --alias api --path /api

# Start and bind domains
forge start
forge project bind

# Open https://my-app.local
```

## Existing Project

If a project already has a `.forgerc.json` (e.g. cloned from a repo), just register and start it:

```sh
cd ~/projects/existing-app
forge project register
forge start
forge project bind
```

## Commands

All project commands work from any subdirectory within the project.

### System

| Command | Description |
|---------|-------------|
| `forge init` | Create Docker network, start Traefik (one-time setup) |

### Lifecycle

| Command | Description |
|---------|-------------|
| `forge start` | `docker compose up -d`, connect services to forge-network, show status |
| `forge stop` | `docker compose stop` (containers kept, restart with `forge start`) |
| `forge destroy` | `docker compose down` (containers and networks removed) |

All three run pre/post hooks if configured.

### Project Management

| Command | Description |
|---------|-------------|
| `forge project init` | Create `.forgerc.json` in current directory |
| `forge project status` | Show service states and forge-network connectivity |
| `forge project bind` | Write `/etc/hosts` entries and Traefik routing config |
| `forge project unbind` | Remove `/etc/hosts` entries and Traefik config |
| `forge project register [path]` | Add project to the global registry |
| `forge project unregister [path]` | Remove project from the global registry |
| `forge project list` | List all registered projects |

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

All alias commands also work interactively (run without arguments).

**`alias add` flags:**

| Flag | Description |
|------|-------------|
| `--port, -P` | Service port (required) |
| `--alias, -a` | Subdomain (`api` becomes `api.<code>.local`). Omit for root domain |
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
forge project bind
# Access at https://my-app.dev.example.com
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
    "hooks": {
      "pre_start": ["echo 'Starting...'"],
      "post_start": []
    },
    "alias": {
      "frontend": { "port": 5173, "alias": null, "cloudflare": true },
      "backend": { "port": 3000, "alias": null, "path": "/api", "cloudflare": true },
      "docs": { "port": 8080, "alias": "docs", "https": false }
    }
  }
}
```

### Compose file

The `compose_file` field is optional. If omitted, forge auto-detects: `compose.yaml` > `compose.yml` > `docker-compose.yml` > `docker-compose.yaml`.

### Alias routing

Alias keys are Docker Compose service names. Given project code `my-project`:

| `alias` value | `path` | Resulting domain |
|---------------|--------|------------------|
| `null` (omit) | — | `my-project.local` |
| `null` | `/api` | `my-project.local/api` |
| `"docs"` | — | `docs.my-project.local` |

With `cloudflare: true` and domain `dev.example.com`, the same patterns apply to `my-project.dev.example.com`.

You do **not** need to add `forge-network` to your compose file. Forge connects services at runtime and registers DNS aliases so Traefik can reach them by service name.

### Hooks

Shell commands that run before/after Docker Compose operations. Executed sequentially via `sh -c` from the project root. If any command fails, the operation stops.

| Hook | Runs |
|------|------|
| `pre_start` / `post_start` | Before/after `docker compose up -d` |
| `pre_stop` / `post_stop` | Before/after `docker compose stop` |
| `pre_destroy` / `post_destroy` | Before/after `docker compose down` |

## Development

```sh
go run .           # Run from source
go build -o forge  # Build binary
go mod tidy        # Sync dependencies
```

## License

MIT
