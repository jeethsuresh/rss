import type { Feed, Folder } from "@rss-reader/shared";

export const FEED_DRAG_MIME = "application/x-rss-feed-id";

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

export function withFeedAssigned(folders: Folder[], folderId: string, feedId: string): Folder[] {
  return folders.map((folder) => {
    if (folder.id !== folderId) return folder;
    const ids = folderFeedIds(folder);
    if (ids.includes(feedId)) return folder;
    return { ...folder, feedIds: [...ids, feedId] };
  });
}

export function withFeedUnassigned(folders: Folder[], folderId: string, feedId: string): Folder[] {
  return folders.map((folder) => {
    if (folder.id !== folderId) return folder;
    return { ...folder, feedIds: folderFeedIds(folder).filter((id) => id !== feedId) };
  });
}

export function isFeedDragTypes(types: ArrayLike<string>): boolean {
  return Array.from(types).includes(FEED_DRAG_MIME);
}

export function feedIdFromDropData(getData: (type: string) => string): string {
  return getData(FEED_DRAG_MIME).trim();
}
