# RSS Reader — Cursor One-Shot Build Specification

You are building a production-quality desktop RSS reader from scratch.

The application should be designed so that **the desktop version is a local-first application today, while the Go backend can later be deployed as a standalone server with minimal architectural changes**.

The most important architectural principle is:

> **Build one application/backend with two possible deployment modes, not two separate applications.**

Do not prematurely implement the future server. Build the local application correctly so the future server is a natural deployment of the same Go application.

---

# 1. Product

Build a polished, modern desktop RSS reader.

The initial application should allow a user to:

- Add RSS/Atom feeds
- Organize feeds
- Poll feeds automatically
- View articles
- Mark articles read/unread
- Star/save articles
- Search articles
- Filter articles
- View article metadata
- Open articles in the system browser
- Manage feed refresh intervals
- See feed errors/status
- Persist everything locally
- Work entirely without an external server

The application should feel like a real, polished desktop product rather than a prototype.

Do not add accounts, authentication, cloud synchronization, or remote servers yet.

However, design the architecture so these can be added later without rewriting the application.

---

# 2. Technology Stack

Use:

## Desktop

- Electron
- React
- TypeScript
- Vite
- Bun for package management, scripts, and JavaScript/TypeScript tooling

## Backend

- Go
- The Go backend runs as a child process of Electron in the desktop application
- The Go backend owns application state and business logic
- The Go backend owns RSS fetching and polling
- The Go backend owns the database

## Database

- SQLite
- Go owns all SQLite access
- Do not access SQLite directly from React or Electron's TypeScript code

## RSS

The Go backend should handle:

- RSS 2.0
- Atom
- Common real-world feed variations
- Feed parsing
- HTTP fetching
- Redirects
- Conditional requests where practical (`ETag`, `Last-Modified`)
- Feed errors
- Deduplication
- Polling

Use mature Go libraries where appropriate rather than implementing XML parsing from scratch.

---

# 3. Critical Architecture

Use this architecture:

```text
┌──────────────────────────────────────────────┐
│                  Electron                    │
│                                              │
│  ┌────────────────────────────────────────┐  │
│  │ React Renderer                         │  │
│  │                                        │  │
│  │ UI / views / interaction               │  │
│  └──────────────────┬─────────────────────┘  │
│                     │ Electron IPC           │
│  ┌──────────────────▼─────────────────────┐  │
│  │ Electron Main / Preload               │  │
│  │                                        │  │
│  │ Process management / IPC / OS APIs    │  │
│  └──────────────────┬─────────────────────┘  │
│                     │ local IPC              │
│  ┌──────────────────▼─────────────────────┐  │
│  │ Go Backend                             │  │
│  │                                        │  │
│  │ Domain                                 │  │
│  │ RSS                                    │  │
│  │ Scheduler                              │  │
│  │ Database                               │  │
│  │ Application services                   │  │
│  └──────────────────┬─────────────────────┘  │
│                     │                        │
│              ┌──────▼──────┐                 │
│              │   SQLite    │                 │
│              └─────────────┘                 │
└──────────────────────────────────────────────┘
```

The Go backend should be treated as the **actual application backend**.

Electron is primarily the desktop shell and UI.

---

# 4. Do NOT Build This

Do not:

- Build a Node backend
- Build a Bun backend
- Build a local HTTP server unless there is a compelling technical reason
- Put business logic in React
- Put RSS parsing in React
- Put RSS fetching in React
- Access SQLite from JavaScript
- Create a second database/state model in Electron
- Implement authentication
- Implement user accounts
- Implement cloud synchronization
- Implement the future remote server
- Add unnecessary microservices
- Introduce Docker
- Introduce Kubernetes
- Introduce a message broker
- Introduce Redis
- Over-engineer the application

The initial application should be a single desktop product with a Go backend process.

---

# 5. Repository Structure

Use a monorepo roughly like:

