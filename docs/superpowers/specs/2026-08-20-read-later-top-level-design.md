# Read Later as top-level app mode

**Date:** 2026-08-20  
**Status:** Approved  

## Goal

Make Read Later a first-class app mode (not a sidebar section), with a working two-pane UX, archive support, and the ability to send RSS articles into Read Later.

## Chrome

- Toolbar mode tabs **`RSS Reader` | `Read Later`** only (no alternate chrome toggle).
- Settings / connection status available in both modes.
- Remove the RSS sidebar “Read later” / “+ Add link” block entirely.

## Read Later surface

Same **3-pane** layout as RSS for uniformity.

**Left sidebar:** All | Unread | Starred | Archived (nav items, same style as RSS filters).

**Toolbar (Read Later mode):** search field + always-visible URL field + Add (mirrors RSS search / Add feed).

**Middle pane:** article list (title, host, relative time).

**Right pane:** reader — Live / Saved crawl + PageFrame. Actions: mark read/unread, star, Archive / Unarchive, Open original.

- All / Unread / Starred exclude archived; Archived shows only archived.

## Send from RSS

On the RSS article reader (non–Read Later items), a **Send to Read Later** button:

- Creates (or upserts) a Read Later article for that URL, copying title when available
- Kicks crawl / live fetch / AI enqueue asynchronously (must not block the RPC on crawl)
- Switches app mode to Read Later and selects the new/updated item
- If already saved (same Read Later fingerprint/URL), select existing and switch mode (idempotent)

## Data / API

- Keep system feed `readlater://local` and `is_read_later`.
- Migration `004`: `articles.archived_at TEXT NULL`.
- Settings: persist `readLaterChrome`.
- IPC:
  - `readLater.list` params: `{ filter?: "all"|"unread"|"starred"|"archived" }`
  - `readLater.add` `{ url }` — non-blocking crawl
  - `readLater.addFromArticle` `{ articleId }` — send RSS → Read Later
  - `readLater.archive` / `readLater.unarchive` `{ id }`
- Shared types updated for `archivedAt`, settings field, and IPC methods.

## Reliability

- Add / send must return and refresh the list even if crawl or live fetch fails.
- Show crawl status in the reader; Re-crawl remains available.

## Out of scope

- Hard delete, folders inside Read Later, search-in-Read-Later
- Deep polish of `brandControl` chrome beyond a thin toggle
- Packaging changes

## Success criteria

- Top-bar mode switch lands in a working 2-pane Read Later.
- URL field adds items that appear in the list.
- Filters (including Archived) behave as specified.
- Archive / Unarchive work; All hides archived.
- RSS **Send to Read Later** creates/selects the item and switches mode.
- RSS sidebar no longer contains Read Later.
