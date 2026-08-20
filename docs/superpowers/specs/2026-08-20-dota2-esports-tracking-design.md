# Dota 2 Esports Tracking — Cursor Implementation Specification

## Objective

Add Dota 2 esports tracking to the existing RSS reader application.

Dota 2 should be implemented as a **distinct sports experience**, not forced into the same data model or UI hierarchy as baseball.

Use:

- **PandaScore** for:
  - leagues
  - tournaments
  - seasons
  - event metadata
  - event tier
  - teams
  - scheduled matches/series
  - event status
- **STRATZ** for:
  - individual Dota 2 games
  - game results
  - heroes
  - hero picks/bans
  - items
  - kills
  - player statistics
  - game duration
  - detailed game events/statistics where available

The application must remain responsive and must aggressively avoid unnecessary API requests.

**API rate limiting and caching are first-class requirements.**

---

# 1. Core Product Concept

The Dota 2 experience should revolve around three things:

1. **Year**
2. **Leagues / Tournaments**
3. **Followed Teams**

The user should be able to:

- select a year
- browse Dota 2 leagues and tournaments for that year
- see their tier
- pin leagues/tournaments to the sidebar
- follow individual teams
- see matches for followed teams
- see matches inside pinned/selected events
- open a match/series
- inspect its individual games
- inspect detailed STRATZ game information

Do not attempt to make this identical to the baseball UI.

Dota has a fundamentally different competitive structure.

---

# 2. Data Provider Responsibilities

## PandaScore

PandaScore is the source of truth for the **competitive/event layer**.

Use PandaScore for:

```text
Year
  └── League
        └── Tournament
              └── Match / Series
                    ├── Team A
                    ├── Team B
                    ├── Scheduled time
                    ├── Status
                    ├── Score
                    └── PandaScore identifiers
```

PandaScore should provide:

- leagues
- tournaments
- tournament status
- tournament tier
- tournament dates
- tournament/league names
- tournament/league logos
- teams
- matchups
- match scheduling
- completed results
- ongoing matches
- event relationships

Do not depend on PandaScore for granular Dota game information when STRATZ can provide it.

---

# 3. STRATZ

STRATZ is the source of truth for **Dota 2 game-level information**.

Use STRATZ when the user opens an individual match/series and wants details.

Retrieve, where available:

- individual games
- game duration
- winner
- Radiant/Dire
- heroes
- hero picks
- hero bans
- players
- player teams
- kills
- deaths
- assists
- items
- item timings
- gold/net worth
- experience
- CS
- towers/buildings
- Roshan
- major game events
- other useful game statistics

Do not fetch detailed STRATZ information for every match automatically.

Fetch it **on demand**, then cache it.

---

# 4. Do NOT Over-Normalize

This is an explicit architectural requirement.

Do not create a generic abstraction such as:

```text
Sport
League
Season
Event
Game
Event
```

and force every sport to conform to it.

Baseball and Dota 2 should have different domain models where necessary.

Shared infrastructure is good.

Shared domain semantics are not required.

For example:

```text
sports/
  baseball/
    ...
  dota/
    ...
```

is preferable to creating a giant generic sports abstraction that both sports must implement.

Shared code may include:

- API caching
- background fetching
- request deduplication
- authentication
- rate limiting
- persistence
- sidebar infrastructure
- common navigation
- common loading/error components

But Dota's domain model and UI should be Dota-specific.

---

# 5. Recommended Dota Domain Model

Create Dota-specific types.

Example:

