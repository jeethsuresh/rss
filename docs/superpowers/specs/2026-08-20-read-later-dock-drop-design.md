# Read Later dock & window drop

**Date:** 2026-08-20  
**Status:** Approved

## Goal

Drop a link on the **macOS Dock icon** or **anywhere on the foreground window** to add it to Read Later. Valid URLs save quietly with a toast + Undo. Invalid text foregrounds the window and opens a modal to fix the URL.

## Capture

### Dock (Electron main)

- Handle macOS `open-url` (and cold-start queued URLs).
- Focus + show the main window, then forward the raw string to the renderer via IPC.
- Prefer not to register as the default `http`/`https` handler; use Alternate document types for `public.url` when packaging is added later.
- Optionally accept `.webloc` / `.url` via `open-file` when practical.

### Window (renderer)

- Document-level `dragover` / `drop` while the window is focused.
- Prefer `text/uri-list`, then `text/plain`.
- Take the first URI / first non-empty line.
- Skip drops whose target is an editable field (`input`, `textarea`, `[contenteditable]`).

## Validation

- Valid: parses as `http://` or `https://`, or scheme-less host that normalizes to `https://…` (same spirit as `readLater.add`).
- Otherwise: invalid path.

## Valid path

- Call `readLater.add(url)`.
- Do **not** switch app mode or focus the new item.
- Brief toast: “Saved to Read Later” with **Undo** (~7s).
- Undo calls `readLater.remove(id)` and **hard-deletes** the article (not archive).

## Invalid path

- Focus + raise the main window if needed.
- Modal shows attempted text (read-only), URL input prefilled with that text, Cancel / Add.
- Successful Add uses the same toast + Undo flow.

## API

- Existing: `readLater.add`.
- New: `readLater.remove({ id })` — delete Read Later article entirely; emit `article.removed` with `{ articleId }`.

## Out of scope

- Multi-URL batch drops.
- Non-macOS Dock parity.
- System share extensions / becoming the default browser.

## Success criteria

1. Window drop of a valid URL saves without mode switch; toast Undo removes it.
2. Window drop of invalid text opens the fix modal with the attempted string.
3. Dock/`open-url` delivery (when the OS provides a URL) focuses the app and follows the same valid/invalid paths.
