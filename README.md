# Hypervane

A modern Proxmox VE operations console. Hypervane is a Docker stack: a Go backend talks to one cluster over the official API, and a React UI streams live inventory, metrics, tasks, and consoles. The browser never holds a Proxmox ticket or token.

Version 3.2 grows Recipes and Scripts. Fleet, SSO, and MCP writes stay tabled; see [ROADMAP.md](ROADMAP.md).

## Stack

- `web` — React, Vite, Tailwind, TanStack Query/Router, uPlot, noVNC, SPICE-HTML5, xterm.js
- `api` — Go, Chi, pgx, Redis, WebSocket hub and console proxy
- `postgres` — local admin, PVE shadow users, sessions with sealed tickets, encrypted cluster token
- `redis` — live fan-out and short metric rings

## Quick start

```bash
cp .env.example .env
# set HYPERVANE_MASTER_KEY to a long random string
docker compose -f deploy/docker-compose.dev.yml up --build
```

UI: [http://localhost:5173](http://localhost:5173)  
API: [http://localhost:8080/healthz](http://localhost:8080/healthz)

Production pulls published images. GitHub Actions builds `linux/amd64` images on every push to `main` and pushes them to `ghcr.io/zypher-systems/hypervane-api` and `ghcr.io/zypher-systems/hypervane-web`. `latest` and `main` track that branch. A `vX.Y.Z` tag also publishes `X.Y.Z`.

```bash
git clone https://github.com/zypher-systems/hypervane.git && cd hypervane
cp .env.example .env
# set HYPERVANE_MASTER_KEY (openssl rand -base64 32), POSTGRES_PASSWORD and
# HYPERVANE_COOKIE_SECURE=true; HYPERVANE_HTTP_PORT picks the port (default 3000)
# HYPERVANE_VERSION selects the image tag (default latest)
docker compose -p hypervane --env-file .env -f deploy/docker-compose.yml up -d
```

To update, run the same `docker compose` command. The database volume is kept. Keep `-p hypervane` and `--env-file .env` every time: without the project name the stack is named `deploy` and starts with an empty database, and without the env file Compose looks for `deploy/.env`.

To build from this checkout instead, use `api/Dockerfile` and `web/Dockerfile`. `HYPERVANE_VERSION` is the version baked into those local images; without it the build says `dev`.

On first visit, create a Hypervane admin and add a Proxmox connection (user `root@pam!tokenid` plus API key). Self-signed TLS is the default (`skip-verify`); optionally pin the node certificate SHA-256.

## Local without Compose (API only)

Postgres and Redis must already be running, then:

```bash
cd api && go run ./cmd/api
cd web && npm install && npm run dev
```

## Layout

- [`api/`](api/) — Go module (`cmd/api`, `internal/pve`, `poller`, `hub`, `console`, `httpapi`)
- [`web/`](web/) — Vite app
- [`deploy/`](deploy/) — Compose files

## Open source

Hypervane is open source under the [MIT License](LICENSE). The audit log, long history, read-only MCP, every recipe, and remote migrate are included. There is no license key, and an install does not contact a licensing server.

## Recipes

**Recipes** runs common jobs as ordinary Proxmox API calls. Users signed in with a Proxmox account run them with their own permissions; local users run them with the connection's token. Each recipe checks its inputs against the cluster and shows the exact steps before anything changes. Recipes that stop, move or delete things ask first, and every run reports its progress step by step, with a link to each Proxmox task.

- **Templates**: Debian 12 and 13, Ubuntu 22.04, 24.04 and 26.04, Fedora 44, Alpine 3.24, openSUSE Leap 16.0, Arch Linux, AlmaLinux 9 and 10, Rocky Linux 9 and 10, CentOS Stream 10, and refreshing a template from the newest image. Images download to a storage with **Import** content (usually `local`) and are removed once the template is built. Templates stay as small as the image unless you set a disk size.
- **Guests**: deploy a guest from a template with its user, SSH keys, address and disk size (the disk grows before the guest starts), turn on the guest agent, turn a VM into a template, and deploy several guests at once.
- **Nodes**: check every node for updates, shut down or start a node's guests, and move all guests off a node.
- **Backups & snapshots**: back up unprotected guests now, snapshot a node's guests, schedule backups for unprotected guests, and clean up old snapshots.

The Fedora and Alpine templates look up the newest build on the distribution's mirror from the Hypervane host, a plain HTTPS request carrying nothing about your cluster. Without internet access they use the build Hypervane ships with.

## Security notes

- Proxmox credentials never leave the API process.
- The API token is AES-GCM encrypted at rest with `HYPERVANE_MASTER_KEY`.
- Destructive guest/storage actions require a confirmation in the UI.
