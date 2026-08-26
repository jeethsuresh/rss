import { describe, expect, test } from "bun:test";
import type { Article, Story } from "@rss-reader/shared";
import {
  adjacentMetaStory,
  adjacentStoryListRow,
  hasNoOtherUnreadStoryMembers,
  memberArticle,
  nextStoryVote,
  storyListRowKey,
  storyListRows,
  storiesInSnapshot,
  unreadStorySnapshotIds,
  unreadStoryCount,
  upsertStoryInPlace,
} from "./stories";

function article(partial: Partial<Article> & Pick<Article, "id">): Article {
  return {
    feedId: "feed",
    title: partial.id,
    url: `https://example.com/${partial.id}`,
    author: "",
    content: "",
    summary: `${partial.id} snippet`,
    rssContent: "",
    crawledContent: "",
    liveContent: "",
    crawlStatus: "none",
    crawlError: "",
    crawlUnreliable: false,
    publishedAt: "2026-08-20T12:00:00.000Z",
    updatedAt: null,
    externalId: partial.id,
    isRead: false,
    isStarred: false,
    isReadLater: false,
    priority: "none",
    discoveredAt: "2026-08-20T12:00:00.000Z",
    feedTitle: "News",
    ...partial,
  };
}

function story(partial: Partial<Story> & Pick<Story, "id">): Story {
  return {
    title: partial.id,
    summary: "cluster",
    isRead: false,
    isStarred: false,
    memberCount: partial.articles?.length ?? 0,
    createdAt: "2026-08-20T00:00:00.000Z",
    updatedAt: "2026-08-20T00:00:00.000Z",
    ...partial,
  };
}

describe("storyListRows", () => {
  test("expands only the selected story and skips read later members", () => {
    const a = article({ id: "a" });
    const b = article({ id: "b" });
    const rl = article({ id: "rl", isReadLater: true });
    const cluster = story({ id: "s1", articles: [a, rl, b], memberCount: 2 });
    const other = story({ id: "s2", memberCount: 2 });
    const rows = storyListRows([cluster, other], cluster);
    expect(rows).toEqual([
      { kind: "story", storyId: "s1" },
      { kind: "member", storyId: "s1", articleId: "a" },
      { kind: "member", storyId: "s1", articleId: "b" },
      { kind: "story", storyId: "s2" },
    ]);
  });

  test("adjacent row walks story headers then members", () => {
    const cluster = story({
      id: "s1",
      articles: [article({ id: "a" }), article({ id: "b" })],
    });
    const rows = storyListRows([cluster], cluster);
    const next = adjacentStoryListRow(rows, storyListRowKey({ kind: "story", storyId: "s1" }), 1);
    expect(next).toEqual({ kind: "member", storyId: "s1", articleId: "a" });
  });

  test("sorts expanded members reverse-chronologically", () => {
    const older = article({ id: "older", publishedAt: "2026-08-19T12:00:00.000Z" });
    const newer = article({ id: "newer", publishedAt: "2026-08-21T12:00:00.000Z" });
    const cluster = story({ id: "s1", articles: [older, newer] });
    expect(storyListRows([cluster], cluster)).toEqual([
      { kind: "story", storyId: "s1" },
      { kind: "member", storyId: "s1", articleId: "newer" },
      { kind: "member", storyId: "s1", articleId: "older" },
    ]);
  });

  test("moves between meta-story headers without stopping on members", () => {
    const first = story({ id: "s1", memberCount: 2 });
    const second = story({ id: "s2", memberCount: 2 });
    expect(adjacentMetaStory([first, second], "s1", 1)?.id).toBe("s2");
  });

  test("recognizes the final unread member even when read members follow it", () => {
    const current = article({ id: "current", isRead: false });
    const read = article({ id: "read", isRead: true });
    const cluster = story({ id: "s1", articles: [current, read] });
    expect(hasNoOtherUnreadStoryMembers(cluster, current.id)).toBe(true);
    expect(hasNoOtherUnreadStoryMembers({ ...cluster, articles: [{ ...read, isRead: false }, current] }, current.id)).toBe(false);
  });

  test("still advances after mark-on-open has marked the final member read", () => {
    const current = article({ id: "current", isRead: true });
    const cluster = story({
      id: "s1",
      articles: [current, article({ id: "older", isRead: true })],
    });
    expect(hasNoOtherUnreadStoryMembers(cluster, current.id)).toBe(true);
  });

  test("memberArticle looks up expanded members", () => {
    const a = article({ id: "a" });
    const cluster = story({ id: "s1", articles: [a] });
    expect(memberArticle(cluster, "a")?.id).toBe("a");
    expect(memberArticle(cluster, "missing")).toBeNull();
  });

  test("upsertStoryInPlace patches flags without moving the row", () => {
    const a = story({ id: "a", memberCount: 2 });
    const b = story({ id: "b", memberCount: 2 });
    const next = upsertStoryInPlace([a, b], { ...b, isRead: true });
    expect(next.map((s) => s.id)).toEqual(["a", "b"]);
    expect(next[1].isRead).toBe(true);
  });

  test("upsertStoryInPlace does not insert stories with fewer than two members", () => {
    const visible = story({ id: "keep", memberCount: 2 });
    const next = upsertStoryInPlace([visible], story({ id: "empty", memberCount: 0, title: "(3) leftover" }));
    expect(next.map((s) => s.id)).toEqual(["keep"]);
  });

  test("upsertStoryInPlace removes a story that dropped below two members", () => {
    const keep = story({ id: "keep", memberCount: 2 });
    const gone = story({ id: "gone", memberCount: 3, title: "(3) US Army" });
    const next = upsertStoryInPlace([keep, gone], { ...gone, memberCount: 0 });
    expect(next.map((s) => s.id)).toEqual(["keep"]);
  });

  test("storyListRows skips stories with fewer than two members", () => {
    const empty = story({ id: "empty", memberCount: 0, title: "(3) leftover" });
    const one = story({ id: "one", memberCount: 1 });
    const two = story({ id: "two", memberCount: 2 });
    const rows = storyListRows([empty, one, two], empty);
    expect(rows).toEqual([{ kind: "story", storyId: "two" }]);
  });
});

describe("nextStoryVote", () => {
  test("toggles off when clicking the active vote", () => {
    expect(nextStoryVote("up", "up")).toBe("none");
    expect(nextStoryVote(undefined, "down")).toBe("down");
    expect(nextStoryVote("up", "down")).toBe("down");
  });
});

describe("unreadStoryCount", () => {
  test("counts only listable unread meta-stories", () => {
    expect(
      unreadStoryCount([
        story({ id: "unread", memberCount: 2 }),
        story({ id: "read", memberCount: 3, isRead: true }),
        story({ id: "singleton", memberCount: 1 }),
      ]),
    ).toBe(1);
  });
});

describe("unread story snapshots", () => {
  test("keeps the captured stories after they become read", () => {
    const first = story({ id: "first", memberCount: 2 });
    const last = story({ id: "last", memberCount: 3 });
    const snapshot = unreadStorySnapshotIds([first, last]);

    expect(storiesInSnapshot([{ ...first, isRead: true }, { ...last, isRead: true }], snapshot)).toEqual([
      { ...first, isRead: true },
      { ...last, isRead: true },
    ]);
  });

  test("does not add newly unread stories to an existing snapshot", () => {
    const captured = story({ id: "captured", memberCount: 2 });
    const later = story({ id: "later", memberCount: 2 });
    const snapshot = unreadStorySnapshotIds([captured]);

    expect(storiesInSnapshot([later, captured], snapshot).map((item) => item.id)).toEqual(["captured"]);
  });
});
