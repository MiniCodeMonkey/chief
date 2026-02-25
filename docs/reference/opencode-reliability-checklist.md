---
description: Release validation checklist for OpenCode provider reliability in Chief.
---

# OpenCode Reliability Checklist

Use this checklist before releasing changes that affect OpenCode support.

## 1. Install Check

- [ ] Confirm OpenCode CLI is installed and discoverable:
  ```bash
  opencode --version
  ```
- [ ] If your environment requires a custom binary path, confirm `.chief/config.yaml` includes:
  ```yaml
  agent:
    provider: opencode
    opencode:
      cliPath: /absolute/path/to/opencode
  ```

## 2. Config Check

- [ ] Validate provider resolution and OpenCode config behavior:
  ```bash
  go test ./internal/agent -run 'TestResolve_.*OpenCode|TestResolve_env|TestResolve_priority' -v
  ```
- [ ] If `agent.opencode.requiredEnv` is used, verify missing/invalid env vars fail fast with actionable errors.

## 3. Run Check

- [ ] Validate successful OpenCode workflow execution path:
  ```bash
  go test ./internal/agent -run TestOpenCodeProvider_RunIntegration_Success -v
  ```
- [ ] Validate full OpenCode unit/integration coverage:
  ```bash
  go test ./internal/agent -run OpenCode -v
  ```

## 4. Failure Check

- [ ] Validate explicit failure-state mapping (missing binary, timeout, non-zero exit):
  ```bash
  go test ./internal/agent -run 'TestOpenCodeProvider_RunIntegration_(Failure|MissingBinary|Timeout|Canceled)' -v
  go test ./internal/loop -run ExecutionError -v
  ```
- [ ] Confirm failure output includes:
  - Explicit error kind (`missing_binary`, `timeout`, `non_zero_exit`, or `process_failure`)
  - Labeled stderr context when available
  - Remediation guidance for next steps

## 5. Regression Gate

- [ ] Run the project-wide quality checks and confirm no provider regressions:
  ```bash
  go test ./...
  go vet ./...
  ```
- [ ] If your CI includes linting, run the same linter locally (for example, `golangci-lint run ./...`).