```ts
type DotaSeason = {
  year: number;
};

type DotaEvent = {
  id: string;
  name: string;

  type: "league" | "tournament";

  tier:
    | "amateur"
    | "semi-pro"
    | "professional"
    | "premier"
    | "unknown";

  status:
    | "upcoming"
    | "ongoing"
    | "completed";

  startAt?: string;
  endAt?: string;

  logoUrl?: string;

  leagueId?: string;
  tournamentId?: string;

  year: number;
};

type DotaTeam = {
  id: string;
  name: string;
  shortName?: string;
  logoUrl?: string;
};

type DotaMatch = {
  id: string;

  eventId?: string;

  teamA: DotaTeam;
  teamB: DotaTeam;

  scheduledAt?: string;

  status:
    | "upcoming"
    | "live"
    | "completed";

  bestOf?: number;

  scoreA?: number;
  scoreB?: number;

  year: number;
};

type DotaGame = {
  id: string;

  matchId: string;

  durationSeconds?: number;

  winner?: "radiant" | "dire";

  radiantTeam?: DotaTeam;
  direTeam?: DotaTeam;

  heroes?: DotaHeroPick[];
  bans?: DotaHeroBan[];

  players?: DotaPlayer[];
};

type DotaHeroPick = {
  heroId: number;
  heroName: string;
  playerId?: number;
  team?: "radiant" | "dire";
};

type DotaHeroBan = {
  heroId: number;
  heroName: string;
  team?: "radiant" | "dire";
};
```

These are examples, not requirements to copy literally.

Adapt them to the existing application's architecture.

---

# 6. Year Is the Primary Filter

The main Dota navigation should make **year** the most important organizing concept.

Example:

```text
Dota 2

2026
  Tournaments & Leagues
    The International
    ESL One
    DreamLeague
    BLAST Slam
    ...
```

The user should be able to change the year.

When the year changes:

1. Load the events for that year.
2. Group them into leagues/tournaments.
3. Display their tier.
4. Preserve followed teams independently of the selected year.
5. Preserve pinned events where appropriate, but hide pins that don't belong to the current year unless the UI explicitly shows them as historical.

Do not make the tournament organizer/company the primary hierarchy.

For example, do NOT organize primarily as:

```text
ESL
  ├── ESL One
  └── DreamLeague

Valve
  └── The International
```

Instead:

```text
2026

Premier
  The International
  ESL One
  ...

Professional
  ...
```

The entity/organizer can be displayed as metadata.

---

# 7. Event Tier Is Important

Tier is a first-class piece of UI information.

Users should immediately be able to distinguish:

- Premier
- Professional
- Semi-Pro
- Amateur
- Unknown

Use PandaScore's available tournament/league tier information and map it into the application's internal representation.

Do not discard tier information.

Display it in:

- event lists
- event headers
- sidebar entries where useful
- filtering/sorting controls

Example:

```text
2026

PREMIER
  The International

PROFESSIONAL
  ESL One
  DreamLeague

SEMI-PRO
  ...
```

The exact classification should follow PandaScore's actual API data rather than inventing a new ranking system.

---

# 8. Following Teams

The user must be able to follow individual Dota 2 teams.

Example:

```text
Following

★ Team Spirit
★ Tundra Esports
★ Nigma Galaxy
```

Following a team should allow the application to show:

- upcoming matches
- ongoing matches
- recent completed matches
- the event associated with each match

A followed team is independent of the selected year.

When viewing a year, only matches/events relevant to that year should be displayed.

Do not duplicate teams for every season.

---

# 9. Pinning Events

Users must be able to pin a Dota 2 league or tournament to the sidebar.

Examples:

```text
Dota 2
  ★ The International
  ★ ESL One Birmingham
  ★ DreamLeague Season 27
```

Pinned events should be persisted in the application's existing user/settings persistence mechanism.

A pin should identify the underlying PandaScore event ID, not just the display name.

Example:

```ts
type PinnedDotaEvent = {
  eventId: string;
  eventType: "league" | "tournament";
};
```

Do not use names as identifiers.

Events can have similar or changing names.

---

# 10. Sidebar

Add a Dota-specific section to the existing sidebar.

Suggested structure:

```text
Sports

Baseball
  ...

Dota 2
  Following
    Team Spirit
    Tundra

  Pinned
    The International
    ESL One Birmingham

  Browse
    2026
```

