# Forge

CLI tool for managing projects in a development environment. Built with Go and the Charmbracelet stack.

## Tech Stack

- **Language:** Go (1.25+)
- **CLI Framework:** [urfave/cli v3](https://github.com/urfave/cli) (command routing, flags, help generation)
- **Interactive UI:** [charmbracelet/huh](https://github.com/charmbracelet/huh) (forms, prompts — interactive mode only)
- **Styling:** [lipgloss](https://github.com/charmbracelet/lipgloss) (terminal colors and formatting)
- **Module path:** `github.com/ivancerovina/forge`

## Project Structure

Single-module Go project.

```
main.go                    CLI entry point (urfave/cli app, action functions, display helpers)
internal/
  config/                  Types (ForgeProject, Environment, etc.), config R/W, project registry, walk-up .forgerc.json discovery
  docker/                  Docker Compose operations, service status, forge-network connect
  system/                  System init (Docker network, Traefik, TLS certs)
  bind/                    Domain binding (/etc/hosts, Traefik dynamic config)
  ui/                      lipgloss styles and colors
```

## Data Directory

`~/.forge/` — created on first use by commands that need it (`forge init`, `forge project init`, etc.). Read-only commands like `--help` and `forge project list` do not create it. Existing files are never overwritten.
- `config.json` — global configuration (`cloudflare_domain`, `cloudflare_tunnel` flag)
- `projects.json` — project registry (initialized as `[]`)
- `docker-compose.yml` — system-level compose file for Traefik + optional cloudflared (written by `forge init`)
- `cf-config.yml` — cloudflared ingress config (written by `forge tunnel init`)

## Project File

`.forgerc.json` — created in the current directory by `forge project init`. Stores project metadata and environment config. Most commands auto-discover `.forgerc.json` by walking up the directory tree from the current directory, stopping at a `.git` boundary or the user's home directory. This means commands like `forge start` work from any subdirectory within a project.
```json
{
  "name": "My Project",
  "description": "Some description",
  "code": "my-project",
  "environment": {
    "compose_file": "docker-compose.yml",
    "hooks": {
      "pre_start": [],
      "post_start": [],
      "pre_stop": [],
      "post_stop": [],
      "pre_destroy": [],
      "post_destroy": []
    },
    "alias": {
      "myproject-frontend": { "port": 5173, "alias": null, "cloudflare": true },
      "myproject-backend": { "port": 3000, "alias": "backend", "path": "/api" }
    }
  }
}
```

- `environment.compose_file` — path to compose file (relative to project dir). Omit or leave empty for auto-detection (`compose.yaml` > `compose.yml` > `docker-compose.yml` > `docker-compose.yaml`).
- `environment.hooks` — shell commands run before/after native Docker Compose operations
- `environment.alias` — maps container/service names to Traefik routing rules:
  - `alias: null` → `<project-code>.local` (index, no subdomain)
  - `alias: "backend"` → `backend.<project-code>.local`
  - `path: "/api"` → path prefix routing with StripPrefix middleware
  - `cloudflare: true` → also creates a public Traefik router via Cloudflare tunnel
- Legacy `environment.commands` format is still supported with a deprecation warning

## Build & Install

The `forge` binary in this directory is symlinked to the user's PATH — it IS the globally installed `forge` command. **NEVER delete it.** To rebuild:

```sh
go build -o forge  # Rebuild binary (overwrites in place, preserves symlink)
go run .           # Run without building
go mod tidy        # Sync dependencies
```

### `forge init`

Initialize the forge system infrastructure. Creates the `forge-network` Docker network and starts a Traefik reverse proxy container (ports 80/443). If a Cloudflare tunnel is enabled, also starts the cloudflared container. Idempotent — safe to run multiple times.

- Writes `~/.forge/docker-compose.yml` with the Traefik service (+ cloudflared if tunnel enabled)
- Runs `docker compose up -d` to start the stack

### `forge project init`

Initialize a new forge project in the current directory (creates `.forgerc.json`).

- **Interactive:** `forge project init` — launches a huh form for name, description, code
- **Non-interactive:** `forge project init -t "Name" -c "code"` or `forge project init --title "Name" --code "code" --description "desc"`
- Flags: `-t`/`--title` (required with -c), `-c`/`--code` (required with -t), `-d`/`--description` (optional)
- `--register` / `--no-register` — control project registration without interactive prompt
- `--force` — skip overwrite confirmation
- Prompts before overwriting an existing `.forgerc.json`
- Note: non-interactive mode (`-t`/`-c`) no longer launches any interactive prompts

### `forge start` / `forge stop` / `forge destroy`

- `forge start` — runs `docker compose up -d`, auto-connects services to forge-network, shows status
- `forge stop` — runs `docker compose stop`
- `forge destroy` — runs `docker compose down`
- All three run pre/post hooks if configured
- All three work from any subdirectory within the project (walks up to find `.forgerc.json`)

### `forge project bind` / `forge project unbind`

- Generates Traefik routing config for all aliases (local `.local` domains + Cloudflare public domains)
- Only local domains are added to `/etc/hosts` (public CF domains are routed through the tunnel)
- No longer requires `sudo` — prompts for password internally when writing `/etc/hosts`
- Note: forge refuses to run as root (`sudo forge ...` is blocked)
- Works from any subdirectory within the project

### `forge project status`

- Shows Docker Compose service states and forge-network connectivity
- Works from any subdirectory within the project

### `forge project alias add` / `forge project alias remove` / `forge project alias info`

- `forge project alias add <service> --port <port>` — add a service alias (supports `--alias`, `--path`, `--http`, `--cloudflare`, `--force`)
- `forge project alias remove <service>` — remove a service alias
- `forge project alias info [service]` — show alias details (single or all)
- All three support interactive mode (run without arguments)
- All three work from any subdirectory within the project; `alias add` and `alias remove` write `.forgerc.json` back to the discovered project root

### `forge tunnel init`

Initialize the Cloudflare tunnel. Requires `$CLOUDFLARE_TUNNEL_TOKEN` to be set in the environment.

- Sets `cloudflare_tunnel: true` in `~/.forge/config.json`
- Writes `~/.forge/cf-config.yml` with catch-all ingress to `http://forge-traefik:80`
- Adds cloudflared container to the system compose file
- Starts the container

### `forge tunnel stop`

Stop and remove the cloudflared container.

- Clears the tunnel flag from config
- Regenerates compose file without cloudflared
- Removes the orphaned container

### `forge tunnel set-domain <domain>`

Set the Cloudflare base domain (e.g. `dev.example.com`). Aliases with `cloudflare: true` will generate public Traefik routers using this domain.

### `forge tunnel info`

Show current tunnel configuration: domain, enabled status, container state.

## Theme

Purple color palette. All styled output uses lipgloss with these colors:
- Primary: `#9D4EDD` (purple) — titles, emphasis
- Secondary: `#7B2CBF` (dim purple) — section headings
- Text: `#E0E0E0` (white) — commands, key terms
- Muted: `#6C6C6C` (dim) — descriptions, secondary text
- Error: `#FF6B6B` (red) — error messages

## Code Conventions

- Standard Go formatting (`gofmt` / `goimports`)
- Handle errors explicitly
- Business logic packages (`config`, `docker`, `system`, `bind`) return errors — they never call `os.Exit()` or print to stdout/stderr
- Only `main.go` handles output formatting and exit codes
- Use `huh` for interactive prompts; never use `huh` in non-interactive (flag-driven) code paths
- Use `lipgloss` for all styled/colored terminal output
- Keep the main package thin; extract logic into internal packages as the project grows
- Forge must not run as root; commands needing elevation use `sudo` internally for the specific operation
