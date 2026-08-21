import type { Feed, Folder } from "@rss-reader/shared";

export const FEED_DRAG_MIME = "application/x-rss-feed-id";

export function normalizeFolderName(name: string): string {
  return name.trim();
}

export function folderFeedIds(folder: Folder): string[] {
  return coerceFeedIds(folder.feedIds);
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

export function coerceFeedIds(value: unknown): string[] {
  if (Array.isArray(value)) {
    return [...new Set(value.map((id) => String(id).trim()).filter(Boolean))];
  }
  if (typeof value === "string" && value.trim()) {
    return [value.trim()];
  }
  return [];
}

export function mergeFolderMemberships(listed: Folder[], previous: Folder[]): Folder[] {
  const prevById = new Map(previous.map((folder) => [folder.id, folder]));
  return listed.map((folder) => {
    const ids = new Set([
      ...coerceFeedIds(folder.feedIds),
      ...folderFeedIds(prevById.get(folder.id) ?? { ...folder, feedIds: [] }),
    ]);
    return { ...folder, feedIds: [...ids] };
  });
}

export function isFeedDragTypes(types: ArrayLike<string>): boolean {
  return Array.from(types).includes(FEED_DRAG_MIME);
}

export function feedIdFromDropData(getData: (type: string) => string): string {
  return getData(FEED_DRAG_MIME).trim();
}

export function isFolderCollapsed(collapsedIds: ReadonlySet<string>, folderId: string): boolean {
  return collapsedIds.has(folderId);
}

export function toggleCollapsedFolder(
  collapsedIds: ReadonlySet<string>,
  folderId: string,
): Set<string> {
  const next = new Set(collapsedIds);
  if (next.has(folderId)) next.delete(folderId);
  else next.add(folderId);
  return next;
}

export function withFolderExpanded(collapsedIds: ReadonlySet<string>, folderId: string): Set<string> {
  const next = new Set(collapsedIds);
  next.delete(folderId);
  return next;
}

export function folderUnreadCount(feeds: Feed[], folder: Folder): number {
  return feedsForFolder(feeds, folder).reduce((n, feed) => n + (feed.unreadCount || 0), 0);
}

export function folderNameWithUnread(name: string, unread: number): string {
  if (unread <= 0) {
    return name;
  }
  return `${name} (${unread})`;
}
