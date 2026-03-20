---
name: add-command
description: Scaffold a new forge CLI command across all required files
disable-model-invocation: true
argument-hint: "[command-name]"
---

# Add Command — Forge CLI Command Scaffolder

Scaffold a new CLI command for the forge project. This skill ensures every touchpoint is handled consistently.

## Arguments

`$ARGUMENTS` is the name of the new command (e.g., `project status`, `tunnel list`). If empty, ask the user what command to add.

## Before You Start

Ask the user for:
1. **Command name and position** — top-level (`forge <cmd>`) or subcommand (`forge project <cmd>`, `forge tunnel <cmd>`, `forge project alias <cmd>`)
2. **Purpose** — one-sentence description
3. **Flags** — names, types, short aliases, defaults
4. **Interactive mode?** — should it support a `huh` form when run without flags?
5. **Does it modify state?** — writes `.forgerc.json`, runs Docker commands, writes files, etc.

## Scaffolding Checklist

Work through each step in order. Do NOT skip any step.

### Step 1: Business Logic (internal package)

Determine which `internal/` package owns this logic:
- `internal/config/` — project config, validation, registry
- `internal/docker/` — Docker Compose operations, service status
- `internal/system/` — system init, Docker network, Traefik, TLS certs
- `internal/bind/` — domain binding, /etc/hosts, Traefik dynamic config
- `internal/ui/` — lipgloss styles only (no logic)

Create or extend functions in the appropriate package. Rules:
- Functions return errors — never call `os.Exit()` or print to stdout/stderr
- Follow existing naming conventions in the package
- Add validation functions to `internal/config/validate.go` if new fields are introduced

### Step 2: CLI Wiring (main.go)

Add the command function following the established pattern in `main.go`:

```go
func myNewCmd() *cli.Command {
    return &cli.Command{
        Name:  "name",
        Usage: "Short description",
        Flags: []cli.Flag{ /* ... */ },
        Action: func(ctx context.Context, cmd *cli.Command) error {
            // ...
        },
    }
}
```

Key conventions:
- Command functions are named `<scope><Name>Cmd()` (e.g., `projectInfoCmd`, `tunnelInitCmd`)
- Register in the appropriate `Commands` slice in the main app setup
- Support both interactive (huh forms) and non-interactive (flags) paths when applicable
- Use `config.FindForgeRC(".")` for commands that operate on the current project
- Use `lipgloss` styles from `internal/ui/` for all formatted output
- `huh` is only used in interactive code paths, never when flags are provided

### Step 3: Alias Entry Fields (if applicable)

If adding a new field to `.forgerc.json` alias entries:

1. Add field to `AliasEntry` struct in `internal/config/config.go` with proper JSON tag and `omitempty`
2. Add field to `DomainBinding` struct in `internal/bind/bind.go` if it affects routing
3. Propagate in `ComputeBindings()` in `internal/bind/bind.go`
4. Handle in `writeTraefikConfig()` in `internal/bind/traefik.go`
5. Add to JSON schema in `internal/config/forgerc.schema.json`
6. Add validation in `internal/config/validate.go` if needed
7. Add CLI flag in `projectAliasAddCmd()` in `main.go`
8. Add interactive prompt in the `huh` form
9. Display in `projectAliasInfoCmd()` in `main.go`

### Step 4: Config Fields (if applicable)

If adding a new field to global config (`~/.forge/config.json`):

1. Add field to `ForgeConfig` struct in `internal/config/config.go`
2. Read/write via `config.ReadConfig()` / `config.WriteConfig()`

If adding a new field to `.forgerc.json` (non-alias):

1. Add to `ForgeProject`, `Environment`, or `Hooks` struct in `internal/config/config.go`
2. Add to JSON schema in `internal/config/forgerc.schema.json`

### Step 5: Build & Verify

Delegate to the **`go-check`** agent to run build, vet, and format checks:

```
Use the go-check agent to verify the build compiles, passes vet, and has correct formatting.
```

If `go-check` reports failures, fix them before proceeding.

Then verify manually:
- `forge --help` shows the new command
- `forge <new-command> --help` shows correct flags and usage

### Step 6: Struct Sync Check (if new fields were added)

If Steps 3 or 4 added new struct fields, delegate to the **`struct-sync`** agent:

```
Use the struct-sync agent to verify all touchpoints are covered for the new fields.
```

Fix any gaps it reports before proceeding to docs.

### Step 7: Update Documentation

Update ALL of these:
- `CLAUDE.md` — command list, any new config fields, any new `.forgerc.json` fields
- `.claude/skills/forge/SKILL.md` — command table, field reference tables, schema example

### Step 8: Regression Scan

Before finishing, delegate to the **`regression-scan`** agent:

```
Use the regression-scan agent to analyze the working diff for missed callers, parity gaps, and edge cases.
```

Fix any "MUST FIX" items it reports. "SHOULD FIX" items are at your discretion.

## Agent Dependencies

This skill delegates to these agents during execution:

| Agent | When | Purpose |
|-------|------|---------|
| `go-check` | Step 5 | Verify build, vet, formatting |
| `struct-sync` | Step 6 (if new fields) | Verify all struct touchpoints |
| `regression-scan` | Step 8 | Final impact analysis |

## Output Format

After scaffolding, provide a summary:
- Files created/modified
- Agent check results (go-check, struct-sync, regression-scan)
- How to test the new command
- Any manual follow-up needed (e.g., "run `forge project bind` to test routing")
