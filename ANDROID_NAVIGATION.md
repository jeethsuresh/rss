# News Android navigation

The native Work/News shell lives in [Private Store](https://github.com/jeethsuresh/appstore/tree/main/webview-android).
Follow the [shared Gravitas Android contract](https://github.com/jeethsuresh/appstore/tree/main/integrations/navigation).
The shell handles Android 14+ left-edge Back/right-edge Forward, hardware Back,
keyboard and modal precedence, cancelled gestures, and older Android Back.

The hosted reader handles its state-based destinations in
`renderer/lib/hostedNavigation.ts` and `destinationHistory.ts`. It records at most
64 past destinations in memory, including RSS selections, reader/list/sidebar
panes, settings, content tabs and app modes. New destinations discard Forward.
Background data and typing refresh the current destination without adding visits.
The account-keyed app is unmounted on logout, revocation or an account change,
which discards its history and removes the event listeners.

The shell dispatches the cancelable `gravitas:navigate` DOM event with
`detail.direction` set to `back` or `forward`. The hosted reader prevents default
when it restores a destination, and always consumes empty Forward. Empty Back
falls through to native page history/root exit. The `gravitas:navigation-state`
event supplies a mutable detail object with `back`/`forward` booleans so the
shell's navigation buttons reflect the reader's state history. Neither event
exposes a privileged native JavaScript interface.

Restore setters directly; never call selection handlers that mark articles read
or issue other commands. Article restoration uses a read-only lookup, validates
access and falls back to the list for a removed/inaccessible item. Late lookups
are ignored after another interaction. Missing feeds/folders return to All Items.
Keep transient forms and consent out of destination snapshots. The shell handles
dialogs before dispatching state navigation. Future hosted screens must preserve
this contract when adding their own destinations.

Validation: `bun run typecheck`, `bun test`, the production renderer build and
`node scripts/test-pwa.mjs`. The browser fixture verifies pane Back/Forward,
branching, account isolation and no replay of mark-on-open or other mutations.
The shell additionally runs device tests on Android APIs 29 and 35.
