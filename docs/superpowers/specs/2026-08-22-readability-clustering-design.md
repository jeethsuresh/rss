# Readability extracts for clustering

**Date:** 2026-08-22  
**Status:** Approved  
**Approach:** Persist extract, delay grouping until extract/crawl is terminal (A)

## Goal

Every article gets one backend Readability pass (`github.com/mackee/go-readability`) on crawled HTML, then one frontend `@mozilla/readability` pass if Go failed. Successful extracts are the clustering body. Failures use RSS text. No extract retries after a completed failure. Crawl retries after app close or a connectivity error; it does not retry 4xx, 5xx, or an invalid page that actually came back.

## Crawl retry policy

| Outcome | Retry? |
|---|---|
| `none` / `pending` (including app quit mid-fetch) | Yes |
| No HTTP response: timeout, DNS, connection reset, TLS, offline | Yes |
| HTTP **4xx** or **5xx** | No |
| Response body empty, not HTML, or otherwise invalid | No |
| User **Recrawl** | Yes — fresh crawl + extract cycle |

Persist `crawlRetryable`. `ListNeedingCrawl` includes `none`/`pending` and `failed` + retryable + empty crawled body + not unreliable.

## Extract

Columns: `reader_content`, `extract_status` (`none` \| `js` \| `ok` \| `failed`), `extract_source` (`go` \| `js` \| `""`).

After a successful crawl:

1. Go Readability once. Non-empty strip-tags content → `ok` / `go`.
2. Else → `js`. Emit `article.updated` with `extractStatus: "js"`.
3. Renderer drains `js`: `extractReaderArticle(crawledContent, url)` once. `articles.setExtract` with HTML or `""`.
4. App quit while `js`: retry JS on next launch (local HTML, no re-fetch).
5. Both failed → `failed`, stop.

Permanent crawl fail: skip extractors, `extract_status=failed`, cluster on RSS.

Read Later: same extract; still not clustered.

## Clustering

- Body: strip-tags(`readerContent`) when `extract_status=ok`; else `rssContent` else `summary`.
- Title tokens: article title.
- Never use raw `crawledContent` / `liveContent` as the bag-of-words.
- Do not `ClusterNew` immediately after RSS insert.
- After extract `ok`/`failed` or permanent crawl fail: `clusterOne` if ungrouped and not Read Later.
- Already in a story: store extract, do not move until Re-index.
- Re-index and Split use extract-or-RSS.
- Backfill: existing `crawl_status=ok` + `extract_status=none` + crawled HTML → Go (no re-fetch), then JS if needed, then `clusterOne` if still ungrouped.

## IPC / UI

- `articles.setExtract` `{ articleId, html }`
- `articles.pendingExtract` → `{ articleIds: string[] }` (`extract_status=js`)
- Article payload: `extractStatus`, `extractSource`, `readerContent`, `crawlRetryable`
- Renderer drains `js` on startup and `article.updated`
- Reader tab prefers stored `readerContent` when `ok`

## Tests

- Go extract success → clustering uses extract, not RSS
- Go fail + JS success → `ok`/`js`
- Both fail → RSS fallback; no second pass
- 404/500/invalid HTML → not retryable; RSS cluster
- Timeout / `pending` at startup → crawl retried
- `js` at startup → JS retried, no re-fetch
- Read Later extracted, not clustered

## Out of scope

- Paywall bypass / running page JS
- Moving existing story members when a late extract arrives
- Threshold changes
- Using extracted title as clustering title
