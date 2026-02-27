<p align="center">
  <img src="logo.png" alt="Forge" width="200">
</p>

# Forge

A CLI tool for managing local development projects with Docker Compose and Traefik. Forge handles project scaffolding, environment lifecycle, automatic service discovery on a shared Docker network, and local HTTPS — so you can access your projects at clean domains like `https://my-project.local`.

## Why Forge

Most local dev setups involve juggling `docker compose up`, manually wiring containers to a shared network, editing `/etc/hosts`, and configuring a reverse proxy. Forge handles all of that:

- **One command to start** — `forge start` runs Docker Compose, connects every service to a shared network, and shows you their status.
- **Local domains that just work** — `forge project bind` writes `/etc/hosts` entries and generates Traefik routing config so `https://my-project.local` points to the right container and port.
- **HTTPS by default** — Wildcard TLS certificates via [mkcert](https://github.com/FiloSottile/mkcert) with automatic HTTP-to-HTTPS redirect.
- **No changes to your compose file** — Forge connects services to the shared network at runtime. Your `docker-compose.yml` stays clean.
- **Project registry** — Track all your forge projects across directories with `forge project list`.
- **Lifecycle hooks** — Run arbitrary commands before/after start, stop, and destroy operations.

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

## Quick Start

```sh
# 1. Initialize forge (creates Docker network + starts Traefik)
forge init

# 2. Go to a project that has a docker-compose.yml
cd ~/projects/my-app

# 3. Initialize the forge project
forge project init -t "My App" -c my-app

# 4. Start the environment
forge start

# 5. Add aliases to .forgerc.json, then bind domains
#    (edit .forgerc.json — see "Alias configuration" below)
forge project bind

# 6. Open https://my-app.local in your browser
```

## Commands

### `forge init`

Initialize the forge system infrastructure. This is a one-time setup that:

1. Creates the `forge-network` Docker network
2. Generates local TLS certificates with mkcert (if installed)
3. Writes a system-level `docker-compose.yml` to `~/.forge/`
4. Starts a Traefik reverse proxy container (ports 80 and 443)

Idempotent — safe to run multiple times.

### `forge project init`

Initialize a new forge project in the current directory. Creates a `.forgerc.json` file with project metadata and an empty environment config.

**Interactive mode** (launches a form):

```sh
forge project init
```

**Non-interactive mode** (flag-driven, no prompts):

```sh
forge project init -t "My App" -c my-app
forge project init -t "My App" -c my-app -d "A web application"
forge project init -t "My App" -c my-app -r git@github.com:user/repo.git
```

| Flag | Description |
|------|-------------|
| `-t, --title` | Project name (required with `-c`) |
| `-c, --code` | Project code — lowercase, hyphens allowed (required with `-t`) |
| `-d, --description` | Project description |
| `-p, --path` | Directory to initialize in (defaults to cwd) |
| `-r, --remote` | Git remote URL (initializes git repo and sets origin) |
| `--register` | Register the project in the global registry after init |
| `--no-register` | Skip the registration prompt (interactive mode) |
| `--force` | Overwrite an existing `.forgerc.json` without confirmation |

### `forge start`

Start the project environment. Runs from the project directory (where `.forgerc.json` lives).

1. Runs `pre_start` hooks
2. Runs `docker compose up -d`
3. Connects all services to `forge-network` (with DNS aliases so Traefik can reach them by service name)
4. Runs `post_start` hooks
5. Displays service status

### `forge stop`

Stop the project environment.

1. Runs `pre_stop` hooks
2. Runs `docker compose stop`
3. Runs `post_stop` hooks

Containers are stopped but not removed — `forge start` will restart them.

### `forge destroy`

Tear down the project environment.

1. Runs `pre_destroy` hooks
2. Runs `docker compose down`
3. Runs `post_destroy` hooks

Containers and project networks are removed.

### `forge project status`

Show the state of each Docker Compose service and whether it's connected to `forge-network`.

```
Services:

  ● ✓ web    running
  ● ✓ api    running
  ○ – db     not created
```

- `●` / `○` — running/stopped indicator
- `✓` / `–` — connected to forge-network or not

### `forge project register` / `forge project unregister`

Add or remove a project from the global registry (`~/.forge/projects.json`).

```sh
forge project register                # Register current directory
forge project register -p ~/my-app    # Register a specific path
forge project unregister              # Unregister current directory
```

### `forge project list`

List all registered projects with their names and paths.

```sh
forge project list
```

### `forge project bind`

Configure local domain routing for the project. Reads `environment.alias` from `.forgerc.json` and:

1. Adds `/etc/hosts` entries pointing each domain to `127.0.0.1`
2. Writes a Traefik dynamic config file to `~/.forge/traefik/<code>.yml`
3. Regenerates TLS certificates to cover project-specific wildcard domains

Forge prompts for your password internally (via `sudo tee`) when writing `/etc/hosts`. Do not run forge itself with `sudo`.

### `forge project unbind`

Remove domain routing for the project. Deletes the `/etc/hosts` entries and Traefik config created by `bind`.

## Project Configuration

Each project has a `.forgerc.json` in its root:

```json
{
  "name": "My Project",
  "description": "A web application",
  "code": "my-project",
  "environment": {
    "compose_file": "docker-compose.yml",
    "hooks": {
      "pre_start": ["echo 'Starting...'"],
      "post_start": [],
      "pre_stop": [],
      "post_stop": [],
      "pre_destroy": [],
      "post_destroy": []
    },
    "alias": {
      "frontend": { "port": 5173, "alias": null },
      "backend": { "port": 3000, "alias": "api" }
    }
  }
}
```

### Compose file resolution

The `compose_file` field is optional. If omitted, forge looks for compose files in this order:

1. `compose.yaml`
2. `compose.yml`
3. `docker-compose.yml`
4. `docker-compose.yaml`

### Hooks

Shell commands that run before and after Docker Compose operations. Each hook is an array of commands executed sequentially via `sh -c`. If any hook command fails, the operation stops.

| Hook | Runs |
|------|------|
| `pre_start` | Before `docker compose up -d` |
| `post_start` | After containers start and connect to forge-network |
| `pre_stop` | Before `docker compose stop` |
| `post_stop` | After containers stop |
| `pre_destroy` | Before `docker compose down` |
| `post_destroy` | After containers are removed |

### Alias configuration

The `alias` map controls how services are exposed through Traefik. Keys are Docker Compose **service names** — the same names defined under `services:` in your compose file.

```json
"alias": {
  "frontend": { "port": 5173, "alias": null },
  "backend": { "port": 3000, "alias": "api" },
  "docs": { "port": 8080, "alias": "docs", "https": false }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `port` | number | The port the container listens on internally |
| `alias` | string or null | `null` = root domain (`<code>.local`), string = subdomain (`<alias>.<code>.local`) |
| `https` | boolean | Route via HTTPS with TLS (default: `true`). Set `false` for HTTP-only |

For the example above with project code `my-project`:

| Service | Domain | Protocol |
|---------|--------|----------|
| `frontend` | `my-project.local` | HTTPS |
| `backend` | `api.my-project.local` | HTTPS |
| `docs` | `docs.my-project.local` | HTTP |

HTTPS-enabled routes automatically redirect HTTP requests to HTTPS (301 permanent redirect).

### Network connectivity

You do **not** need to add `forge-network` to your compose file. When you run `forge start`, forge automatically connects each service to `forge-network` and registers DNS aliases matching the service name. This is how Traefik resolves `http://frontend:5173` or `http://backend:3000` from its routing config.

Services that are only internal (like a database) can be excluded by simply not adding them to the `alias` map. They remain accessible to other services through the compose-internal network as usual.

### Legacy `commands` format

Older projects may use `environment.commands` instead of hooks with native compose:

```json
"environment": {
  "commands": {
    "start": ["docker compose up -d"],
    "stop": ["docker compose stop"],
    "destroy": ["docker compose down"]
  }
}
```

This still works but prints a deprecation warning. Migrate to `hooks` + native compose for better integration (auto network connect, service status, etc).

## Data Directory

Forge stores its data in `~/.forge/`:

```
~/.forge/
├── config.json          # User configuration
├── projects.json        # Registered project paths
├── docker-compose.yml   # Traefik service definition
├── traefik/             # Traefik dynamic configuration
│   ├── _tls.yml         # TLS certificate config
│   └── <project>.yml    # Per-project routing rules
└── certs/               # mkcert TLS certificates
    ├── local.pem
    └── local-key.pem
```

Created on first use by commands that need it (`forge init`, `forge project init`, etc). Read-only commands like `forge project list` will not create it if it doesn't exist.

## Development

```sh
go run .           # Run from source
go build -o forge  # Build binary
go mod tidy        # Sync dependencies
```

## License

MIT
