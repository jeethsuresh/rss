import { describe, expect, test } from "bun:test";
import type { Feed, Folder } from "@rss-reader/shared";
import {
  FEED_DRAG_MIME,
  feedIdFromDropData,
  feedsForFolder,
  feedsNotInFolder,
  folderFeedIds,
  isFeedDragTypes,
  isFolderCollapsed,
  mergeFolderMemberships,
  normalizeFolderName,
  toggleCollapsedFolder,
  unassignedFeeds,
  withFeedAssigned,
  withFeedUnassigned,
  withFolderExpanded,
  folderUnreadCount,
  folderNameWithUnread,
} from "./folders";

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

  test("keeps an assigned feed visible even if list omitted feedIds", () => {
    const empty = folder({ id: "news", name: "News", feedIds: [] });
    const next = withFeedAssigned([empty], "news", "hn");
    expect(feedsForFolder([nyt, bbc, hn], next[0]!).map((f) => f.id)).toEqual(["hn"]);
  });

  test("does not duplicate an already assigned feed", () => {
    const next = withFeedAssigned([news], "news", "nyt");
    expect(folderFeedIds(next[0]!)).toEqual(["nyt", "bbc"]);
  });

  test("removes a feed from a folder", () => {
    const next = withFeedUnassigned([news], "news", "nyt");
    expect(feedsForFolder([nyt, bbc, hn], next[0]!).map((f) => f.id)).toEqual(["bbc"]);
  });

  test("keeps the previous feed when a reload omits feedIds and another feed is assigned", () => {
    const prev = [folder({ id: "news", name: "News", feedIds: ["nyt"] })];
    const listed = [folder({ id: "news", name: "News", feedIds: [] })];
    const next = withFeedAssigned(mergeFolderMemberships(listed, prev), "news", "bbc");
    expect(folderFeedIds(next[0]!)).toEqual(["nyt", "bbc"]);
  });

  test("unions memberships from the server list with local folder state", () => {
    const prev = [folder({ id: "news", name: "News", feedIds: ["nyt"] })];
    const listed = [folder({ id: "news", name: "News", feedIds: ["bbc"] })];
    expect(folderFeedIds(mergeFolderMemberships(listed, prev)[0]!)).toEqual(["bbc", "nyt"]);
  });
});

describe("feed drag payload", () => {
  test("recognizes the internal feed mime type so window URL-drop does not steal it", () => {
    expect(isFeedDragTypes(["text/plain", FEED_DRAG_MIME])).toBe(true);
    expect(isFeedDragTypes(["text/plain", "text/uri-list"])).toBe(false);
  });

  test("reads the feed id from the internal mime type, not text/plain", () => {
    const id = feedIdFromDropData((type) => (type === FEED_DRAG_MIME ? "feed-1" : "https://example.com"));
    expect(id).toBe("feed-1");
  });
});

describe("folder collapse", () => {
  test("folders start expanded", () => {
    expect(isFolderCollapsed(new Set(), "news")).toBe(false);
  });

  test("toggle collapse then expand", () => {
    const collapsed = toggleCollapsedFolder(new Set(), "news");
    expect(isFolderCollapsed(collapsed, "news")).toBe(true);
    expect(isFolderCollapsed(toggleCollapsedFolder(collapsed, "news"), "news")).toBe(false);
  });

  test("selecting a folder expands it", () => {
    const collapsed = toggleCollapsedFolder(new Set(), "news");
    expect(isFolderCollapsed(withFolderExpanded(collapsed, "news"), "news")).toBe(false);
  });
});

describe("folder unread total", () => {
  test("sums unread across assigned feeds and ignores read later", () => {
    const nyt = feed({ id: "nyt", unreadCount: 3 });
    const bbc = feed({ id: "bbc", unreadCount: 5 });
    const hn = feed({ id: "hn", unreadCount: 9 });
    const later = feed({ id: "rl", unreadCount: 4, isReadLater: true });
    const news = folder({ id: "news", name: "News", feedIds: ["nyt", "bbc", "rl"] });
    expect(folderUnreadCount([nyt, bbc, hn, later], news)).toBe(8);
  });

  test("hides the parenthetical when there are no unread items", () => {
    expect(folderNameWithUnread("News", 0)).toBe("News");
    expect(folderNameWithUnread("News", 8)).toBe("News (8)");
  });
});
