# Contributing to PanelX

Thanks for contributing to PanelX.

This project is currently in an MVP stage focused on practical VPS deployment and operator workflows.

## Current project scope

Primary components:
- `apps/control-plane` (Go)
- `apps/node-agent` (Go)
- `apps/web-console` (React + TypeScript)
- `deploy/install-panelx.sh` (VPS bootstrap)

## Contribution priorities

1. Reliability and installation stability
2. Security hardening
3. API contract consistency
4. Web panel usability
5. Test coverage and CI quality

## Engineering principles

1. **Contract-first APIs**
   - Define/adjust request and response contracts before handler changes.
2. **Safe-by-default behavior**
   - Validate input aggressively.
   - Avoid shell command injection and path traversal issues.
3. **Operational clarity**
   - Errors should be actionable and visible in logs.
4. **Incremental PRs**
   - Keep each pull request focused and reviewable.

## Branch and PR workflow

1. Branch from `main`.
2. Use clear branch names, for example: `feat/wordpress-backup-job` or `fix/install-script-permissions`.
3. Open PR with:
   - Problem statement
   - Proposed change
   - Risk notes
   - Manual test steps
4. Link related architecture docs/ADRs when relevant.

## Required checks before merge

- CI must pass in the configured GitHub Actions workflow(s).
- Go services should pass:
  - `go fmt ./...`
  - `go vet ./...`
  - `go test ./...`
- Web console should pass:
  - `npm install`
  - `npm run build`

## Commit message guidance

Use concise, imperative messages:
- `control-plane: add wordpress backup endpoint`
- `web-console: improve file editor save flow`
- `installer: validate php-fpm socket path`

## Security reporting

If you discover a security issue, do not publish exploit details in public issues.
Use responsible disclosure through the maintainer contact channel.
