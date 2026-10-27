# Responsive web app and offline support

On phones, the bottom bar switches between RSS, Read Later, Sports, and Settings.
Within RSS, **Feeds** opens the sidebar, choosing a feed opens its article list,
and choosing an article opens its detail. The header returns to the previous
pane. The bottom bar stays visible in every section, including Settings, and
accounts for the device's bottom safe area. Phone landscape uses the same flow;
tablets retain the two-pane reader and desktops retain all three panes.

News caches visited RPC reads in IndexedDB, scoped to the account ID. Open an
article or saved link online before relying on it offline. Visited lists, stories,
settings, and sports responses are cached too. A search/filter that has not been
loaded may need a connection.

Article read/unread changes commit atomically with a durable outbox. Reconnecting
retries these absolute operations after verifying the server identity. Star
changes, feeds, new saved links, story operations, refreshes, and recrawls require
a connection and are never replayed speculatively. Offline read-state changes can
supersede another device's intervening read state.

On network failure, the last signed-in account can reopen its saved data. A
confirmed 401 or explicit sign-out removes its offline identity and clears that
account's read cache/outbox. Authorization failures never fall back to cached RPC
responses. Authentication/session responses never enter the service-worker cache.

## Notifications and platform support

Use **App** in the account bar to install the app, request persistent storage,
choose notifications, or apply an available app update. Updates activate only
when requested, so a waiting worker cannot reload an active editor. Browser data
can still be removed by the user; keep independent backups of important work.

Notification permission is requested only after **Enable notifications** is
clicked. The browser creates a Web Push subscription; the server sends encrypted
notifications through the browser's push provider even when the app is closed.
Notifications can be disabled per device. On iPhone/iPad, add the app to the Home
Screen first where required. Delivery still depends on the browser, OS settings,
and connectivity; there is no background polling loop in the page.

News sends a single digest alert only after every currently enabled feed has
successfully refreshed since the previous alert (or initial opt-in). Failed feeds
hold the digest until a successful refresh. Alerts require new articles and are
limited to one batch per account in a rolling 24-hour period, across all devices.

The server persists VAPID keys, subscriptions, activity rounds, daily limits and a
retry outbox in `news-push.json` beside its existing database/data directory.
Keep this private file on the existing persistent volume and include it in
backups. It is atomically replaced with owner-only permissions. No additional
secret configuration is required. Do not delete it during normal deployments:
rotating VAPID keys requires devices to subscribe again.

Subscriptions are bound to the signed-in account. Federated delivery uses a
separate encrypted, notification-only grant and revalidates the central session
family before sending. The grant cannot authenticate app requests. Temporary
authentication/provider failures retry; revoked grants and gone endpoints are
removed. Signing out disables the worker's account gate, unsubscribes the device,
and revokes its server binding. A queued notice for a different account is ignored.
Generic notification text avoids exposing document/article contents on the lock
screen. Push endpoint hosts are restricted to the Chrome, Firefox, Apple and
Windows push services; redirects and arbitrary outbound destinations are rejected.

## Service worker and deployment

The production build contains a manifest, 192px/512px install icons, an Apple touch
icon, and a versioned service worker. Serve from the origin root over HTTPS
(loopback HTTP works for development). Vite development does not register workers.
Only build assets enter the public cache. API, authentication, RPC, event-stream,
and session requests bypass it. Cached HTML is normalized to avoid redirected
responses failing offline navigation in Chromium. Worker/manifest HTTP responses
revalidate. Remote images, fonts, and original websites are not downloaded; text
and app controls remain available using system fonts when necessary.

## Validation

Run `bun test`, the Docker `backend-test` and `frontend-test` targets, and
`bun run --filter @rss-reader/desktop build:renderer`.
Install Playwright Chromium (`bunx playwright install chromium`) and run
`bun run test:pwa`. `PWA_BROWSER_PATH` can point to an existing Chromium binary.
The browser suite starts a loopback fixture server and tests nine touch viewport
shapes, offline navigation, a second reload, durable read-state replay, confirmed
revocation, and account switching. Mobile checks cover all four bottom destinations,
sidebar/list/detail transitions, touch target sizes, and content staying above the
bar, including while Settings scrolls. Screenshots go to `.cache/pwa/`.