The exact visual structure should follow the existing application's design system.

Do not redesign unrelated portions of the application.

---

# 11. Event Page

Clicking an event should show:

```text
The International
PREMIER
2026

[Overview] [Matches] [Standings if available]

Upcoming
  Team A vs Team B
  Team C vs Team D

Live
  Team E 1 - 1 Team F

Completed
  Team G 2 - 0 Team H
```

The event page should be driven by PandaScore.

Do not request STRATZ data merely because the event page was opened.

STRATZ should only be consulted when game-level information is actually needed.

---

# 12. Match / Series Page

A Dota "match" from PandaScore should be treated as a **series** where appropriate.

Example:

```text
Team Spirit 2 — 1 Tundra Esports

ESL One
Playoffs
August 20, 2026

Games

Game 1
  34:12
  Team Spirit WIN

Game 2
  41:03
  Tundra WIN

Game 3
  29:47
  Team Spirit WIN
```

The series-level information comes from PandaScore.

The individual game information should come from STRATZ.

---

# 13. Game Detail Page

When the user opens an individual game, display useful Dota information.

Suggested structure:

```text
Game 3

Team Spirit
Radiant
WIN

Tundra Esports
Dire
LOSS

Duration
29:47

Heroes

Radiant
  Hero A
  Hero B
  Hero C
  Hero D
  Hero E

Dire
  Hero F
  Hero G
  Hero H
  Hero I
  Hero J

Bans
  ...

Score
  Kills
    24 - 13

Players
  Player
  Hero
  K
  D
  A
  GPM
  XPM
  Net Worth

Items
  ...
```

Where STRATZ provides the information, use it.

Do not invent statistics when they are unavailable.

---

# 14. Important: Do Not Fetch STRATZ Data Eagerly

This is critical.

Do NOT do this:

```text
Load event
  -> fetch all matches
  -> fetch every game's STRATZ data
  -> fetch all players
  -> fetch all items
  -> fetch all events
```

That will generate unnecessary API traffic.

Instead:

```text
Load event
  -> PandaScore matches only

Open match
  -> fetch STRATZ game metadata

Open game
  -> fetch detailed STRATZ statistics
```

Use lazy loading throughout.

---

# 15. API Rate Limiting

The application MUST stay under the API rate limits for both PandaScore and STRATZ.

Treat API rate limits as a hard engineering constraint.

Do not simply retry aggressively.

Implement:

- request throttling
- request deduplication
- caching
- exponential backoff
- stale-while-revalidate where appropriate
- background refresh
- cancellation of obsolete requests

Never create polling loops that can accidentally multiply requests.

---

# 16. Asynchronous Fetching

All API calls should be asynchronous.

The UI must never block while waiting for a complete hierarchy to load.

Prefer:

```text
Render page
  ↓
Load cached data immediately
  ↓
Display stale data if available
  ↓
Asynchronously refresh
  ↓
Update UI
```

For live matches:

```text
cached state
    ↓
background refresh
    ↓
updated state
```

Do not repeatedly fetch data just because a React component rendered.

Avoid `useEffect` dependency mistakes that create request loops.

---

# 17. Request Deduplication

If multiple components request the same resource simultaneously:

```text
Event A
  ├── Sidebar requests event
  ├── Event header requests event
  └── Match list requests event
```

there should be **one network request**, not three.

Create a shared request/cache layer.

Conceptually:

```ts
getCachedOrFetch(
  cacheKey,
  fetcher
)
```

If a request for the same key is already in flight, return the existing promise.

Example:

```ts
const promise = inflight.get(key);

if (promise) {
  return promise;
}

const request = fetchData();

inflight.set(key, request);

try {
  return await request;
} finally {
  inflight.delete(key);
}
```

Adapt this to the existing application's architecture.

---

# 18. Cache Strategy

Use different cache durations depending on the data.

Suggested defaults:

