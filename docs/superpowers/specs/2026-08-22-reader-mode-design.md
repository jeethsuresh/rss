# Article reader mode

**Date:** 2026-08-22  
**Status:** Approved  

## Goal

Add a **Reader** content tab for RSS and Read Later that extracts the article from saved page HTML (Safari/Firefox-style) and shows it in the app’s normal reader column. Light and dark follow Settings → theme.

## Chrome

Third tab, not default:

- RSS: **Feed | Reader | Full page**
- Read Later: **Live | Reader | Saved crawl**

Opening an article still starts on Feed / Live. Changing the selected article resets to Feed / Live (existing `activeId` effect). No new setting.

Reader is never full-bleed. RSS shows the pane `h1` (app title) on Feed and Reader. Read Later shows that `h1` on Reader only. Full page / Live / Saved crawl stay `PageFrame` iframes.

## Extraction

Renderer-only helper `extractReaderArticle(html, url)` using `@mozilla/readability`. No Go, IPC, or SQLite changes. Reader does not fetch the network. Live fetch still happens only on the Live tab.

**HTML source, in order:**

1. `crawledContent` if non-empty  
2. else `liveContent` if non-empty  
3. else RSS `rssContent || content || summary`

Parse to a `Document` (`DOMParser` in Electron; `linkedom` in tests), run Readability, return `{ title, byline, contentHtml }` or `null`. Sanitize `contentHtml` with `sanitizeArticleHtml` (no scripts/iframes; images allowed).

Show `article.title` as the `h1`. If Readability’s byline is present, show it under the title. Do not render a second extracted title.

## Layout and theme

Reuse `.reader-body` (narrow column, existing type). Byline, figures, and captions use `var(--ink)`, `var(--muted)`, `var(--bg1)`, `var(--accent)` so `system` / `light` / `dark` apply automatically. No reader-only palette, font-size slider, or width control.

## Empty / failure states

| Situation | Reader tab |
|---|---|
| No source HTML and crawl pending | “Crawl in progress…” |
| No source HTML otherwise | “No page to extract yet.” + Recrawl |
| Readability returns null | Sanitized source HTML in Reader layout |
| Nothing usable after sanitize | “Couldn’t extract an article.” + Recrawl |

## Out of scope

Font size / margins, persisted `readerHtml`, auto-switching to Reader, TTS, paywall bypass, running page JS in Reader.

## Architecture

- `apps/desktop/renderer/lib/readerMode.ts` — source pick + extract  
- `apps/desktop/renderer/components/ReaderBody.tsx` — shared pane body  
- `App.tsx` + `ReadLaterView.tsx` — `ContentTab` includes `"reader"`  
- `global.css` — byline/figure rules on tokens only  

Go still owns SQLite/RSS/crawl. React still only talks via `window.rss`.

## Testing

- Fixture: extract main article text, drop nav/newsletter chrome  
- Empty HTML → `null`  
- Source order: crawled > live > RSS  
- Existing html/PageFrame tests still pass  
