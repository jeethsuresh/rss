import { describe, expect, test } from "bun:test";
import type { Feed, Folder } from "@rss-reader/shared";
import { feedsForFolder, feedsNotInFolder, normalizeFolderName, unassignedFeeds } from "./folders";

function feed(partial: Partial<Feed> & Pick<Feed, "id">): Feed {
  return {
    url: `https://example.com/${partial.id}.xml`,
    title: partial.id,
    description: "",
    siteUrl: "",
    iconUrl: "",
    lastSuccessAt: null,
    lastAttemptAt: null,
    lastError: null,
    etag: "",
    lastModified: "",
    pollIntervalSeconds: 3600,
    enabled: true,
    unreadCount: 0,
    isReadLater: false,
    crawlAttempts: 0,
    crawlFailures: 0,
    badCrawlPercent: 0,
    createdAt: "2026-08-20T00:00:00.000Z",
    updatedAt: "2026-08-20T00:00:00.000Z",
    ...partial,
  };
}

function folder(partial: Partial<Folder> & Pick<Folder, "id" | "name">): Folder {
  return {
    createdAt: "2026-08-20T00:00:00.000Z",
    feedIds: [],
    ...partial,
  };
}

describe("normalizeFolderName", () => {
  test("trims whitespace", () => {
    expect(normalizeFolderName("  News  ")).toBe("News");
  });

  test("rejects blank names", () => {
    expect(normalizeFolderName("   ")).toBe("");
  });
});

describe("folder feed grouping", () => {
  const nyt = feed({ id: "nyt", title: "NYT" });
  const bbc = feed({ id: "bbc", title: "BBC" });
  const hn = feed({ id: "hn", title: "HN" });
  const later = feed({ id: "rl", title: "Read Later", isReadLater: true });
  const news = folder({ id: "news", name: "News", feedIds: ["nyt", "bbc"] });

  test("lists assigned feeds under a folder", () => {
    expect(feedsForFolder([nyt, bbc, hn, later], news).map((f) => f.id)).toEqual(["nyt", "bbc"]);
  });

  test("keeps unassigned feeds out of folders", () => {
    expect(unassignedFeeds([nyt, bbc, hn, later], [news]).map((f) => f.id)).toEqual(["hn"]);
  });

  test("offers remaining feeds when adding to a folder", () => {
    expect(feedsNotInFolder([nyt, bbc, hn, later], news).map((f) => f.id)).toEqual(["hn"]);
  });
});
