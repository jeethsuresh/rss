# Multi-tenant Go server

The standalone server stores one canonical copy of feeds, articles, crawled pages,
meta-stories, and sports payloads. User subscriptions and state are separate rows,
so shared upstream content is fetched once without sharing personal read/star,
Read Later, or followed-team state.

## Run locally

The server defaults to `127.0.0.1:8787` and `rss-server.db` in the current directory.

```bash
RSS_SERVER_REGISTRATION_ENABLED=true bun run server:dev
```

Then open `http://127.0.0.1:8787`. The browser app redirects unauthenticated
visitors to `/login`; while registration is enabled, the login screen can also
create an account. The browser holds only an HttpOnly, SameSite session cookie.
It uses the authenticated server RPC and event stream for every operation.

Create an account while registration is enabled:

```bash
curl -X POST http://127.0.0.1:8787/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"reader","password":"choose-a-long-password"}'
```

For a deployed instance, create the intended accounts and then restart without
`RSS_SERVER_REGISTRATION_ENABLED=true`. Put TLS in front of the server before
using it across an untrusted network. The HTTP server intentionally listens on
loopback by default.

Build a standalone binary:

```bash
bun run server:build
./apps/desktop/resources/bin/rss-server -db ./rss-server.db -addr 127.0.0.1:8787
```

## Run with Docker Compose

The included Compose service builds the Go server, persists SQLite in a named
volume, bundles the production web app, runs as an unprivileged user with all
Linux capabilities dropped, and publishes only to host loopback by default:

```bash
docker compose up --build -d
docker compose ps
curl http://127.0.0.1:8787/healthz
```

Registration defaults to enabled in Compose so a new database can create its
first account. After registering the intended users, set
`RSS_SERVER_REGISTRATION_ENABLED=false` in a local `.env` file and recreate the
container. Set `RSS_SERVER_BIND=0.0.0.0` only when a TLS reverse proxy or a
trusted private network is protecting the port. When TLS terminates at a reverse
proxy, also set `RSS_SERVER_COOKIE_SECURE=true`.

Back up the database with SQLite's online backup mechanism from inside the
volume; do not copy a live WAL database as independent files. To use a host
directory instead, replace `rss-server-data:/data` with an absolute bind mount
whose directory is writable by the container's `rss` user.

## Configuration

- `RSS_SERVER_DB`: SQLite database path. Default: `rss-server.db`.
- `RSS_SERVER_ADDR`: listen address. Default: `127.0.0.1:8787`.
- `RSS_SERVER_REGISTRATION_ENABLED`: permit `POST /v1/auth/register`. Default: `false`.
- `RSS_SERVER_WEB_DIR`: directory containing the built browser app. Docker sets
  this to `/app/web`; local commands discover `apps/desktop/dist-renderer`
  automatically.
- `RSS_SERVER_COOKIE_SECURE`: mark browser session cookies HTTPS-only. Enable
  this for TLS deployments.
- `RSS_SERVER_AI_ENABLED`: enable server-wide AI triage and AI meta-story actions.
- `RSS_SERVER_AI_BASE_URL`: OpenAI-compatible API base, such as `https://api.example.com/v1`.
- `RSS_SERVER_AI_MODEL`: model identifier sent to the compatible API.
- `RSS_SERVER_AI_API_KEY`: optional bearer token. It is kept in process memory and is not written to SQLite.

Deterministic meta-story clustering runs whether AI is enabled or not. When AI is
enabled, every newly stored canonical article is queued once for the existing AI
triage/meta-story workflow.

## Browser authority model

The hosted web app reuses the desktop React interface but replaces Electron IPC
with same-origin `POST /v1/rpc` calls. It never downloads upstream RSS or sports
data directly, runs page extraction, or performs deterministic/AI grouping in
the browser. Feed refreshes and recrawls only enqueue canonical server work;
sports reads use the shared server cache; AI settings and actions remain
server-wide. Server-sent events are reduced to tenant-safe invalidations, after
which each browser reloads only data authorized for its account.

