# Forge

A CLI tool for managing local development projects with Docker and Traefik. Forge handles project scaffolding, environment lifecycle, service discovery, and automatic HTTPS — so you can access your projects at clean local domains like `https://my-project.local`.

## Features

- **Project scaffolding** — Interactive or flag-driven project initialization with `.forgerc.json`
- **Environment lifecycle** — Start, stop, and destroy project environments with custom commands
- **Local domain routing** — Automatic `/etc/hosts` entries and Traefik reverse proxy configuration
- **HTTPS by default** — Wildcard TLS certificates via `mkcert` for `*.local` domains
- **Project registry** — Track all your forge projects across directories
- **Service status** — See which Docker Compose services are running and connected to the forge network

## Prerequisites

- [Go](https://go.dev/) 1.25+
- [Docker](https://docs.docker.com/get-docker/) with Compose V2
- [mkcert](https://github.com/FiloSottile/mkcert) (optional, for HTTPS)

## Installation

```sh
git clone https://github.com/ivancerovina/forge.git
cd forge
go build -o forge
```

Move the binary to somewhere in your `$PATH`:

```sh
sudo mv forge /usr/local/bin/
```

## Quick Start

```sh
# 1. Initialize forge infrastructure (Docker network + Traefik)
forge init

# 2. Create a new project
cd ~/projects/my-app
forge project init

# 3. Configure your .forgerc.json with start commands and aliases

# 4. Start your environment
forge start

# 5. Bind local domains (requires sudo)
sudo forge project bind

# 6. Open https://my-app.local in your browser
```

## Commands

### `forge init`

Initialize the forge system infrastructure. Creates the `forge-network` Docker network and starts a Traefik reverse proxy container on ports 80 and 443. Generates local TLS certificates if `mkcert` is installed.

Safe to run multiple times (idempotent).

### `forge project init`

Initialize a new forge project in the current directory.

```sh
# Interactive mode
forge project init

# Non-interactive mode
forge project init -t "My App" -c my-app
forge project init -t "My App" -c my-app -d "Description" -r git@github.com:user/repo.git
```

| Flag | Description |
|------|-------------|
| `-p, --path` | Directory to initialize in (defaults to cwd) |
| `-t, --title` | Project name (required with `-c`) |
| `-c, --code` | Project code (required with `-t`) |
| `-d, --description` | Project description |
| `-r, --remote` | Git remote URL (implies git init) |

### `forge start` / `forge stop` / `forge destroy`

Run the corresponding commands defined in `.forgerc.json`:

```sh
forge start    # Runs environment.commands.start
forge stop     # Runs environment.commands.stop
forge destroy  # Runs environment.commands.destroy
```

### `forge project register` / `forge project unregister`

Add or remove a project from the global registry (`~/.forge/projects.json`).

```sh
forge project register              # Register current directory
forge project register -p ~/myapp   # Register a specific path
forge project unregister
```

### `forge project list`

List all registered projects with their names and paths.

### `forge project status`

Show Docker Compose service status for the current project, including which services are connected to the `forge-network`.

### `forge project bind` / `forge project unbind`

Configure or remove local domain routing. Requires `sudo`.

```sh
sudo forge project bind    # Add /etc/hosts entries + Traefik config
sudo forge project unbind  # Remove /etc/hosts entries + Traefik config
```

## Project Configuration

Each project has a `.forgerc.json` in its root directory:

```json
{
  "name": "My Project",
  "description": "A web application",
  "code": "my-project",
  "environment": {
    "commands": {
      "start": ["docker compose up -d"],
      "stop": ["docker compose stop"],
      "destroy": ["docker compose down"]
    },
    "alias": {
      "myproject-frontend": { "port": 5173, "alias": null },
      "myproject-backend": { "port": 3000, "alias": "backend" }
    }
  }
}
```

### Alias routing

The `environment.alias` section maps Docker container/service names to local domains:

| Alias value | Resulting domain |
|-------------|-----------------|
| `null` | `<code>.local` (root domain) |
| `"backend"` | `backend.<code>.local` |

Each alias entry has:
- `port` — The container port to proxy to
- `alias` — Subdomain prefix (`null` for root)
- `https` (optional) — Set to `false` to use HTTP only (defaults to HTTPS)

## Data Directory

Forge stores its data in `~/.forge/`:

```
~/.forge/
├── config.json          # User configuration
├── projects.json        # Registered project paths
├── docker-compose.yml   # Traefik service definition
├── traefik/             # Traefik dynamic configuration files
│   ├── _tls.yml         # TLS certificate config
│   └── <project>.yml    # Per-project routing rules
└── certs/               # mkcert TLS certificates
    ├── local.pem
    └── local-key.pem
```

## Development

```sh
go run .           # Run from source
go build -o forge  # Build binary
go mod tidy        # Sync dependencies
```

## License

MIT
