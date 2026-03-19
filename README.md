# PanelX

PanelX is an open-source VPS control panel built for practical operations: provision WordPress quickly, monitor server health, and manage multiple sites from a single interface.

It is designed for operators who want a simple lightweight control surface on a Linux VPS without heavy platform overhead.

---

## Project History

Work on PanelX began more than five years ago.  
Much of this codebase reflects that original period of development.

In 2023, we lost [@rahuldraz](https://github.com/rahuldraz).  
This repository is being published now with essential fixes and stabilization work, in his memory.

If time allows, the long-term goal remains the same as when we started: to make PanelX the panel we originally set out to build.

---

## Table of Contents

- [Key Features](#key-features)
- [Architecture](#architecture)
- [Supported Environments](#supported-environments)
- [System Requirements](#system-requirements)
- [Quick Install](#quick-install)
- [What the Installer Configures](#what-the-installer-configures)
- [Accessing the Panel](#accessing-the-panel)
- [API Overview](#api-overview)
- [Operational Commands](#operational-commands)
- [Development](#development)
- [Repository Layout](#repository-layout)
- [Production and Security Guidance](#production-and-security-guidance)
- [Contributing](#contributing)
- [License](#license)

---

## Key Features

PanelX currently includes:

- One-command VPS installation
- Browser UI at `/panel`
- Control-plane API under `/v1/*`
- One-click WordPress provisioning:
  - domain + install path support
  - MariaDB database/user provisioning
  - WordPress download/extract
  - `wp-config.php` generation
  - Nginx vhost generation/reload
  - WP-CLI bootstrap
- Installations registry and listing APIs
- File manager operations scoped to managed site roots
- Live system telemetry:
  - CPU, memory, swap, load, uptime
  - disk usage
  - service states (`nginx`, `mariadb`, PHP-FPM, PanelX services)
  - network counters
  - WordPress runtime checks

---

## Architecture

PanelX is a monorepo with two primary runtime components:

- **Control Plane (Go):** API server, provisioning workflows, installations/domain/file services, and health/status data.
- **Web Console (React + Vite + TypeScript):** operator UI for setup, monitoring, and day-to-day management.

---

## Supported Environments

### Fully Supported

- Ubuntu 24.04 LTS (`amd64`)
- Debian 12 (`amd64`)

### Best-Effort / Legacy-Compatible

- Ubuntu 22.04 LTS (`amd64`)
- Ubuntu 20.04 LTS (`amd64`)

Behavior in legacy environments can vary with package mirror and PHP packaging state.

---

## System Requirements

### Minimum (single low-traffic site)

- 2 vCPU
- 4 GB RAM
- 40 GB SSD
- Public IPv4
- Open ports: 80, 443 (and access to panel route)

### Recommended Baseline (multi-site / operational headroom)

- 4 vCPU
- 8 GB RAM
- 80 GB SSD
- 2–4 GB swap
- Keep ~30% disk free for updates, logs, and backups

---

## Quick Install

Run as `root` on a fresh server:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/Harsh223/PanelX/main/deploy/install-panelx.sh)"
```

Optional explicit repo/branch override:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/Harsh223/PanelX/main/deploy/install-panelx.sh)" -- \
  --repo https://github.com/Harsh223/PanelX.git \
  --branch main
```

---

## What the Installer Configures

Installer script: [`deploy/install-panelx.sh`](deploy/install-panelx.sh)

It provisions:

- Nginx, MariaDB, PHP-FPM, WP-CLI
- Build/runtime dependencies for PanelX binaries and web assets
- Systemd services:
  - `panelx-control-plane.service`
  - `panelx-node-agent.service`
- Runtime config under `/etc/panelx/`
- Randomized admin token and provisioning DB credentials

At completion, it prints panel access details and bootstrap token information.

---

## Accessing the Panel

After installation:

1. Open: `http://<server-ip>/panel`
2. Complete first-time admin setup using installer token
3. Use the UI sections for:
   - **Home / Launch Assistant** (guided flow)
   - **Sites** (installations and credentials)
   - **Files** (file manager)
   - **Domains / SSL / Logs** (advanced controls)
   - **System** (infrastructure health)

---

## API Overview

Panel APIs are exposed under `/v1/*`.

Typical auth mechanisms:

- Admin session cookie (after panel login), or
- `X-PanelX-Token: <token>`, or
- `Authorization: Bearer <token>` (where supported)

Common endpoint groups:

- Panel auth/session:
  - `GET /v1/panel/auth/status`
  - `POST /v1/panel/auth/setup`
  - `POST /v1/panel/auth/login`
  - `POST /v1/panel/auth/logout`
  - `GET /v1/panel/me`
- WordPress/installations:
  - `POST /v1/wordpress/install`
  - `GET /v1/installations`
- Domains:
  - `GET /v1/domains`
  - `POST /v1/domains/create`
  - `POST /v1/domains/health`
  - `POST /v1/domains/redirects`
  - `POST /v1/domains/ssl/issue`
  - `POST /v1/domains/ssl/renew`
  - `POST /v1/domains/ssl/revoke`
  - `GET /v1/domains/logs`
- File manager:
  - `GET /v1/files/list`
  - `GET /v1/files/read`
  - `POST /v1/files/write`
  - `POST /v1/files/delete`
- Health:
  - `GET /health` (control plane)
  - `GET http://127.0.0.1:8090/health` (node-agent local endpoint)

---

## Operational Commands

Service status:

```bash
systemctl status panelx-control-plane --no-pager
systemctl status panelx-node-agent --no-pager
systemctl status nginx mariadb --no-pager
```

Logs:

```bash
journalctl -u panelx-control-plane -n 200 --no-pager
journalctl -u panelx-node-agent -n 200 --no-pager
journalctl -u nginx -n 100 --no-pager
journalctl -u mariadb -n 100 --no-pager
```

Local health checks:

```bash
curl -fsS http://127.0.0.1:8080/health
curl -fsS http://127.0.0.1:8090/health
```

---

## Development

### Control Plane (Go)

From repository root:

```bash
go -C apps/control-plane test ./...
go -C apps/control-plane build ./...
```

### Web Console (Node + Vite)

From repository root:

```bash
npm --prefix apps/web-console install
npm --prefix apps/web-console run build
```

---

## Repository Layout

- [`apps/control-plane`](apps/control-plane) — Go control plane and API
- [`apps/web-console`](apps/web-console) — React operator console
- [`deploy`](deploy) — installer and deployment scripts
- [`docs`](docs) — documentation
- [`plans`](plans) — planning artifacts
- [`tests`](tests) — integration and VPS validation scripts

---

## Production and Security Guidance

PanelX is actively evolving. For production use, apply standard hardening:

- Enforce TLS and secure redirect policy
- Restrict firewall exposure to required ports only
- Rotate tokens/secrets and avoid credential reuse
- Enable reliable backups and test restores
- Monitor services and set alerting
- Patch OS + runtime dependencies regularly
- Use SSH keys and disable password auth where possible

---

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for development standards and contribution workflow.

---

## License

See repository license metadata for current canonical license terms.
