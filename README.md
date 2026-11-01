# Hypervane

A modern Proxmox VE operations console. Hypervane is a Docker stack: a Go backend talks to your clusters over the official API, and a React UI streams live inventory, metrics, tasks, and consoles. The browser never holds a Proxmox ticket or token.

Version 3.3 is the first public release, with a calmer redesign, insights on the datacenter overview, and every feature included. Fleet, SSO, and MCP writes stay tabled; see [ROADMAP.md](ROADMAP.md).

![The Hypervane datacenter overview, with insights, node health and failover capacity](assets/screenshots/overview.png)

## Screenshots

These show simulated clusters, so every name, address and number in them is invented. Select an image to see it at full size.

<table>
  <tr>
    <td width="50%" valign="top"><a href="assets/screenshots/guest.png"><img src="assets/screenshots/guest.png" alt="A guest's live status, details and history charts"></a></td>
    <td width="50%" valign="top"><a href="assets/screenshots/top.png"><img src="assets/screenshots/top.png" alt="The Top 10 page ranking the busiest guests by CPU"></a></td>
  </tr>
  <tr>
    <td><strong>Guests.</strong> Live status, details, and history for every guest.</td>
    <td><strong>Top 10.</strong> The guests putting the most pressure on the cluster.</td>
  </tr>
  <tr>
    <td><a href="assets/screenshots/recipes.png"><img src="assets/screenshots/recipes.png" alt="The Recipes page with a form for a Debian cloud template"></a></td>
    <td><a href="assets/screenshots/backups.png"><img src="assets/screenshots/backups.png" alt="Backup coverage listing each guest and its last backup"></a></td>
  </tr>
  <tr>
    <td><strong>Recipes.</strong> Common jobs that show their exact steps before they run.</td>
    <td><strong>Backups.</strong> Which guests are covered, and when each last ran.</td>
  </tr>
  <tr>
    <td><a href="assets/screenshots/storage.png"><img src="assets/screenshots/storage.png" alt="A storage page with usage history and its volumes"></a></td>
    <td><a href="assets/screenshots/tasks.png"><img src="assets/screenshots/tasks.png" alt="The task drawer streaming the log of a live migration"></a></td>
  </tr>
  <tr>
    <td><strong>Storage.</strong> Usage over time and everything stored there.</td>
    <td><strong>Tasks.</strong> Running work with its log streaming as it happens.</td>
  </tr>
</table>

## Stack

- `web` — React, Vite, Tailwind, TanStack Query/Router, uPlot, noVNC, SPICE-HTML5, xterm.js
- `api` — Go, Chi, pgx, Redis, WebSocket hub and console proxy
- `postgres` — local admin, PVE shadow users, sessions with sealed tickets, encrypted cluster token
- `redis` — live fan-out and short metric rings

## Deploy

Hypervane runs from published images, so you don't need a copy of this repository to deploy it. You need Docker with the Compose plugin on an `x86-64` Linux host. GitHub Actions builds `linux/amd64` images on every push to `main` and pushes them to `ghcr.io/zypher-systems/hypervane-api` and `ghcr.io/zypher-systems/hypervane-web`. `latest` and `main` track that branch. A `vX.Y.Z` tag also publishes `X.Y.Z`.

Make a folder for the stack:

```bash
mkdir hypervane && cd hypervane
```

Save this as `compose.yaml`. It is the same stack as [`deploy/docker-compose.yml`](deploy/docker-compose.yml).

```yaml
name: hypervane

services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: hypervane
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
      POSTGRES_DB: hypervane
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U hypervane -d hypervane"]
      interval: 5s
      timeout: 3s
      retries: 10

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    command: ["redis-server", "--save", "", "--appendonly", "no"]
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10

  api:
    image: ghcr.io/zypher-systems/hypervane-api:${HYPERVANE_VERSION:-latest}
    pull_policy: always
    restart: unless-stopped
    environment:
      DATABASE_URL: postgres://hypervane:${POSTGRES_PASSWORD}@postgres:5432/hypervane?sslmode=disable
      REDIS_URL: redis://redis:6379/0
      HYPERVANE_HTTP_ADDR: :8080
      HYPERVANE_COOKIE_SECURE: ${HYPERVANE_COOKIE_SECURE:-true}
      HYPERVANE_MASTER_KEY: ${HYPERVANE_MASTER_KEY:?set HYPERVANE_MASTER_KEY to a long random secret}
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1:8080/healthz >/dev/null"]
      interval: 10s
      timeout: 3s
      retries: 10
    expose:
      - "8080"

  web:
    image: ghcr.io/zypher-systems/hypervane-web:${HYPERVANE_VERSION:-latest}
    pull_policy: always
    restart: unless-stopped
    environment:
      HYPERVANE_TRUSTED_PROXIES: ${HYPERVANE_TRUSTED_PROXIES:-}
    ports:
      - "${HYPERVANE_HTTP_PORT:-3000}:80"
    depends_on:
      api:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1/ >/dev/null"]
      interval: 10s
      timeout: 3s
      retries: 10

volumes:
  pgdata:
```

Create a `.env` beside it with the two required secrets:

```bash
printf 'HYPERVANE_MASTER_KEY=%s\nPOSTGRES_PASSWORD=%s\n' \
  "$(openssl rand -base64 32)" "$(openssl rand -hex 24)" > .env
chmod 600 .env
```

You can also set these in `.env`:

| Variable | Default | What it does |
| --- | --- | --- |
| `HYPERVANE_HTTP_PORT` | `3000` | Host port for the UI. The API is served behind it under `/api`. |
| `HYPERVANE_VERSION` | `latest` | Image tag to run: `latest`, `main`, `X.Y.Z` or `sha-<commit>`. |
| `HYPERVANE_TRUSTED_PROXIES` | empty | Addresses or CIDR ranges of your reverse proxies, separated by commas. Hypervane reads the real client address from `X-Forwarded-For` only on requests from these, and the proxy must set that header. |
| `HYPERVANE_COOKIE_SECURE` | `true` | Marks session cookies secure, so sign-in only works over HTTPS. Put a TLS reverse proxy in front of the port, or set `false` to use plain HTTP on a trusted network. |

Failed sign-ins are limited per client address and per account. Behind a reverse proxy, every client looks like the proxy until you list it in `HYPERVANE_TRUSTED_PROXIES`, so until then they all share one limit. List only your proxies: anything on that list can report any client address.

Start it:

```bash
docker compose up -d
```

Open `http://<host>:3000` (or your proxy's HTTPS address). On first visit, create a Hypervane admin and add a Proxmox connection (user `root@pam!tokenid` plus API key). Self-signed TLS is the default (`skip-verify`); optionally pin the node certificate SHA-256.

To update, run `docker compose up -d` again in the same folder. It pulls the newest images for `HYPERVANE_VERSION` and keeps the database in the `hypervane_pgdata` volume. Keep `.env` safe and backed up: `HYPERVANE_MASTER_KEY` decrypts the stored Proxmox token, and a new key can't read the old one.

From a checkout of this repository, the same stack runs with `docker compose --env-file .env -f deploy/docker-compose.yml up -d` and a `.env` at the repository root (see [`.env.example`](.env.example)). To build images from source instead, use `api/Dockerfile` and `web/Dockerfile`. `HYPERVANE_VERSION` is the version baked into those local images; without it the build says `dev`.

## Development

```bash
cp .env.example .env
# set HYPERVANE_MASTER_KEY to a long random string
docker compose -f deploy/docker-compose.dev.yml up --build
```

UI: [http://localhost:5173](http://localhost:5173)  
API: [http://localhost:8080/healthz](http://localhost:8080/healthz)

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
