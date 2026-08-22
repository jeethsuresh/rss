# Deterministic meta-stories

**Date:** 2026-08-22  
**Status:** Approved  
**Approach:** Incremental nearest-neighbour (A)

## Goal

When AI triage is off, RSS articles still cluster into meta-stories. The grouper uses RSS text only (never crawled HTML), drops the 100 most common English words, up-weights proper nouns, and joins via nearest-neighbour scores. Thumbs on members adjust per-token weights for later scoring. AI triage, when on, runs after this grouper and may override membership; it can also query the deterministic suggestion. Deterministic titles are `(N) {centroid article title}`.

## Ownership

- An article belongs to at most one story (existing `story_id`).
- After a successful RSS fetch that inserts articles, the deterministic grouper runs on those newly discovered RSS articles (not Read Later).
- If AI triage is on, the existing AI worker still runs afterward and may `create` / `join` as today, moving members if it disagrees.
- Turning AI off does **not** dissolve AI stories. New articles only join via the deterministic path until AI is on again.
- Stories record `source`: `deterministic` | `ai`. Any successful AI `create`/`join` sets `source` to `ai`.
- AI enqueue is skipped when AI triage is disabled (clustering still runs).

## Text and tokens

- Input: `rssContent`, else `summary`, plus the title. Never `crawledContent` / `liveContent`.
- Strip HTML. Split on non-letters. Lowercase for matching.
- Drop a fixed list of the 100 most common English words.
- **Title tokens:** included after stopword filter, never marked proper-noun (base weight 1).
- **Body proper nouns:** leftover tokens whose original form had an uppercase first letter. Base weight **3**; other leftover body tokens weight **1**.
- Learned multiplier per token: `1 + 0.25 * (upCount - downCount)`, clamped to `[0.1, 4]`.
- Article vector: sparse map of token → sum of (baseWeight × learnedMultiplier).

## Similarity and grouping

- Score = cosine similarity of the two vectors (0 if either vector is empty).
- Candidates for a new article: other non–Read Later RSS articles whose published/discovered time is within the last **7 days**, plus every current member of any story whose newest member is ≤ **14 days** old.
- Also score against each eligible **story centroid** (mean of member vectors).
- Take the best neighbour (article or centroid). Skip the article itself. Treat membership in an excluded story as ungrouped (used after thumbs-down).
- **Join** an existing story if the best match is that story (or a member of it) and score ≥ **0.50**. If the story’s newest member is older than **72 hours**, require score ≥ **0.70** instead. The 72h clock uses the newest member’s published/discovered time, so ongoing coverage stays open.
- **Create** a story if the best match is an ungrouped article, score ≥ **0.50**, and both are eligible RSS items. Title/summary as below.
- Otherwise leave the article ungrouped (singleton stories stay hidden).
- Process new articles in published/discovered order so same-fetch items can pair.
- Skip Read Later everywhere, same as AI clustering.
- Do not pull an article out of an existing (non-excluded) story to form a new pair.

## Titles and summaries

- Deterministic title: `(N) ` + title of the member whose vector is closest to the cluster centroid. `N` is the eligible (non–Read Later) member count. Recalculate whenever the clusterer changes membership.
- Deterministic summary: RSS summary/snippet of that same centroid article (truncated ~280 chars). No LLM text.
- AI-created/overridden stories keep the model’s title and summary.

## Thumbs

- On each expanded member row: thumbs up / down. Same control for AI and deterministic stories.
- On the story reader: story-level thumbs (persisted for future reports; does not affect grouping).
- **Up:** keep membership; increment `upCount` on tokens that appear in both this article and the rest of the cluster; store the vote.
- **Down:** snapshot current member IDs; increment `downCount` on overlapping tokens; remove the article from this story; retitle the leftover cluster if it still has ≥2 members; re-run NN for the removed article **excluding that story**; re-rank the leftover cluster against other stories (merge remaining members into the best matching story if it clears the join/72h threshold; if only one member remains, re-rank that article the same way).
- **Undo:** clicking the active thumb clears the vote. For up, reverse token counts only. For down, reverse token counts and `SetMembers` back to the snapshot (pulling articles back from any story they joined during re-rank). Switching up↔down undoes the old vote then applies the new one.

## Persistence

- `stories.source` (`deterministic` | `ai`; existing rows default `ai`)
- `story_token_weights(token, up_count, down_count)`
- `story_article_votes(story_id, article_id, vote, member_snapshot, created_at)` with `vote` in `up` | `down`
- `story_votes(story_id, vote, created_at)` for story-level thumbs
- Existing `stories` / `story_articles` remain the membership tables

## AI tool

New tool `suggest_meta_story` (implied current triage article):

- Runs the same scorer the fetch path uses (query only; no writes).
- Returns `{ action, storyId?, title, memberIds, score, threshold, tokens }`.
- System prompt: consult this before `create`/`join`; you may still override.

## UI / IPC

- `stories.voteArticle` `{ storyId, articleId, vote: "up"|"down"|"none" }` → updated `Story`
- `stories.voteStory` `{ storyId, vote: "up"|"down"|"none" }` → updated `Story`
- Story payload includes `source`, `vote` (story-level), and `articleVotes` map.
- Stories empty state no longer says clustering requires AI triage.
- No new Settings toggle: grouping always runs on fetch.

## Tests

- Tokenization: stopwords gone; title “Biden” is not a proper noun; body “Biden” is.
- Cosine join/create thresholds; 72h barrier blocks moderate scores and allows 0.70.
- Thumbs-down removes, re-ranks article and leftover cluster, undo restores snapshot; token weights move.
- AI tool is read-only.
- Read Later never clustered.

## Out of scope

- Using thumbs in reports/UI charts
- Full-page crawl text
- Rebuilding historical AI clusters when toggling AI
- Multi-story membership
