<p align="center">
  <img src="logo.png" alt="Forge" width="200">
</p>

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

### Connecting services to the forge network

For Traefik to route traffic to your containers, they must be on the `forge-network`. Add it to your project's `docker-compose.yml`:

```yaml
services:
  frontend:
    image: node:20
    networks:
      - forge-network
      - internal

  backend:
    image: node:20
    networks:
      - forge-network

  database:
    image: postgres:16
    networks:
      - internal  # not exposed through forge

networks:
  forge-network:
    external: true
  internal:
    driver: bridge
```

Services on `forge-network` are reachable by Traefik and can be assigned local domains via aliases. Services **not** on the network (like the database above) remain internal and inaccessible from the browser.

`forge project status` shows a `✓` next to services connected to the forge network and `–` for those that aren't.

### Domain binding

`forge project bind` reads the `environment.alias` section from `.forgerc.json` and does two things:

1. **Adds `/etc/hosts` entries** — maps each domain to `127.0.0.1` so your browser resolves them locally
2. **Writes Traefik config** — creates a routing file at `~/.forge/traefik/<code>.yml` that tells Traefik how to proxy each domain to the correct container and port

`forge project unbind` reverses both steps.

### Alias configuration

The keys in `environment.alias` are Docker Compose **service names** (the container names Traefik uses to reach them on the forge network). Each entry configures how that service is exposed:

```json
"alias": {
  "myproject-frontend": { "port": 5173, "alias": null },
  "myproject-backend": { "port": 3000, "alias": "backend" },
  "myproject-api": { "port": 8080, "alias": "api", "https": false }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `port` | number | The port the container listens on. Traefik proxies to `http://<service>:<port>`. |
| `alias` | string or null | Controls the domain name. `null` = root domain (`<code>.local`), a string = subdomain (`<alias>.<code>.local`). |
| `https` | boolean (optional) | Whether to route through the HTTPS entrypoint with TLS. Defaults to `true`. Set to `false` for HTTP-only. |

For the example above with project code `my-project`:

| Service | Domain | Protocol |
|---------|--------|----------|
| `myproject-frontend` | `my-project.local` | HTTPS |
| `myproject-backend` | `backend.my-project.local` | HTTPS |
| `myproject-api` | `api.my-project.local` | HTTP |

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
