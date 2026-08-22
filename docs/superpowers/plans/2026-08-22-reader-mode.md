# Article Reader Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Reader content tab that extracts article HTML with Mozilla Readability and shows it in the existing light/dark reader column for RSS and Read Later.

**Architecture:** Renderer-only. Pick crawled → live → RSS HTML, run `@mozilla/readability`, sanitize, render in `ReaderBody`. No backend/schema changes. `ContentTab` gains `"reader"`; iframe tabs stay `PageFrame`.

**Tech Stack:** Electron renderer (React), `@mozilla/readability`, `linkedom` (Bun tests / DOMParser fallback), existing `sanitizeArticleHtml`, CSS variables.

## Global Constraints

- React uses only `window.rss` (`ReaderBackend`); no localhost HTTP IPC.
- Go owns SQLite, RSS, scheduler, crawl; this feature does not change Go.
- Do not implement `cmd/server`.
- Theme via existing `data-theme` + CSS variables (`system` / `light` / `dark`).
- No merge to `main` (another agent is using it).

## File structure

- Create: `apps/desktop/renderer/lib/readerMode.ts` — `readerSourceHtml`, `extractReaderArticle`, `ContentTab`
- Create: `apps/desktop/renderer/lib/readerMode.test.ts`
- Create: `apps/desktop/renderer/components/ReaderBody.tsx`
- Modify: `apps/desktop/package.json` — `@mozilla/readability`, `linkedom`
- Modify: `apps/desktop/renderer/App.tsx` — third tab, Reader body, fullbleed/h1 rules
- Modify: `apps/desktop/renderer/views/ReadLaterView.tsx` — same tabs
- Modify: `apps/desktop/renderer/styles/global.css` — byline/figure
- Create: `docs/superpowers/specs/2026-08-22-reader-mode-design.md` (already written)

---

### Task 1: Source pick + Readability extract

**Files:**
- Create: `apps/desktop/renderer/lib/readerMode.ts`
- Create: `apps/desktop/renderer/lib/readerMode.test.ts`
- Modify: `apps/desktop/package.json`

**Interfaces:**
- Produces:
  - `export type ContentTab = "primary" | "reader" | "secondary"`
  - `export type ReaderArticle = { title: string; byline: string; contentHtml: string }`
  - `export function readerSourceHtml(article: Pick<Article, "crawledContent" | "liveContent" | "rssContent" | "content" | "summary">): string | null`
  - `export function extractReaderArticle(html: string, pageUrl: string): ReaderArticle | null`

- [ ] **Step 1: Add dependencies**

Run: `cd apps/desktop && bun add @mozilla/readability linkedom && bun add -d @types/mozilla-readability`
Expected: packages in `apps/desktop/package.json` dependencies (`@types/mozilla-readability` only if the readability package has no bundled types).

- [ ] **Step 2: Write failing tests**

`apps/desktop/renderer/lib/readerMode.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { extractReaderArticle, readerSourceHtml } from "./readerMode";

const noisyPage = `<!doctype html><html><body>
<nav>Home Sports Weather</nav>
<article>
  <h1>Rural broadband funding</h1>
  <p>By Jane Doe</p>
  <p>${"The province announced new funding for rural broadband so towns can finally get reliable service. ".repeat(8)}</p>
  <p>${"Officials said construction starts next spring and households can apply for subsidies. ".repeat(8)}</p>
</article>
<aside>Subscribe to our newsletter<script>fetch("https://subscriptions.cbc.ca/api/newsletter/get_active_newsletters?source=")</script></aside>
</body></html>`;

describe("readerSourceHtml", () => {
  test("prefers crawled, then live, then RSS fields", () => {
    expect(
      readerSourceHtml({
        crawledContent: "<p>crawl</p>",
        liveContent: "<p>live</p>",
        rssContent: "<p>rss</p>",
        content: "<p>content</p>",
        summary: "sum",
      }),
    ).toBe("<p>crawl</p>");
    expect(
      readerSourceHtml({
        crawledContent: "  ",
        liveContent: "<p>live</p>",
        rssContent: "<p>rss</p>",
        content: "",
        summary: "",
      }),
    ).toBe("<p>live</p>");
    expect(
      readerSourceHtml({
        crawledContent: "",
        liveContent: "",
        rssContent: "",
        content: "<p>content</p>",
        summary: "sum",
      }),
    ).toBe("<p>content</p>");
    expect(
      readerSourceHtml({
        crawledContent: "",
        liveContent: "",
        rssContent: "",
        content: "",
        summary: "",
      }),
    ).toBeNull();
  });
});

describe("extractReaderArticle", () => {
  test("returns null for empty html", () => {
    expect(extractReaderArticle("", "https://www.cbc.ca/news/x")).toBeNull();
  });

  test("extracts article text and drops newsletter chrome", () => {
    const result = extractReaderArticle(noisyPage, "https://www.cbc.ca/news/x");
    expect(result).not.toBeNull();
    expect(result!.contentHtml.toLowerCase()).not.toContain("<script");
    expect(result!.contentHtml).toContain("rural broadband");
    expect(result!.contentHtml.toLowerCase()).not.toContain("get_active_newsletters");
  });
});
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `bun test apps/desktop/renderer/lib/readerMode.test.ts`
Expected: FAIL — cannot find module `./readerMode`

- [ ] **Step 4: Implement `readerMode.ts`**

Use `DOMParser` when defined, else `linkedom` `parseHTML`. Clone is not required if we parse a fresh document each time. Set a `<base href>` from `pageUrl` before `new Readability(doc).parse()`. Sanitize `content` with `sanitizeArticleHtml`. Return null when parse yields no content HTML.

- [ ] **Step 5: Run tests to verify they pass**

Run: `bun test apps/desktop/renderer/lib/readerMode.test.ts`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/desktop/package.json apps/desktop/bun.lock bun.lock apps/desktop/renderer/lib/readerMode.ts apps/desktop/renderer/lib/readerMode.test.ts
git commit -m "feat: extract reader-mode HTML with Readability"
```

