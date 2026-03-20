---
name: test-forge
description: Build the forge binary and run structured smoke tests against it
disable-model-invocation: true
argument-hint: "[command-to-test]"
---

# Test Forge — Build & Smoke Test Runner

Build the forge binary and run structured smoke tests. Since forge has no automated test suite, this skill acts as a manual verification harness.

## Arguments

- Empty: run the full test suite below
- Specific command path: test only that area (e.g., `project init`, `tunnel info`, `alias add`)

## Critical Rules

- **NEVER delete the `forge` binary** — it is symlinked to the user's global PATH. Rebuild with `go build -o forge`.
- Use a temporary directory for test projects. Clean up after.
- Do NOT run `forge project bind` or `forge project unbind` — they require sudo for /etc/hosts.
- Do NOT run `forge setup` — it starts Docker containers.
- Do NOT run `forge project start/stop/destroy` unless Docker is confirmed running and the user approves.

## Agent Dependencies

This skill delegates to these agents during execution:

| Agent | When | Purpose |
|-------|------|---------|
| `go-check` | Phase 1 | Build, vet, and format verification |
| `regression-scan` | Phase 2 | Pre-test impact analysis of working changes |

## Test Suite

### Phase 1: Build & Lint

Delegate to the **`go-check`** agent:

```
Use the go-check agent to verify the build compiles, passes vet, and has correct formatting.
```

If `go-check` reports build failures, stop and report them. Do not proceed to smoke tests with a broken binary.

### Phase 2: Regression Pre-scan

Delegate to the **`regression-scan`** agent:

```
Use the regression-scan agent to analyze the working diff for missed callers and parity gaps.
```

Note any "MUST FIX" items — these are likely to cause smoke test failures. Report them but continue to Phase 3.

### Phase 3: Go Unit Tests

```bash
go test ./...
```

Report any failures. Continue to Phase 4 regardless (there may be no tests yet).

### Phase 4: CLI Smoke Tests

Run these in order. Each test should verify exit code and output.

#### 4.1 Help & Version

```bash
forge --help
forge --version
forge project --help
forge tunnel --help
forge project alias --help
```

Verify: each prints help/version text, exits 0.

#### 4.2 Project Init (non-interactive)

```bash
TMPDIR=$(mktemp -d)
cd "$TMPDIR"
forge project init -t "Test Project" -c "test-smoke" --no-register
```

Verify:
- `.forgerc.json` exists and is valid JSON
- Contains `"name": "Test Project"`, `"code": "test-smoke"`
- Contains `$schema` field
- `environment.alias` is an empty array `[]`

#### 4.3 Project Init Overwrite Protection

```bash
# Should fail without --force
forge project init -t "Test 2" -c "test-2" --no-register 2>&1
echo "exit: $?"
```

Verify: exits non-zero or prompts about existing file.

```bash
forge project init -t "Test 2" -c "test-2" --no-register --force
```

Verify: succeeds, `.forgerc.json` now has `"code": "test-2"`.

#### 4.4 Project Info (no Docker)

```bash
forge project info 2>&1
echo "exit: $?"
```

Verify: shows project header with name/code. May show errors about Docker — that's fine if Docker isn't running.

#### 4.5 Alias Add (non-interactive)

```bash
forge project alias add myapp-web --port 3000 --force
```

Verify: `environment.alias` array contains entry with `"service": "myapp-web"`, `"port": 3000`.

```bash
forge project alias add myapp-api --port 8080 --alias api --path /api --force
```

Verify: alias entry has `"service": "myapp-api"`, `"alias": "api"`, `"path": "/api"`, `"port": 8080`.

#### 4.6 Alias Add with Forward Pathname

```bash
forge project alias add myapp-ws --port 9000 --alias ws --path /socket --forward-pathname --force
```

Verify: alias entry has `"forward_pathname": true`.

#### 4.6b Alias Add with Target Path

```bash
forge project alias add myapp-backend --port 5542 --target-path /test --force
```

Verify: alias entry has `"service": "myapp-backend"`, `"target_path": "/test"`, `"port": 5542`.

#### 4.7 Alias Info

```bash
forge project alias info myapp-web
forge project alias info myapp-backend
forge project alias info
```

Verify: prints alias details. Single alias shows Port/Domain/HTTPS. `myapp-backend` shows `Target path: /test`. All aliases shows table.

#### 4.8 Alias Remove (non-interactive)

```bash
forge project alias remove myapp-ws
```

Verify: `myapp-ws` is no longer in `.forgerc.json`.

#### 4.9 Project List

```bash
forge project list
```

Verify: runs without panic. Output depends on whether projects are registered.

#### 4.10 Tunnel Info

```bash
forge tunnel info 2>&1
```

Verify: shows tunnel configuration (domain, enabled status). May show container errors if Docker isn't running — that's acceptable.

### Phase 5: Validation

Read the final `.forgerc.json` and verify:
- Valid JSON
- Schema field present
- All expected alias entries present/absent based on add/remove operations

### Phase 6: Cleanup

```bash
rm -rf "$TMPDIR"
```

## Reporting

After all tests, print a summary table:

```
Phase                         Status
---                           ---
go-check agent                PASS/FAIL (build, vet, format)
regression-scan agent         PASS/WARN (must-fix count, should-fix count)
Go unit tests                 PASS/FAIL/SKIP
Help & version                PASS/FAIL
Project init                  PASS/FAIL
Overwrite protection          PASS/FAIL
Project info                  PASS/FAIL
Alias add                     PASS/FAIL
Alias forward-pathname        PASS/FAIL
Alias info                    PASS/FAIL
Alias remove                  PASS/FAIL
Project list                  PASS/FAIL
Tunnel info                   PASS/FAIL
Validation                    PASS/FAIL
Cleanup                       PASS/FAIL
```

If any test fails, include the error output and suggest a fix.
