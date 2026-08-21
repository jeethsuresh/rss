import { describe, expect, test } from "bun:test";
import type { Article, Story } from "@rss-reader/shared";
import {
  adjacentStoryListRow,
  memberArticle,
  storyListRowKey,
  storyListRows,
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

  test("memberArticle looks up expanded members", () => {
    const a = article({ id: "a" });
    const cluster = story({ id: "s1", articles: [a] });
    expect(memberArticle(cluster, "a")?.id).toBe("a");
    expect(memberArticle(cluster, "missing")).toBeNull();
  });

  test("upsertStoryInPlace patches flags without moving the row", () => {
    const a = story({ id: "a" });
    const b = story({ id: "b" });
    const next = upsertStoryInPlace([a, b], { ...b, isRead: true });
    expect(next.map((s) => s.id)).toEqual(["a", "b"]);
    expect(next[1].isRead).toBe(true);
  });
});
