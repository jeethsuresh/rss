# RSS Reader for iOS

A SwiftUI client for the canonical RSS Reader server. The app intentionally has
no RSS polling, crawler, sports provider, AI pipeline, or persistent content
database. The server owns all content and user state; iOS keeps only the server
address and bearer token in Keychain plus transient UI state in memory.

## Open and run

```bash
cd apps/ios
xcodegen generate
open RSSReader.xcodeproj
```

Select an iPhone simulator and run `RSSReader`. Sign in with the URL of an HTTPS
RSS Reader server. Local-network HTTP is allowed for development, but deployed
servers should use TLS.

## Architecture

- `APIClient` is an actor and the sole HTTP/SSE boundary.
- `SessionStore` is main-actor isolated because it owns observable app/UI state.
- Feature views call the server directly through the session store; there is no
  mobile sync engine or local content source of truth.
- Server-sent events are tenant-safe invalidations. The app responds by bumping
  a refresh generation, and visible features reload their authorized data.
- Swift 6 strict concurrency is enabled. UI stores are explicitly main-actor
  isolated, while transport models and the API actor remain nonisolated.

The mobile UI is organized around bottom tabs, navigation stacks, full-screen
reader pages, pull-to-refresh, context menus, and leading/trailing swipe actions.
