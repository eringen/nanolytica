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
- **Active engagement time** -- tracks actual time on page (pauses when tab is hidden or unfocused)
- **Scroll depth** -- tracks how far visitors scroll down each page
- **Bot detection** -- 55+ patterns covering search engines, AI crawlers, HTTP clients, headless browsers, monitoring tools
- **Browser/OS/device breakdowns**, referrer tracking
- **Real-time** -- live visitor count (last 5 minutes)
- **Single binary** (~10MB) -- all assets embedded, SQLite with WAL mode, zero external dependencies
- **talkDOM dashboard** -- server-rendered HTML fragments via [talkDOM](https://github.com/eringen/talkdom), minimal client JS
- **Login page** -- session-based authentication with HMAC-signed cookies (no browser Basic Auth popup)
- **Type-safe templates** via [templ](https://templ.guide/)
- **Rate limiting** -- per-IP rate limiting on collect endpoint (5 req/s, burst 10) and login (5 attempts per 5 minutes)
- **Strict CSP** -- `script-src 'self'`, no inline scripts, no `unsafe-inline`
- **Gzip compression** -- all responses compressed automatically
- **Data export** -- CSV export for visitor and bot stats
- **Docker ready** -- multi-stage build, non-root runtime
- **Client-side filtering** -- skips localhost, automated browsers (Selenium, Puppeteer, Cypress, PhantomJS)

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
| `COOKIE_SECURE` | `false` | Set to `true` for HTTPS (enables Secure flag on session cookie) |
| `NANOLYTICA_CORS_ORIGINS` | `*` | Allowed CORS origins (comma-separated, or `*` for all) |
| `NANOLYTICA_DB_MAX_OPEN_CONNS` | `10` | Max open database connections |
| `NANOLYTICA_DB_MAX_IDLE_CONNS` | `5` | Max idle database connections |

If no credentials are set, a random password is generated and logged on startup.

## API

### Public

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check (JSON) |
| `GET` | `/nanolytica.js` | Tracking script |
| `POST` | `/api/analytics/collect` | Collect page view (respects DNT, rate-limited, returns 204) |
| `GET` | `/admin/login` | Login page |
| `POST` | `/admin/login` | Authenticate |
| `POST` | `/admin/logout` | Sign out (clears session) |

### Admin (authenticated)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/admin/analytics/` | Dashboard |
| `GET` | `/admin/analytics/api/stats?period=week` | Visitor stats (JSON) |
| `GET` | `/admin/analytics/api/bot-stats?period=week` | Bot stats (JSON) |
| `GET` | `/admin/analytics/api/export/stats?period=week` | Visitor stats (CSV download) |
| `GET` | `/admin/analytics/api/export/bot-stats?period=week` | Bot stats (CSV download) |
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
  "duration_sec": 45,
  "scroll_depth": 82
}
```

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | Page path (max 2048 chars) |
| `referrer` | string | Referrer URL (max 2048 chars) |
| `screen_size` | string | Viewport size, e.g. `1920x1080` |
| `user_agent` | string | Browser User-Agent (max 512 chars) |
| `duration_sec` | int | Active engagement time in seconds (0-86400) |
| `scroll_depth` | int | Max scroll depth percentage (0-100) |

## Architecture

```
Client                          Server
  |                               |
  |-- GET /nanolytica.js -------->|  tracking script
  |                               |
  |-- POST /collect ------------->|  Echo HTTP server
  |                               |    -> parse UA, detect bots, hash IP
  |                               |    -> rate limit (5 req/s per IP)
  |                               |    -> store in SQLite (WAL mode)
  |                               |
  |-- GET /admin/login ---------->|  login page (session-based auth)
  |-- POST /admin/login --------->|  authenticate, set session cookie
  |                               |
  |-- GET /admin/analytics/ ----->|  talkDOM dashboard
  |                               |    -> server-rendered HTML fragments
```

### Tracking Script

The client tracking script (`nanolytica.js`) collects:

- **Active engagement time** -- only counts time when the tab is visible AND focused. Pauses on blur/hidden, resumes on focus/visible.
- **Scroll depth** -- tracks the maximum scroll position as a percentage of total page height.
- **Localhost exclusion** -- skips tracking on `localhost`, `127.x.x.x`, `[::1]`, and `file:` protocol.
- **Automation detection** -- skips tracking for headless browsers (`navigator.webdriver`, PhantomJS, Cypress, Nightmare).

### Request Flow

1. Client loads `/nanolytica.js`
2. Script sends POST to `/api/analytics/collect` on page load (duration=0, initial scroll depth)
3. Script tracks active engagement time and scroll depth while the user is on the page
4. On page unload, script sends final POST with actual engaged time and max scroll depth
5. Server parses User-Agent, detects bots, hashes IP (salted SHA-256)
6. Visit stored in SQLite via async batch insert queue
7. Dashboard renders stats via talkDOM HTML fragment updates

## Database

Two main tables (`visits` for humans, `bot_visits` for crawlers) plus a `settings` table for configuration. Schema auto-created on first run with version-tracked migrations.

SQLite runs in WAL mode. Data older than 365 days is cleaned up daily.

### Schema

The `visits` table includes:

| Column | Type | Description |
|--------|------|-------------|
| `visitor_id` | TEXT | Anonymous fingerprint hash |
| `session_id` | TEXT | Day-scoped session identifier |
| `ip_hash` | TEXT | Salted SHA-256 hash of IP |
| `browser` | TEXT | Browser name |
| `os` | TEXT | Operating system |
| `device` | TEXT | desktop, mobile, tablet |
| `path` | TEXT | Page path |
| `referrer` | TEXT | Referrer domain |
| `screen_size` | TEXT | Viewport dimensions |
| `timestamp` | DATETIME | Visit time (UTC) |
| `duration_sec` | INTEGER | Active engagement time |
| `scroll_depth` | INTEGER | Max scroll depth (0-100) |

### sqlc

All database queries are defined in `analytics/sqlcgen/queries.sql` using [sqlc](https://sqlc.dev/) annotations. Running `make sqlc` generates type-safe Go code from these queries -- no hand-written SQL strings or manual `rows.Scan()` calls. The generated files are committed to the repo so builds don't require sqlc as a dependency.

To modify a query, edit `queries.sql` (or `schema.sql` for DDL changes), then run:

```bash
make sqlc       # regenerate Go code
make test       # verify
```

`store.go` is a thin wrapper that delegates to the generated `sqlcgen.Queries` methods and converts between internal types and sqlc-generated types.

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
+-- main.go                     # server setup, middleware, routing, session auth
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
|       +-- login.templ         # login page
|       +-- fragments.templ     # talkDOM HTML fragments
|       +-- types.go            # view model types
+-- fe_src/
|   +-- analytics.ts            # client tracking script
|   +-- dashboard.ts            # dashboard JS (talkDOM integration)
|   +-- css/input.css           # Tailwind input
+-- static/                     # built assets (gitignored)
|   +-- js/talkdom.js           # talkDOM library
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
Environment="COOKIE_SECURE=true"
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

## Security

- Session-based authentication with HMAC-signed cookies (30-day expiry)
- Salted SHA-256 IP hashing (per-installation random salt)
- Constant-time password comparison
- Per-IP rate limiting on collect endpoint (5 req/s, burst 10) and login (5 attempts per 5 minutes)
- Strict Content Security Policy: `script-src 'self'`, no inline scripts
- Configurable CORS origins (`NANOLYTICA_CORS_ORIGINS`), scoped to public endpoints only
- Security headers: XSS protection, Content-Type nosniff, X-Frame-Options DENY
- Type-safe SQL via [sqlc](https://sqlc.dev/) -- no hand-written query strings
- Input validation: path length, screen size format, duration range, scroll depth range, body size limit (10KB)
- Client-side bot/automation detection (Selenium, Puppeteer, Cypress, PhantomJS)
- Gzip compression on all responses
- Docker runs as non-root user
- Graceful shutdown on SIGINT/SIGTERM

## Privacy

No PII stored. IP addresses are salted+hashed (irreversible). No cookies, no localStorage, no cross-site tracking. Honors `DNT: 1`. Query parameters stripped from URLs. Localhost traffic excluded automatically.

**Data collected:** page path, referrer domain, screen size, user-agent (for browser/OS/device detection), active engagement time, scroll depth.

GDPR/CCPA compliant by design.

## License

MIT. See [LICENSE](LICENSE).
