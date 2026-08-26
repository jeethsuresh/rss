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

## Configuration

- `RSS_SERVER_DB`: SQLite database path. Default: `rss-server.db`.
- `RSS_SERVER_ADDR`: listen address. Default: `127.0.0.1:8787`.
- `RSS_SERVER_REGISTRATION_ENABLED`: permit `POST /v1/auth/register`. Default: `false`.
- `RSS_SERVER_AI_ENABLED`: enable server-wide AI triage and AI meta-story actions.
- `RSS_SERVER_AI_BASE_URL`: OpenAI-compatible API base, such as `https://api.example.com/v1`.
- `RSS_SERVER_AI_MODEL`: model identifier sent to the compatible API.
- `RSS_SERVER_AI_API_KEY`: optional bearer token. It is kept in process memory and is not written to SQLite.

Deterministic meta-story clustering runs whether AI is enabled or not. When AI is
enabled, every newly stored canonical article is queued once for the existing AI
triage/meta-story workflow.

## Desktop feed synchronization

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

On migration, every existing local feed becomes a `present=true` event. Normal
local feed creates and deletes are then captured by SQLite triggers. The client
pushes unacknowledged events, pulls events after its cursor, and applies the
winning event for each URL. Pulled changes are guarded so they do not echo as new
local operations.

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
- `POST /v1/sync/feeds`, `GET /v1/feeds`
- `GET /v1/articles`, `GET|PATCH /v1/articles/{id}`
- `GET /v1/stories`, `PATCH /v1/stories/{id}`
- Tenant folders under `/v1/folders` and preferences under `/v1/settings`
- `GET|POST /v1/read-later`, `GET|PATCH|DELETE /v1/read-later/{id}`
- `GET|PUT /v1/sports/followed/{sport}`
- Cached MLB routes under `/v1/sports/mlb/*`
- Cached Formula 1 routes under `/v1/sports/f1/*`
- `GET /v1/ai/status` and unauthenticated `GET /healthz`

The current desktop sync intentionally covers feed membership only. The server
already owns APIs and tenant storage for article state, Read Later, stories, and
sports tracking, leaving those client sync adapters as follow-on work without a
schema redesign.