---

### Task 2: ReaderBody + CSS

**Files:**
- Create: `apps/desktop/renderer/components/ReaderBody.tsx`
- Modify: `apps/desktop/renderer/styles/global.css`

**Interfaces:**
- Consumes: `readerSourceHtml`, `extractReaderArticle` from Task 1
- Produces: `ReaderBody({ article, contentBusy, onRecrawl })`

- [ ] **Step 1: Implement ReaderBody**

If no source HTML: pending crawl → “Crawl in progress…”; else “No page to extract yet.” plus Recrawl button.

If extract returns null: `sanitizeArticleHtml(source)` as body.

If sanitized/extracted body `stripHtml` is empty: “Couldn’t extract an article.” plus Recrawl.

Otherwise: optional `.reader-byline` then `.reader-body` with `dangerouslySetInnerHTML`.

Title `h1` stays in the parent pane (App/Read Later), not inside ReaderBody.

- [ ] **Step 2: CSS**

```css
.reader-byline {
  color: var(--muted);
  font-size: 0.95rem;
  margin: 0 0 1rem;
}
.reader-body figure { margin: 1.25rem 0; }
.reader-body figcaption { color: var(--muted); font-size: 0.9rem; }
```

Reader tab must not use `.reader-fullbleed`.

- [ ] **Step 3: Commit**

```bash
git add apps/desktop/renderer/components/ReaderBody.tsx apps/desktop/renderer/styles/global.css
git commit -m "feat: add ReaderBody layout using theme tokens"
```

---

### Task 3: Wire RSS and Read Later tabs

**Files:**
- Modify: `apps/desktop/renderer/App.tsx`
- Modify: `apps/desktop/renderer/views/ReadLaterView.tsx`

- [ ] **Step 1: Shared ContentTab**

Replace local `"primary" | "secondary"` with `ContentTab` from `readerMode.ts`. Add a Reader button between the two existing tabs.

- [ ] **Step 2: RSS App.tsx**

`handleContentTab("reader")` only sets state (no live fetch).
`renderContentBody`: if `contentTab === "reader"`, return `<ReaderBody … onRecrawl={recrawlActive} />`.
`h1` when `(contentTab === "primary" || contentTab === "reader") && !active.isReadLater`.
Fullbleed when `contentTab === "secondary" || (contentTab === "primary" && active.isReadLater)`.
Recrawl button also when `contentTab === "reader"`.
Keep `useEffect` that resets tab to `"primary"` on `activeId`.

- [ ] **Step 3: ReadLaterView.tsx**

Same three tabs. Live fetch effect stays `contentTab === "primary"`.
`renderBody`: reader → `ReaderBody`; primary/secondary unchanged PageFrame.
Pane class: `reader` plus `reader-fullbleed` only for primary/secondary.
Show `<h1>` on reader tab.
Recrawl on reader and secondary.

- [ ] **Step 4: Exhaustive tab handling**

Any switch on `ContentTab` uses a `never` default.

- [ ] **Step 5: Run tests + build**

Run: `bun test && cd backend && go test ./...`
Run: `bun run build`
Expected: all tests pass, build exit 0.

- [ ] **Step 6: Commit**

```bash
git add apps/desktop/renderer/App.tsx apps/desktop/renderer/views/ReadLaterView.tsx
git commit -m "feat: add Reader tab for RSS and Read Later"
```

---

### Task 4: Do not merge

Leave `feat/reader-mode` unmerged. Do not merge to `main`.
