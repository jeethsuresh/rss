# Settings Page + AI Triage Design

**Date:** 2026-08-20  
**Status:** Approved  

## Decisions

- Per-article priority: `none | low | medium | high`
- Meta-stories clustered in list + expandable detail panel
- Sidebar **Stories** filter shows only meta-stories
- Cascade read/unread/star from story → all member articles
- Scan: manual 24h/7d + auto on newly fetched articles
- Import/export: one feed URL per line (`#` comments OK)
- LM Studio OpenAI-compatible API; default `http://127.0.0.1:1234/v1`
- Go backend owns AI client, tools, and persistence

## Architecture

Go calls LM Studio `chat/completions` with tool `search_articles`. Results write `articles.priority` and `stories` / `story_articles`. React Settings page configures URL and triggers scans; Stories UI renders groups.

## Implementation order

1. Migrations + domain
2. Settings page + general settings
3. Feed table + import/export
4. AI client + worker + IPC
5. Stories UI + cascade
6. Auto-enqueue on new articles
