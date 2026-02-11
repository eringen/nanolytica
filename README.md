# Nanolytica

Privacy-first, self-hosted web analytics. No cookies, no tracking consent, single binary.

> Originally built as part of [eringen.com](https://eringen.com), now extracted and maintained as a standalone project.

## Quick Start

```bash
git clone https://github.com/eringen/nanolytica.git
cd nanolytica
make setup && make build
./nanolytica
```

Add to your website:

```html
<script src="http://your-server:8080/nanolytica.js"></script>
```

Dashboard: `http://localhost:8080/admin/analytics/`

## Features

- **Privacy-first** -- no cookies, no localStorage, honors DNT
- **Bot detection** -- 15+ crawlers identified and separated
- **Browser/OS/device breakdowns**, referrer tracking, time on page
- **Real-time** -- live visitor count (last 5 minutes)
- **Single binary** (~10MB) -- all assets embedded, SQLite with WAL mode, zero external dependencies
- **HTMX dashboard** -- server-rendered HTML fragments, minimal client JS
- **Type-safe templates** via [templ](https://templ.guide/)
- **Docker ready** -- multi-stage build, non-root runtime

## Installation

### From Source

**Standard build** (requires `static/` folder at runtime):
```bash
make templ    # generate templates
make build    # build binary

# cross-compile
make build-linux
make build-darwin
make build-windows
```

**Single binary build** (all assets embedded, no external files needed):
```bash
make singlebinary    # build self-contained binary with embedded CSS/JS

# cross-compile single binary
make singlebinary-linux
make singlebinary-darwin
make singlebinary-windows
make singlebinary-all   # all platforms
```

### Docker

```bash
docker build -t nanolytica .
docker run -p 8080:8080 \
  -e NANOLYTICA_USERNAME=admin \
  -e NANOLYTICA_PASSWORD=secret \
  -v $(pwd)/data:/app/data \
  nanolytica
```

### Docker Compose

```yaml
version: '3.8'
services:
  nanolytica:
    build: .
    ports:
      - "8080:8080"
    environment:
      - NANOLYTICA_USERNAME=${NANOLYTICA_USERNAME}
      - NANOLYTICA_PASSWORD=${NANOLYTICA_PASSWORD}
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `NANOLYTICA_DB_PATH` | `data/nanolytica.db` | SQLite database path |
| `NANOLYTICA_USERNAME` | *(none)* | Dashboard username |
| `NANOLYTICA_PASSWORD` | *(none)* | Dashboard password |

If no credentials are set, the dashboard is publicly accessible (warning logged on startup).

## API

### Public

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check (JSON) |
| `GET` | `/nanolytica.js` | Tracking script |
| `POST` | `/api/analytics/collect` | Collect page view (respects DNT, returns 204) |

### Admin (authenticated)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/admin/analytics/` | Dashboard |
| `GET` | `/admin/analytics/api/stats?period=week` | Visitor stats (JSON) |
| `GET` | `/admin/analytics/api/bot-stats?period=week` | Bot stats (JSON) |
| `GET` | `/admin/analytics/fragments/stats?period=week` | Visitor stats (HTML fragment) |
| `GET` | `/admin/analytics/fragments/bot-stats?period=week` | Bot stats (HTML fragment) |
| `GET` | `/admin/analytics/fragments/setup` | Setup instructions (HTML fragment) |

Period options: `today`, `week`, `month`, `year`.

### Collect Request Body

```json
{
  "path": "/blog/hello-world",
  "referrer": "https://google.com",
  "screen_size": "1920x1080",
  "user_agent": "Mozilla/5.0...",
  "duration_sec": 45
}
```

## Architecture

```
Client                          Server
  |                               |
  |-- GET /nanolytica.js -------->|  tracking script
  |                               |
  |-- POST /collect ------------->|  Echo HTTP server
  |                               |    -> parse UA, detect bots, hash IP
  |                               |    -> store in SQLite (WAL mode)
  |                               |
  |-- GET /admin/analytics/ ----->|  HTMX dashboard
  |                               |    -> server-rendered HTML fragments
```

### Request Flow

1. Client loads `/nanolytica.js`
2. Script sends POST to `/api/analytics/collect`
3. Server parses User-Agent, detects bots, hashes IP (salted SHA-256)
4. Visit stored in SQLite
5. Dashboard renders stats via HTMX HTML fragments

## Database

Two main tables (`visits` for humans, `bot_visits` for crawlers) plus a `settings` table for configuration. Schema auto-created on first run with version-tracked migrations.

SQLite runs in WAL mode. Data older than 365 days is cleaned up daily.

### sqlc

All database queries are defined in `analytics/sqlcgen/queries.sql` using [sqlc](https://sqlc.dev/) annotations. Running `make sqlc` generates type-safe Go code from these queries -- no hand-written SQL strings or manual `rows.Scan()` calls. The generated files are committed to the repo so builds don't require sqlc as a dependency.

To modify a query, edit `queries.sql` (or `schema.sql` for DDL changes), then run:

```bash
make sqlc       # regenerate Go code
make test       # verify
```

`store.go` is a thin wrapper that delegates to the generated `sqlcgen.Queries` methods and converts between internal types and sqlc-generated types.

```sql
CREATE INDEX idx_visits_timestamp ON visits(timestamp);
CREATE INDEX idx_visits_visitor_id ON visits(visitor_id);
CREATE INDEX idx_visits_path ON visits(path);
CREATE INDEX idx_visits_browser ON visits(browser);
CREATE INDEX idx_visits_os ON visits(os);
CREATE INDEX idx_visits_device ON visits(device);
CREATE INDEX idx_bot_visits_timestamp ON bot_visits(timestamp);
CREATE INDEX idx_bot_visits_name ON bot_visits(bot_name);
```

## Development

### Prerequisites

- Go 1.24+
- Node.js 18+ (TypeScript + Tailwind CSS)
- templ CLI: `go install github.com/a-h/templ/cmd/templ@latest`
- sqlc (only if modifying queries): `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`

### Commands

| Command | Description |
|---------|-------------|
| `make build` | Build binary (includes templ + assets) |
| `make run` | Build and run |
| `make dev` | Run with live reload (requires [air](https://github.com/cosmtrek/air)) |
| `make test` | Run tests |
| `make templ` | Generate Go code from .templ files |
| `make assets` | Build TypeScript + Tailwind CSS |
| `make sqlc` | Regenerate type-safe query code from `analytics/sqlcgen/queries.sql` |
| `make clean` | Remove build artifacts |
| `make clean-data` | Delete database (destructive) |
| `make prod` | Full production build |
| `make singlebinary` | Build self-contained binary with embedded assets |
| `make singlebinary-all` | Cross-compile single binary for all platforms |
| `make check` | TypeScript check + go vet |

### Project Structure

```
nanolytica/
+-- main.go                     # server setup, middleware, routing
+-- analytics/
|   +-- analytics.go            # types, UA parsing, bot detection, hashing
|   +-- store.go                # SQLite operations (thin wrapper around sqlcgen)
|   +-- handlers.go             # HTTP handlers
|   +-- analytics_test.go       # core function tests
|   +-- handlers_test.go        # validation tests
|   +-- sqlcgen/                # sqlc-generated type-safe query code
|       +-- sqlc.yaml           # sqlc config
|       +-- schema.sql          # DDL for sqlc type inference
|       +-- queries.sql         # annotated SQL queries
|       +-- *.go                # generated Go code (committed)
|   +-- templates/
|       +-- layout.templ        # base layout, tab/period selectors
|       +-- dashboard.templ     # dashboard page
|       +-- fragments.templ     # HTMX HTML fragments
|       +-- types.go            # view model types
+-- fe_src/
|   +-- analytics.ts            # client tracking script
|   +-- dashboard.ts            # dashboard JS
|   +-- css/input.css           # Tailwind input
+-- static/                     # built assets (gitignored)
+-- data/                       # SQLite database (gitignored)
+-- embed.go                    # embedded static files (embed build tag)
+-- embed_default.go            # filesystem static files (default build tag)
```

Files ending in `_templ.go` are auto-generated -- never edit them directly.

## Deployment

### Systemd

```ini
[Unit]
Description=Nanolytica Analytics
After=network.target

[Service]
Type=simple
User=nanolytica
WorkingDirectory=/opt/nanolytica
ExecStart=/opt/nanolytica/nanolytica
Environment="PORT=8080"
Environment="NANOLYTICA_DB_PATH=/var/lib/nanolytica/nanolytica.db"
Environment="NANOLYTICA_USERNAME=admin"
Environment="NANOLYTICA_PASSWORD=your-secure-password"
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

### Reverse Proxy

**Nginx:**
```nginx
server {
    listen 443 ssl http2;
    server_name analytics.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**Caddy:**
```
analytics.example.com {
    reverse_proxy localhost:8080
}
```

Rate limiting is not built-in. Use your reverse proxy:

```nginx
limit_req_zone $binary_remote_addr zone=analytics:10m rate=10r/s;
location /api/analytics/collect {
    limit_req zone=analytics burst=20 nodelay;
    proxy_pass http://localhost:8080;
}
```

## Security

- Salted SHA-256 IP hashing (per-installation random salt)
- Constant-time password comparison
- CORS scoped to public endpoints only
- Security headers: XSS protection, Content-Type nosniff, X-Frame-Options DENY
- Type-safe SQL via [sqlc](https://sqlc.dev/) -- no hand-written query strings
- Request body size limit (10KB)
- Docker runs as non-root user
- Graceful shutdown on SIGINT/SIGTERM

## Privacy

No PII stored. IP addresses are salted+hashed (irreversible). No cookies, no localStorage, no cross-site tracking. Honors `DNT: 1`. Query parameters stripped from URLs.

**Data collected:** page path, referrer domain, screen size, user-agent (for browser/OS/device detection), time on page.

GDPR/CCPA compliant by design.

## License

MIT. See [LICENSE](LICENSE).
