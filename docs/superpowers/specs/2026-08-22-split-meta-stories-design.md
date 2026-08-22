# Split meta-stories

**Date:** 2026-08-22  
**Status:** Approved  
**Approach:** Connected components at join threshold (A)

## Goal

A **Split** control on the selected meta-story dissolves that cluster, regroups **only its members** into tighter stories or hidden singletons, and down-weights tokens that overlapped *across* the resulting groups so a later refresh does not glue them back together. New RSS grouping uses join threshold **0.50** instead of **0.35**.

## Join threshold

- `JoinThreshold` is **0.50** (create, join, leftover merge, re-index, AI `suggest_meta_story`).
- `StaleJoinThreshold` stays **0.70** when the story’s newest member is older than **72 hours**.
- Existing clusters are not rebuilt until Split, thumbs-down leftover merge, or **Re-index all stories**.

## Split algorithm

- Available on every listed meta-story (`deterministic` and `ai`).
- Snapshot eligible (non–Read Later) members. Read Later stays excluded.
- Build article vectors with the current tokenizer and learned token weights (RSS text only).
- Undirected graph: an edge exists when pairwise cosine ≥ **0.50**. The 72h stale rule does **not** apply inside Split.
- Connected components:
  - **≥2 members** → new `source=deterministic` story with centroid title `(N) {centroid article title}` and that member’s summary.
  - **1 member** → ungrouped (hidden).
- Members never join some other existing story during Split.
- If the graph is still one component, Split is a **no-op**: membership and token weights unchanged; return the original story id.
- When the graph breaks, clear membership on the original story (row may remain in SQLite for thumbs-down undo snapshots; it stays hidden at `<2` members). Create new stories only for components of size ≥2.

## Bias re-weight

- After components are known (and only when there are **two or more** components), increment `downCount` on tokens that appear in **both** of any pair of component mean vectors.
- Do **not** down-weight tokens that only overlap inside a surviving component.
- Learned multiplier stays `1 + 0.25 * (upCount - downCount)`, clamped to `[0.1, 4]`.
- Split is not a member thumbs vote: no per-article vote snapshot and **no undo**.

## UI / IPC

- Story reader actions (next to Mark read / Star / thumbs): **Split**.
- `stories.split` `{ storyId }` → `{ storyIds: string[] }` — new visible stories, largest first then stable id order. Empty when every leftover is a singleton. No-op returns `{ storyIds: [originalId] }`.
- Desktop reloads the stories list and selects `storyIds[0]` when present; otherwise selection follows the usual list fallback.
- Settings → AI log: `split story: N → M groups` when membership actually changes (`N` eligible members, `M` connected components).
- No confirmation dialog.

## Tests

- Join threshold is 0.50: cosine 0.49 does not create/join; 0.50 does.
- Mixed cluster (A–A′ high, B–B′ high, A–B low) splits into two stories; tokens shared across the cut get `downCount+1`; intra-group-only tokens do not.
- A 2-member story whose pair scores `< 0.50` becomes unlistable.
- Split is a no-op (same members, no weight change) when all members still form one component.
- Read Later members are ignored.

## Out of scope

- Undo Split
- Split control on list headers
- Auto re-index of the whole library
- Changing the 72h / 0.70 stale rule
- Treating Split as a story-level thumbs vote