```text
rss-reader/
├── apps/
│   └── desktop/
│       ├── electron/
│       │   ├── main.ts
│       │   └── preload.ts
│       │
│       ├── renderer/
│       │   ├── components/
│       │   ├── views/
│       │   ├── hooks/
│       │   ├── lib/
│       │   └── ...
│       │
│       ├── package.json
│       ├── vite.config.ts
│       └── ...
│
├── backend/
│   ├── cmd/
│   │   └── desktop/
│   │       └── main.go
│   │
│   ├── internal/
│   │   ├── domain/
│   │   ├── feeds/
│   │   ├── articles/
│   │   ├── rss/
│   │   ├── scheduler/
│   │   ├── storage/
│   │   │   └── sqlite/
│   │   └── application/
│   │
│   ├── migrations/
│   ├── go.mod
│   └── ...
│
├── package.json
├── bun.lock
├── README.md
└── ...
```

Adjust the exact structure when necessary, but preserve the separation between:

1. React UI
2. Electron shell
3. Go application/backend
4. SQLite persistence

Do not create `server/` yet.

The Go backend should eventually be capable of gaining a second command such as:

```text
backend/cmd/server/
```

without requiring a rewrite of the existing backend.

---

# 6. Go Backend Architecture

Use idiomatic Go.

Prefer simple, explicit architecture over excessive abstraction.

Suggested conceptual layers:

```text
Transport / IPC
       ↓
Application Services
       ↓
Domain
       ↓
Repositories
       ↓
SQLite
```

The domain/application layer must not know whether it is being used by Electron or a future HTTP server.

For example:

```go
type FeedRepository interface {
    List(ctx context.Context) ([]Feed, error)
    Get(ctx context.Context, id string) (*Feed, error)
    Create(ctx context.Context, feed *Feed) error
    Update(ctx context.Context, feed *Feed) error
    Delete(ctx context.Context, id string) error
}
```

Similarly define appropriate repository interfaces for articles and settings.

Do not create interfaces merely for the sake of having interfaces. Use them where they establish a useful architectural boundary.

---

# 7. Domain Models

Create clean domain models for at least:

## Feed

Include appropriate fields such as:

- ID
- URL
- title
- description
- site URL
- icon/favicon URL if available
- last successful fetch
- last attempted fetch
- last error
- ETag
- Last-Modified
- polling interval
- enabled/disabled state
- created timestamp
- updated timestamp

## Article

Include appropriate fields such as:

- ID
- feed ID
- title
- URL
- author
- content
- summary
- published timestamp
- updated timestamp
- GUID/external ID
- read state
- starred/saved state
- discovered timestamp

Do not blindly copy every field exposed by every RSS format.

Normalize the data into a useful internal representation.

---

# 8. Article Identity and Deduplication

RSS feeds are messy.

Do not assume that every feed provides a reliable GUID.

Implement sensible article identity/deduplication logic using:

1. Feed-provided GUID when reliable
2. Canonical article URL
3. Reasonable fallback fingerprinting when necessary

The same article should not appear repeatedly just because the feed changed ordering or formatting.

Design this carefully.

---

# 9. RSS Fetching

The Go backend should have a dedicated RSS service.

Conceptually:

```text
Feed
 ↓
HTTP Fetch
 ↓
Response validation
 ↓
Parse RSS/Atom
 ↓
Normalize
 ↓
Deduplicate
 ↓
Persist
 ↓
Notify application
```

Handle:

- HTTP redirects
- HTTP errors
- timeouts
- malformed XML
- unsupported feeds
- empty feeds
- invalid URLs
- TLS errors
- temporary network failures
- 304 Not Modified
- ETag
- Last-Modified

Set sensible HTTP timeouts.

Do not allow one broken feed to stop other feeds from updating.

---

# 10. Feed Polling

Implement a background scheduler in Go.

The scheduler should:

- maintain feed polling schedules
- avoid polling disabled feeds
- avoid overlapping refreshes of the same feed
- allow manual refresh
- handle failures without killing the scheduler
- use reasonable backoff for repeatedly failing feeds
- recover automatically after temporary failures