| Data | Cache |
|---|---:|
| Historical events | 24 hours |
| Historical matches | 6–24 hours |
| Teams | 24 hours |
| Tournament metadata | 12–24 hours |
| Upcoming matches | 1–5 minutes |
| Live matches | 5–15 seconds |
| STRATZ completed game | 24 hours+ |
| STRATZ historical game details | 24 hours+ |
| STRATZ live game data | 5–15 seconds |

These are starting points.

Make TTLs configurable.

The important principle is:

**Historical data should almost never be repeatedly fetched.**

---

# 19. Persistent Cache

If the existing application has a backend/database, cache API responses there where practical.

Do not depend solely on in-memory caching.

Recommended conceptual model:

```ts
ApiCacheEntry {
  provider: "pandascore" | "stratz";
  key: string;
  response: unknown;
  fetchedAt: Date;
  expiresAt: Date;
}
```

If the application already has an appropriate cache/database abstraction, use it instead of creating another storage mechanism.

---

# 20. Live Match Polling

Live Dota matches require more aggressive updates.

Do not globally poll all Dota matches.

Only poll matches that are:

- visible to the user
- pinned/followed and currently live
- explicitly opened

For example:

```text
User opens live match
       ↓
poll every ~10 seconds
       ↓
stop polling when match ends
```

Do not continue polling after the match is completed.

Prefer server-side/background refresh if the existing application architecture supports it.

---

# 21. Background Refresh

For frequently accessed data:

```text
cache is fresh
    -> return immediately

cache is stale but usable
    -> return cached value
    -> refresh asynchronously

cache missing
    -> fetch
    -> cache
    -> return
```

This should make the application feel instantaneous without increasing API usage unnecessarily.

---

# 22. API Errors

Handle:

- 401/403 authentication failures
- 404 missing resources
- rate limiting
- transient network errors
- provider outages
- malformed provider responses
- missing STRATZ game mappings

A STRATZ failure must NOT prevent PandaScore event information from displaying.

For example:

```text
Team Spirit 2 - 1 Tundra

Game 1
  Detailed statistics unavailable
  Retry

Game 2
  Detailed statistics available

Game 3
  Detailed statistics available
```

Partial data is preferable to an entirely broken match page.

---

# 23. PandaScore → STRATZ Mapping

This is one of the most important implementation details.

PandaScore and STRATZ will not necessarily use the same IDs.

Create an explicit mapping layer.

Example:

```ts
type DotaGameMapping = {
  pandaScoreMatchId: string;
  gameIndex: number;

  stratzMatchId?: string;

  confidence:
    | "exact"
    | "inferred"
    | "unknown";
};
```

Do not scatter ID matching logic throughout React components.

Put it in the Dota provider/service layer.

Investigate what identifiers/metadata are available from each provider and use the strongest possible mapping.

Potential matching signals may include:

- team names
- team IDs
- timestamps
- game ordering
- tournament/event
- match duration
- start time

Do not silently associate two games if the match cannot be established reliably.

---

# 24. Provider Architecture

Create provider modules along these lines:

```text
dota/
  providers/
    pandascore/
      client.ts
      events.ts
      teams.ts
      matches.ts

    stratz/
      client.ts
      games.ts
      heroes.ts
      players.ts
      items.ts

  services/
    eventService.ts
    teamService.ts
    matchService.ts
    gameService.ts
    mappingService.ts

  cache/
    ...

  types/
    ...

  ui/
    ...
```

Adapt the directory structure to the existing project.

Do not blindly create this exact structure if the application has an established architecture.

---

# 25. API Clients

Create isolated API clients.

For example:

```ts
pandaScoreClient
stratzClient
```

They should be responsible for:

- authentication
- HTTP requests
- provider-specific error handling
- request serialization
- response parsing

They should NOT contain UI logic.

Services should translate provider responses into Dota-specific application models.

---

# 26. Environment Variables

Never hardcode API credentials.

Use environment variables.

For example:

