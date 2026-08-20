import type { Feed, Folder } from "@rss-reader/shared";

export function normalizeFolderName(name: string): string {
  return name.trim();
}

export function folderFeedIds(folder: Folder): string[] {
  return folder.feedIds ?? [];
}

export function feedsForFolder(feeds: Feed[], folder: Folder): Feed[] {
  const ids = new Set(folderFeedIds(folder));
  return feeds.filter((feed) => !feed.isReadLater && ids.has(feed.id));
}

export function unassignedFeeds(feeds: Feed[], folders: Folder[]): Feed[] {
  const assigned = new Set(folders.flatMap(folderFeedIds));
  return feeds.filter((feed) => !feed.isReadLater && !assigned.has(feed.id));
}

export function feedsNotInFolder(feeds: Feed[], folder: Folder): Feed[] {
  const ids = new Set(folderFeedIds(folder));
  return feeds.filter((feed) => !feed.isReadLater && !ids.has(feed.id));
}
