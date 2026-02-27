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
  config/                  Types (ForgeProject, Environment, etc.), config R/W, project registry
  docker/                  Docker Compose operations, service status, forge-network connect
  system/                  System init (Docker network, Traefik, TLS certs)
  bind/                    Domain binding (/etc/hosts, Traefik dynamic config)
  ui/                      lipgloss styles and colors
```

## Data Directory

`~/.forge/` — created on first use by commands that need it (`forge init`, `forge project init`, etc.). Read-only commands like `--help` and `forge project list` do not create it. Existing files are never overwritten.
- `config.json` — user configuration (initialized as `{}`)
- `projects.json` — project registry (initialized as `[]`)
- `docker-compose.yml` — system-level compose file for Traefik (written by `forge init`)

## Project File

`.forgerc.json` — created in the current directory by `forge project init`. Stores project metadata and environment config:
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
      "myproject-frontend": { "port": 5173, "alias": null },
      "myproject-backend": { "port": 3000, "alias": "backend" }
    }
  }
}
```

- `environment.compose_file` — path to compose file (relative to project dir). Omit or leave empty for auto-detection (`compose.yaml` > `compose.yml` > `docker-compose.yml` > `docker-compose.yaml`).
- `environment.hooks` — shell commands run before/after native Docker Compose operations
- `environment.alias` — maps container/service names to Traefik routing rules:
  - `alias: null` → `<project-code>.local` (index, no subdomain)
  - `alias: "backend"` → `backend.<project-code>.local`
- Legacy `environment.commands` format is still supported with a deprecation warning

## Commands

```sh
go run .           # Run the CLI
go build -o forge  # Build binary
go mod tidy        # Sync dependencies
```

### `forge init`

Initialize the forge system infrastructure. Creates the `forge-network` Docker network and starts a Traefik reverse proxy container (ports 80/443). Idempotent — safe to run multiple times.

- Writes `~/.forge/docker-compose.yml` with the Traefik service
- Runs `docker compose up -d` to start Traefik

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

### `forge project bind` / `forge project unbind`

- No longer requires `sudo` — prompts for password internally when writing `/etc/hosts`
- Note: forge refuses to run as root (`sudo forge ...` is blocked)

### `forge project status`

- Shows Docker Compose service states and forge-network connectivity

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