Do not spawn uncontrolled goroutines.

Use a clean cancellation/shutdown model with `context.Context`.

The backend must shut down cleanly when Electron exits.

---

# 11. Local Database

Use SQLite.

The Go backend owns the connection and all database operations.

Create migrations rather than relying on implicit schema creation scattered throughout the application.

Likely tables:

```text
feeds
articles
folders
feed_folders
settings
```

You may introduce additional tables when justified.

Indexes should be added for common operations such as:

- article lookup by feed
- article lookup by URL/GUID
- unread filtering
- starred filtering
- published date ordering
- search

Do not prematurely optimize.

---

# 12. Full-Text Search

If practical, use SQLite FTS5 for article search.

Search should cover useful article fields such as:

- title
- author
- content
- summary

Do not implement an external search service.

---

# 13. Communication Between Electron and Go

Prefer a local IPC mechanism rather than localhost HTTP.

The exact mechanism can be selected based on platform support and Electron integration quality.

Good options include:

- stdin/stdout JSON-RPC-style communication
- local sockets
- another simple cross-platform IPC mechanism

The protocol should be:

- request/response based
- structured
- typed
- versionable
- easy to debug

For example:

```json
{
  "id": "123",
  "method": "feeds.list",
  "params": {}
}
```

Response:

```json
{
  "id": "123",
  "result": {
    "feeds": []
  }
}
```

Errors:

```json
{
  "id": "123",
  "error": {
    "code": "FEED_NOT_FOUND",
    "message": "Feed not found"
  }
}
```

Do not expose arbitrary Go functions to the renderer.

Only expose explicit application operations.

---

# 14. Electron Security

Treat Electron security seriously.

Use:

- `contextIsolation: true`
- `nodeIntegration: false`
- a restrictive Content Security Policy
- preload APIs for renderer access
- explicit IPC channels
- no arbitrary code execution from renderer content

The RSS article content is untrusted.

Never allow an RSS article to gain access to Electron APIs.

Do not directly load arbitrary remote websites inside the privileged Electron renderer.

When the user chooses to open an article, open the URL using the system browser.

---

# 15. React UI

The UI should be polished and desktop-oriented.

Use a clean three-pane RSS-reader layout:

```text
┌─────────────────────────────────────────────────────────┐
│ Toolbar                                                  │
├──────────────┬──────────────────────┬───────────────────┤
│              │                      │                   │
│ Sidebar      │ Article List         │ Article           │
│              │                      │                   │
│ All          │ Article 1            │ Title             │
│ Unread       │ Article 2            │                   │
│ Starred      │ Article 3            │ Metadata          │
│              │                      │                   │
│ Folders      │                      │ Content           │
│              │                      │                   │
│ Feeds        │                      │                   │
│              │                      │                   │
└──────────────┴──────────────────────┴───────────────────┘
```

The exact design can improve upon this.

Prioritize:

- excellent typography
- clear hierarchy
- keyboard navigation
- fast interaction
- clean empty states
- subtle animations
- dark/light themes
- responsive resizing
- accessible controls

Avoid making it look like a generic admin dashboard.

It should feel like a polished native reading application.

---

# 16. Sidebar

Include:

- All Articles
- Unread
- Starred
- Recently Read if useful
- folders
- feeds
- Add Feed
- Settings

Show unread counts where useful.

Allow feeds to be organized into folders.

---

# 17. Article List

The article list should display:

- unread/read state
- feed
- title
- author if available
- publication time/date
- short summary
- starred state

Provide useful sorting/filtering.

Default sorting should generally be newest first.

Unread articles should be visually distinct without becoming visually overwhelming.

---

# 18. Article Reader

The reader should provide:

- title
- source/feed
- author
- publication date
- article content/summary
- star/save button
- mark read/unread
- open original article
- next/previous article navigation where practical