The web endpoints are:

- `GET /v1/web/config` and authenticated `GET /v1/web/session`
- `POST /v1/web/login`, optionally `POST /v1/web/register`, and authenticated `POST /v1/web/logout`
- Authenticated `POST /v1/rpc` and `GET /v1/events`

Mutating browser requests require `X-RSS-CSRF: 1`; the server does not enable
cross-origin access. Browser session tokens are never returned in response JSON.

## Desktop synchronization

The desktop remains local-first. Set these variables when launching it:

```bash
RSS_SERVER_URL=http://127.0.0.1:8787 \
RSS_SERVER_USERNAME=reader \
RSS_SERVER_PASSWORD='choose-a-long-password' \
bun dev
```

Optional client variables:

- `RSS_SERVER_AUTO_REGISTER=true` attempts registration if login fails. The server must have registration enabled.
- `RSS_SYNC_INTERVAL_SECONDS=300` changes the periodic sync interval; the minimum is 30 seconds.

On migration, existing local feeds and tenant-owned state become append-only
events. Normal local creates, updates, and deletes are then captured by SQLite
triggers. The client pushes unacknowledged events, pulls events after separate
feed and state cursors, and applies the winning event for each object. Pulled
changes are guarded so they do not echo as new local operations.

Synchronized state includes feed membership, article read/star flags, Read Later
links and flags, folders and folder assignments, reader settings, and followed
MLB teams. Canonical feed content, crawled documents, sports payloads, and
meta-stories stay shared on the server; tenant state remains isolated.

The CRDT winner is the lexicographically greatest tuple:

```text
(logical_clock, device_id, op_id)
```

Every operation is retained in `feed_sync_ops`; `user_feeds` is only the current
materialized result. Deletes are tombstones, so an offline device cannot
accidentally resurrect a newer deletion with an older create.

## Fetch-once model

- A canonical feed URL has one `feeds` row and one `feed_fetch_state` row, regardless of subscriber count.
- A feed is eligible for polling only while at least one user subscribes to it.
- A database lease permits one process to poll a feed at a time.
- The next delay uses an exponential moving average of observed seconds per new item. Productive feeds poll sooner; unchanged feeds stretch toward 24 hours; failures back off exponentially.
- Articles are deduplicated within their canonical feed by GUID, URL, or fingerprint.
- Full-page crawls and Read Later crawls converge on one `web_documents` row per normalized URL. This also deduplicates the same article appearing in different feeds.
- Sports payloads use the global `sports_cache`; cache misses and refreshes use cross-process `shared_fetch_leases`.

Tenant-supplied feed and Read Later URLs use a guarded HTTP transport that rejects
loopback, private, carrier-grade NAT, link-local, unspecified, and multicast
addresses, including after redirects.

## Main API surface

Authentication uses opaque bearer tokens. Only SHA-256 token hashes are stored.
Passwords use PBKDF2-HMAC-SHA256 with a random per-user salt and 600,000
iterations.

- `POST /v1/auth/register`, `POST /v1/auth/login`, `GET /v1/me`
- `POST /v1/sync/feeds`, `POST /v1/sync/state`, `GET /v1/feeds`
- `GET /v1/articles`, `GET|PATCH /v1/articles/{id}`
- `GET /v1/stories`, `PATCH /v1/stories/{id}`
- Tenant folders under `/v1/folders` and preferences under `/v1/settings`
- `GET|POST /v1/read-later`, `GET|PATCH|DELETE /v1/read-later/{id}`
- `GET|PUT /v1/sports/followed/{sport}`
- Cached MLB routes under `/v1/sports/mlb/*`
- Cached Formula 1 routes under `/v1/sports/f1/*`
- `GET /v1/ai/status` and unauthenticated `GET /healthz`