```env
PANDASCORE_API_TOKEN=
STRATZ_API_TOKEN=
```

Use the naming conventions already established in the project if different.

Do not expose private API tokens to the browser.

Requests requiring credentials should go through the application's backend/server layer.

---

# 27. Server-Side Proxying

Prefer:

```text
Browser
   ↓
Application backend
   ↓
PandaScore / STRATZ
```

rather than:

```text
Browser
   ↓
PandaScore
```

This keeps credentials private and gives us one place to implement:

- caching
- rate limiting
- request deduplication
- retries
- logging
- provider normalization

---

# 28. Data Loading Priorities

When opening Dota:

### Priority 1

Load:

- selected year
- cached events
- pinned events
- followed teams

### Priority 2

Load:

- matches for selected event
- matches for followed teams

### Priority 3

Load on demand:

- individual game data
- heroes
- items
- player statistics
- detailed events

Never block the initial UI on Priority 3.

---

# 29. Search

Eventually support searching for:

- teams
- tournaments
- leagues
- players

For the initial implementation, prioritize:

1. teams
2. events

Player search can be added later.

---

# 30. Team Page

A followed team should have a Dota-specific page:

```text
Team Spirit

Upcoming
  vs Tundra
  vs Aurora

Recent
  vs Team Liquid
  vs Xtreme Gaming

Events
  The International
  ESL One
```

The event associated with each match should always be visible.

This makes it easy to navigate from:

```text
Team
  ↓
Match
  ↓
Event
  ↓
Game
```

and:

```text
Event
  ↓
Match
  ↓
Game
```

---

# 31. Persistence

Persist:

- followed Dota teams
- pinned Dota events
- selected Dota year if the application already persists UI preferences

Do not persist transient API state unnecessarily.

Do not store API tokens in client-side persistence.

---

# 32. UI Principles

The UI should make the following immediately obvious:

### What year am I looking at?

Very prominent.

### What event is this?

Prominent.

### How important is this event?

Show tier prominently.

### Is this event ongoing?

Clearly indicate status.

### Is this match upcoming/live/completed?

Clearly indicate status.

### Who is playing?

Always obvious.

### How many games were played?

Obvious from the series score and game list.

### What happened in each game?

Available when opening a game.

---

# 33. Do Not Overload the User

Do not show every STRATZ statistic by default.

Start with:

```text
Result
Duration
Heroes
Bans
Kills
Player K/D/A
Items
```

Then allow richer details where useful.

The goal is to make the match understandable at a glance.

---

# 34. Refresh Behavior

When data is refreshed:

- avoid flashing/loading the entire page
- preserve existing cached data
- update only changed sections
- show subtle stale/loading state
- do not reset scroll position
- do not remount the entire match/game page

Use React Query/TanStack Query or the application's existing data-fetching mechanism if already installed.

If the project already has a caching/data layer, integrate with it instead of adding a competing system.

---

# 35. Rate Limit Safety

Implement a centralized provider rate limiter.

Conceptually:

```ts
class RateLimiter {
  async schedule<T>(
    provider: Provider,
    fn: () => Promise<T>
  ): Promise<T> {
    // enforce provider-specific limits
  }
}
```

Maintain separate limits for:

```text
PandaScore
STRATZ
```

Do not assume their limits are identical.

The exact limits should be configurable rather than hardcoded into business logic.

If provider documentation exposes rate-limit headers, use them.

When receiving HTTP 429:

1. inspect Retry-After if available
2. wait
3. exponentially back off
4. avoid retry storms
5. surface stale cached data when possible

---

# 36. Abort Obsolete Requests

If the user changes:

```text
2026 → 2025
```

while a 2026 request is still running, cancel it where supported.

Use `AbortController` or the existing request cancellation mechanism.

Do not allow obsolete requests to overwrite current state.

---

# 37. Testing Requirements

Add tests for:

### Provider clients

- successful responses
- authentication errors
- rate limiting
- malformed responses

### Cache