Article HTML is untrusted.

Sanitize it before displaying it.

Do not allow arbitrary scripts, forms, Electron APIs, or unsafe embeds.

Prefer a clean reading experience over reproducing arbitrary publisher pages exactly.

---

# 19. Add Feed

The user should be able to paste a feed URL.

Flow:

```text
Add Feed
   ↓
Enter URL
   ↓
Fetch feed
   ↓
Parse
   ↓
Show preview
   ↓
Confirm
   ↓
Save
   ↓
Initial article import
```

Validate URLs.

Provide useful errors.

If the URL is invalid or doesn't appear to contain a supported RSS/Atom feed, explain the problem clearly.

If practical, support feed discovery when the user enters a website URL rather than a direct feed URL.

---

# 20. Feed Refresh UX

Provide:

- global refresh
- individual feed refresh
- visible refreshing state
- last updated time
- error state
- retry action

The application should clearly distinguish:

- currently refreshing
- successfully refreshed
- never refreshed
- failed refresh
- feed disabled

---

# 21. Settings

Include useful settings such as:

- default polling interval
- theme
- article density
- default sort order
- mark article read behavior
- notification preferences if implemented
- database/storage information
- application version

Do not build an enormous settings system.

---

# 22. Notifications

If practical, support desktop notifications for new articles.

Make notifications opt-in or configurable.

Do not notify for every article by default if that would result in notification spam.

Consider allowing notifications per feed.

---

# 23. Keyboard Shortcuts

Build keyboard navigation into the architecture.

Useful shortcuts could include:

```text
j       next article
k       previous article
o       open article
r       mark read
u       mark unread
s       star/unstar
a       archive if archive exists
f       refresh
/       search
```

Use sensible conventions and make shortcuts discoverable.

Do not make shortcuts mandatory for basic functionality.

---

# 24. State Management

Keep frontend state simple.

Do not introduce a giant state-management framework unless necessary.

Separate:

- server/backend state
- UI state
- temporary interaction state

The backend should be authoritative for feeds/articles.

The frontend should react to backend events and requests.

---

# 25. Backend Events

The Go backend should be able to notify Electron when things happen.

Examples:

```text
feed.updated
feed.error
articles.added
article.updated
sync.status
```

This will allow the UI to update automatically when background polling discovers new articles.

Do not make the frontend poll the backend unnecessarily.

---

# 26. Future Server Compatibility

This is extremely important.

The backend should be written so a future command can be added:

```text
backend/cmd/server/main.go
```

That future server should be able to reuse:

- domain models
- RSS fetching
- feed management
- article management
- scheduler
- repository interfaces
- application services

The future server should be able to replace:

```text
SQLite
```

with:

```text
PostgreSQL
```

without rewriting domain logic.

Do not implement the HTTP server now.

Do not implement authentication now.

Do not implement synchronization now.

But don't make the local architecture incompatible with them.

---

# 27. Local vs Remote Backend Boundary

Think of the UI as talking to an abstract backend API.

Conceptually:

```ts
interface ReaderBackend {
  feeds: {
    list(): Promise<Feed[]>
    get(id: string): Promise<Feed>
    add(url: string): Promise<Feed>
    remove(id: string): Promise<void>
    refresh(id: string): Promise<void>
  }

  articles: {
    list(query: ArticleQuery): Promise<Article[]>
    get(id: string): Promise<Article>
    markRead(id: string): Promise<void>
    markUnread(id: string): Promise<void>
    toggleStar(id: string): Promise<void>
  }
}
```

The desktop implementation communicates with the local Go process.

A future implementation could communicate with:

```text
HTTP → Go server
```

Do not make the React UI care which implementation is active.

---

# 28. Build Tooling

Use Bun for the JavaScript/TypeScript side.

The root scripts should make the project easy to operate.

Aim for commands like:

```bash
bun install
bun dev
bun test
bun lint
bun build
```

`bun dev` should start the entire desktop development environment.

It should:

