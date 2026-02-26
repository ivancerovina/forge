# Forge

CLI tool for managing projects in a development environment. Built with Go and the Charmbracelet stack.

## Tech Stack

- **Language:** Go (1.25+)
- **TUI/CLI Framework:** [charmbracelet/huh](https://github.com/charmbracelet/huh) (forms, prompts, interactive inputs)
- **Underlying libraries:** bubbletea (Elm-architecture TUI), lipgloss (styling), bubbles (components)
- **Module path:** `github.com/ivancerovina/forge`

## Project Structure

Single-module Go project. Entry point is `main.go`.

## Data Directory

`~/.forge/` — created automatically on startup if it doesn't exist. Existing files are never overwritten.
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

- `environment.commands` — shell commands run by `forge start`, `forge stop`, `forge destroy`
- `environment.alias` — maps container/service names to Traefik routing rules:
  - `alias: null` → `<project-code>.local` (index, no subdomain)
  - `alias: "backend"` → `backend.<project-code>.local`

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
- Prompts before overwriting an existing `.forgerc.json`

## Theme

Purple color palette. All styled output uses lipgloss with these colors:
- Primary: `#9D4EDD` (purple) — titles, emphasis
- Secondary: `#7B2CBF` (dim purple) — section headings
- Text: `#E0E0E0` (white) — commands, key terms
- Muted: `#6C6C6C` (dim) — descriptions, secondary text
- Error: `#FF6B6B` (red) — error messages

## Code Conventions

- Standard Go formatting (`gofmt` / `goimports`)
- Handle errors explicitly; use `log.Fatal` for unrecoverable startup errors
- Use `huh` for all interactive user input (confirms, selects, text inputs, forms)
- Use `lipgloss` for all styled/colored terminal output
- Keep the main package thin; extract logic into internal packages as the project grows
