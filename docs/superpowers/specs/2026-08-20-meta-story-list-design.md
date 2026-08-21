# Meta-story list & membership

**Date:** 2026-08-20  
**Status:** Approved

## Goal

A meta-story is a cluster of **two or more RSS articles**. The Stories list is where you pick the cluster or a member; the reader shows either the cluster summary or that member’s full article. Read Later items never belong in stories.

## Visibility

- `stories.list` returns only stories with **at least two non–Read Later members**.
- Existing 1-article stories stay in SQLite but are **hidden**.
- AI `create` with fewer than two eligible members is a no-op.

## Read Later exclusion

- Read Later articles are never story members.
- AI triage skips story clustering for Read Later items (priority may still be set).
- `AddMember` / `SetMembers` ignore Read Later IDs.
- `search_articles` omits Read Later hits.
- List/Get member counts and arrays exclude Read Later so old memberships do not appear.

## Stories list

- Story rows stay in the middle pane.
- Selecting a story expands its members under that row (accordion; other stories stay visible; only one story expanded).
- Member rows use Unread chrome (feed, time, title, snippet).
- Member articles are not listed in the reader pane.

## Reader

- **Story selected:** title, AI summary, story read/star. No member list.
- **Member selected:** the normal RSS article reader (same as Unread).

## Keyboard

- `j`/`k` walk visible rows: story headers, then expanded members.
- Landing on a story shows the summary; landing on a member opens that article.
- `r`/`u`/`s` apply to the story when a story row is selected, and to the article when a member is selected.

## Out of scope

- Deleting singleton stories from SQLite
- Changing the AI model beyond clustering rules above