1. Build/watch the Go backend
2. Start the frontend dev server
3. Start Electron
4. Clean up child processes on exit

Avoid requiring the developer to manually run several terminals.

Go should use normal Go tooling:

```bash
go test ./...
go build ./...
go vet ./...
```

Bun should orchestrate these where convenient.

---

# 29. Cross-Platform Support

Target:

- macOS
- Windows
- Linux

Do not assume macOS-specific paths or APIs.

Use platform-aware application data directories.

The SQLite database should live in the appropriate Electron application data directory.

The bundled Go binary must be built for the target platform/architecture.

Account for at least:

- macOS ARM64
- macOS x64
- Windows x64
- Linux x64

Support additional architectures when straightforward.

---

# 30. Packaging

The production Electron application should bundle:

```text
Electron
React application
Go backend binary
```

The user should install one application.

They should not need to separately install:

- Go
- Bun
- Node
- SQLite
- Python
- any development dependencies

The Go backend should not depend on a system Go installation at runtime.

---

# 31. Graceful Shutdown

When Electron exits:

1. Tell Go to shut down
2. Stop feed polling
3. Finish/abort active requests safely
4. Close SQLite
5. Exit Go process
6. Exit Electron

Avoid orphaned backend processes.

Handle unexpected Electron termination reasonably.

---

# 32. Logging

Implement structured but human-readable logging.

Go backend logs should include:

- startup
- shutdown
- feed fetches
- feed failures
- database errors
- unexpected errors

Do not log:

- passwords
- secrets
- sensitive user content unnecessarily

Make development logs easy to inspect.

---

# 33. Error Handling

Errors should be:

- typed
- meaningful
- propagated correctly
- safe to show to users

Distinguish between:

```text
Network error
Feed parsing error
Invalid feed
Database error
Application error
User input error
```

Do not dump stack traces into the UI.

Do preserve useful diagnostic information in logs.

---

# 34. Testing

Build tests as you build the application.

## Go

Test:

- RSS parsing
- article normalization
- deduplication
- feed refresh
- repository operations
- migrations
- scheduler behavior
- error handling

Use fixtures for real-world RSS/Atom examples.

## TypeScript

Test:

- IPC client
- frontend utility functions
- important UI behavior

Do not chase arbitrary test coverage numbers.

Test the areas where regressions would be expensive.

---

# 35. Seed/Test Data

Provide a development mode that can easily populate a local database with sample feeds/articles.

This should make it possible to develop the UI without waiting for live feeds.

Do not make fake/test data part of production.

---

# 36. Database Migration Strategy

Migrations should be explicit and versioned.

The backend should:

1. Open database
2. Check schema version
3. Apply pending migrations
4. Start application

Never silently destroy an existing database during development or upgrades.

---

# 37. Performance

The application should feel instant.

Pay particular attention to:

- large article lists
- thousands of articles
- database queries
- search
- scrolling
- background feed polling
- IPC
- rendering article HTML

Do not load the entire article database into React state.

Use pagination, limits, cursors, or virtualization where appropriate.

---

# 38. Security

RSS content must be considered hostile/untrusted.

Protect against:

- XSS
- malicious HTML
- JavaScript execution
- dangerous URLs
- HTML injection
- arbitrary Electron API access

Never grant remote article content access to Node/Electron privileges.

External links should be opened through a controlled mechanism/system browser.

---

# 39. Design Philosophy

Favor:

- simple
- fast
- polished
- understandable
- maintainable
- local-first
- offline-capable
- privacy-friendly

Avoid:

- unnecessary abstraction
- unnecessary dependencies
- premature distributed architecture
- over-engineered state management
- excessive configuration
- enterprise architecture for a desktop RSS reader

The user should be able to install the application and immediately use it.

---

# 40. Development Order

Implement in approximately this order:

## Phase 1 — Project foundation