- fresh cache
- stale cache
- cache miss
- concurrent requests
- invalidation

### Dota services

- event retrieval
- team retrieval
- match retrieval
- STRATZ mapping
- missing STRATZ data

### UI

- year switching
- pinning events
- following teams
- opening events
- opening matches
- opening games
- live match refresh

### Critical test

Two simultaneous requests for the same resource must result in **one upstream API request**.

---

# 38. Logging / Diagnostics

Add useful structured logs around provider requests.

Example:

```text
[PandaScore] GET tournaments
[PandaScore] cache hit tournaments:2026

[STRATZ] GET game:12345
[STRATZ] cache miss game:12345

[STRATZ] rate limited; retrying in 5s
```

Do not log:

- API tokens
- credentials
- sensitive user information

---

# 39. Implementation Order

Implement in this order.

## Phase 1 — Infrastructure

- provider clients
- environment configuration
- rate limiter
- request deduplication
- cache
- error handling

## Phase 2 — PandaScore

Implement:

- years
- leagues
- tournaments
- tiers
- teams
- matches
- event status

## Phase 3 — Dota Navigation

Implement:

- Dota section
- year selector
- event list
- tier grouping
- event pages
- pinning
- followed teams

## Phase 4 — Team Tracking

Implement:

- team following
- followed team page
- upcoming matches
- live matches
- recent matches

## Phase 5 — STRATZ

Implement:

- PandaScore match → STRATZ mapping
- game list
- game details
- heroes
- bans
- kills
- player stats
- items

## Phase 6 — Live Data

Implement:

- live match detection
- selective polling
- cache refresh
- automatic stop when matches end

## Phase 7 — Polish

Implement:

- loading states
- partial-data handling
- error states
- animations/transitions
- responsive layout
- tests
- performance improvements

---

# 40. Important Constraints

The implementation MUST satisfy all of the following:

- Do not force Dota into the baseball data model.
- Do not over-normalize sports concepts.
- PandaScore owns event/league/tournament information.
- STRATZ owns detailed Dota game information.
- Year is the primary navigation/filter.
- Tournament/league is the next major grouping.
- Event tier is highly visible.
- Organizer is secondary metadata.
- Users can follow teams.
- Users can pin leagues/tournaments.
- Pinned events appear in the sidebar.
- STRATZ data is loaded lazily.
- Historical data is heavily cached.
- Live data is selectively refreshed.
- Requests are asynchronous.
- Identical concurrent requests are deduplicated.
- API rate limits are respected.
- 429 responses are handled safely.
- Credentials never reach the browser.
- Partial provider failures do not destroy the entire UI.
- Existing application architecture and styling should be reused wherever practical.
- Do not modify unrelated application functionality.

---

# 41. Definition of Done

The feature is complete when a user can:

1. Open the Dota 2 section.
2. Select a year.
3. See that year's leagues and tournaments.
4. See their tiers.
5. Pin an event.
6. See the event in the sidebar.
7. Open an event.
8. See its upcoming, live, and completed matches.
9. Follow a Dota 2 team.
10. See that team's matches.
11. Open a completed series.
12. See the individual games.
13. Open a game.
14. See STRATZ-derived heroes, bans, kills, scores, players, and items where available.
15. Open a live match and see it update.
16. Navigate away without leaving unnecessary polling running.
17. Return later and receive cached data immediately.
18. Continue using the application gracefully if either PandaScore or STRATZ is temporarily unavailable.

Most importantly, the implementation should feel like a **native Dota 2 experience inside the RSS reader**, rather than baseball functionality with Dota names substituted into it.

Before writing new infrastructure, inspect the existing application and reuse its existing:

- routing
- persistence
- caching
- API/server patterns
- authentication
- styling
- state management
- component conventions

Do not introduce a new framework or state-management system unless the existing architecture genuinely cannot support the feature.

Start by inspecting the repository and identifying the smallest set of changes required to implement this specification.