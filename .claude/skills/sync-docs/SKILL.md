---
name: sync-docs
description: Synchronize CLAUDE.md, SKILL.md, and forgerc.schema.json with the current codebase
disable-model-invocation: true
argument-hint: "[claude.md|skills|schema|all]"
---

# Sync Docs — Documentation Synchronization

Diff the current codebase against all documentation surfaces and patch them to stay in sync.

## Arguments

- Empty or `all`: full sync of everything below
- `claude.md`: update only `CLAUDE.md`
- `skills`: update only `.claude/skills/forge/SKILL.md`
- `schema`: update only `internal/config/forgerc.schema.json`

## Agent Dependencies

This skill delegates to the **`struct-sync`** agent as a detection step before patching:

| Agent | When | Purpose |
|-------|------|---------|
| `struct-sync` | Before Step 2 | Detect field-level drift across all struct touchpoints |

## Documentation Surfaces

There are three files that document the same information and must stay in sync:

| File | Purpose |
|------|---------|
| `CLAUDE.md` | Primary project docs — commands, flags, config fields, conventions |
| `.claude/skills/forge/SKILL.md` | Agent-facing reference — command table, field reference, schema example, validation rules |
| `internal/config/forgerc.schema.json` | Machine-readable JSON Schema for `.forgerc.json` — IDE autocompletion |

## Sync Procedure

### Step 1: Extract Source of Truth from Code

Read these files to determine the current state:

**Structs (canonical field definitions):**
- `internal/config/config.go` — `AliasEntry`, `ForgeProject`, `Environment`, `Hooks`, `ForgeConfig`

**Validation rules:**
- `internal/config/validate.go` — regexes, range checks, all `Validate*` functions

**CLI commands and flags:**
- `main.go` — all `*Cmd()` functions, their `Name`, `Usage`, `Aliases`, `Flags`

**Traefik behavior:**
- `internal/bind/traefik.go` — middleware logic, router rules
- `internal/bind/bind.go` — `DomainBinding` struct, `ComputeBindings` logic

### Step 1.5: Struct Sync Detection

Delegate to the **`struct-sync`** agent to get a precise per-field drift report:

```
Use the struct-sync agent to check all AliasEntry fields across their 12 touchpoints.
```

The agent will report which fields are missing from which locations. Use this output to guide the patches in Steps 2-3 — it tells you exactly what's out of sync without you having to manually diff every file.

### Step 2: Diff Against Documentation

For each documentation surface, check:

#### CLAUDE.md
- [ ] **Project Structure** section matches actual `internal/` packages
- [ ] **Data Directory** section lists all files in `~/.forge/`
- [ ] **Project File** section — `.forgerc.json` example includes all current `AliasEntry` fields with correct types and descriptions
- [ ] **Command sections** — every command in `main.go` is documented with correct flags
- [ ] **Alias entry fields** — `environment.alias` documentation matches `AliasEntry` struct
- [ ] **Code Conventions** section is accurate

#### .claude/skills/forge/SKILL.md
- [ ] **`.forgerc.json` Full Schema** example includes all fields
- [ ] **Alias Entry Fields** table matches `AliasEntry` struct (field name, type, default, description)
- [ ] **Domain Routing Logic** table is accurate (check against `ComputeBindings` and `writeTraefikConfig`)
- [ ] **All Commands** tables list every command with correct descriptions
- [ ] **Validation Rules Summary** matches `validate.go`
- [ ] **Alias management** command table includes all flags

#### internal/config/forgerc.schema.json
- [ ] Every field in `AliasEntry` has a corresponding schema property
- [ ] Every field in `ForgeProject`, `Environment`, `Hooks` has a schema property
- [ ] Types, patterns, defaults, and descriptions match the Go code
- [ ] `required` arrays are correct
- [ ] `additionalProperties: false` is set where appropriate

### Step 3: Apply Patches

For each discrepancy found, edit the documentation file to match the code. Rules:
- The **code is the source of truth** — never change code to match docs
- Preserve existing formatting style in each file
- Keep CLAUDE.md examples concise but complete
- Keep SKILL.md tables aligned
- Keep schema descriptions under one sentence

### Step 4: Report

Print a summary of what was changed:

```
File                                    Changes
----                                    -------
CLAUDE.md                               Added forward_pathname to alias fields, updated alias add flags
.claude/skills/forge/SKILL.md           Added forward_pathname to field table and schema example
internal/config/forgerc.schema.json     (already in sync)
```

If everything is already in sync, say so.

## Common Drift Patterns

These are the most frequent sources of drift — check them first:

1. **New alias fields** added to `AliasEntry` but missing from docs/schema
2. **New CLI flags** added to a command but missing from CLAUDE.md command docs
3. **New commands** added to `main.go` but missing from command tables
4. **Changed defaults** (e.g., HTTPS default) not reflected in field tables
5. **New internal packages** not listed in project structure
6. **Validation rule changes** not reflected in validation summary tables