- Create repository
- Set up Bun
- Set up Electron
- Set up React + TypeScript + Vite
- Set up Go module
- Establish Electron ↔ Go process communication
- Establish secure preload API
- Add development orchestration

Do not proceed until `bun dev` starts the complete application reliably.

## Phase 2 — Database

- SQLite
- migrations
- feeds table
- articles table
- settings
- repository layer
- tests

## Phase 3 — RSS

- RSS/Atom parser
- feed fetcher
- normalization
- deduplication
- initial import
- tests

## Phase 4 — Feed management

- add feed
- remove feed
- list feeds
- refresh feed
- automatic polling
- errors/status

## Phase 5 — Article UI

- article list
- article reader
- read/unread
- starred
- filtering
- search

## Phase 6 — Polish

- keyboard shortcuts
- dark/light mode
- notifications
- animations
- loading states
- error states
- empty states
- accessibility
- performance

## Phase 7 — Packaging

- macOS
- Windows
- Linux
- bundled Go binaries
- production builds
- installer/package testing

Do not start implementing the future server until the desktop application is complete and stable.

---

# 41. Important Implementation Rule

Do not stop at scaffolding.

Build a **working application end-to-end**.

I want to be able to:

```text
Install
  ↓
Launch
  ↓
Add RSS feed
  ↓
Feed is fetched
  ↓
Articles appear
  ↓
Read article
  ↓
Mark read
  ↓
Star article
  ↓
Search
  ↓
Close application
  ↓
Reopen
  ↓
Everything is still there
```

The application should work through this complete lifecycle before considering the project finished.

---

# 42. Use Good Judgment

You are authorized to make reasonable implementation decisions without asking me questions when the choice does not materially affect the architecture.

Prefer established, mature libraries.

Before adding a dependency, ask whether the standard library or an existing dependency can reasonably solve the problem.

If two approaches are comparable, prefer the simpler one.

Do not introduce a dependency merely because it is fashionable.

---

# 43. Documentation

Create a useful `README.md` explaining:

- what the application is
- architecture
- prerequisites
- development setup
- `bun dev`
- testing
- building
- packaging
- Go backend architecture
- how Electron communicates with Go
- where SQLite lives
- future server architecture

Also create an architecture document if useful:

```text
docs/architecture.md
```

Document the important architectural decisions and, especially, explain the boundary between:

```text
React
Electron
Go
SQLite
future Go server
```

---

# 44. Definition of Done

The project is complete when:

- `bun install` works
- `bun dev` launches the complete application
- Electron launches the Go backend automatically
- React communicates with Go through a secure API
- Go owns SQLite
- feeds can be added
- RSS and Atom feeds can be parsed
- feeds automatically poll
- articles persist
- articles can be read/unread
- articles can be starred
- articles can be searched
- article HTML is safely sanitized
- errors are handled cleanly
- the application works offline with previously downloaded articles
- the UI is polished
- the application shuts down cleanly
- production builds bundle the Go backend
- macOS/Windows/Linux packaging is addressed
- tests cover important backend behavior
- README documents the architecture
- the Go architecture makes adding `cmd/server` later straightforward

Most importantly:

> **The final codebase should feel like one cohesive application, not an Electron project awkwardly glued to a Go project.**

The Go backend is the application's core. Electron is its desktop shell.

Build accordingly.

---

# 45. Final Instruction to Cursor

Start by inspecting the repository.

If the repository is empty, initialize the entire project.

If there is already existing code, preserve useful work and adapt it to this architecture rather than blindly replacing it.

Do not ask me to make routine architectural decisions that are already specified above.

Implement the application incrementally, keeping it runnable after each major phase.

When a choice is unspecified, use the simplest production-quality solution consistent with the architecture.

**Do not merely generate a plan. Actually implement the application.**

At the end, verify:

```bash
bun install
bun dev
bun test
go test ./...
```

and fix any resulting errors.

The result should be a polished, functional desktop RSS reader with a Go backend that can later become a standalone Go server without requiring the application to be fundamentally rewritten.