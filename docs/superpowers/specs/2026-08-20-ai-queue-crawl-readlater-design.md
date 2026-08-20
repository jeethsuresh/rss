# Persistent AI queue, crawl, Read Later

**Date:** 2026-08-20  
**Status:** Approved  

## Addendum
- `mark_crawl_unreliable` recorded per article and aggregated **per feed**
- Settings → Feeds shows **bad crawl %** per feed (`failed crawls / crawled attempts`)

## Rest
See prior design: persistent `ai_queue` + `ai_logs`, dual content tabs (Feed/Live vs Full page), Read Later sidebar (fetch on save + AI re-crawl), AI prefers crawled text unless unreliable.
