/** Shared API contract between Electron renderer and Go backend. */

export const PROTOCOL_VERSION = 1;

export type ErrorCode =
  | "INVALID_PARAMS"
  | "NOT_FOUND"
  | "FEED_NOT_FOUND"
  | "ARTICLE_NOT_FOUND"
  | "INVALID_URL"
  | "INVALID_FEED"
  | "NETWORK_ERROR"
  | "PARSE_ERROR"
  | "DATABASE_ERROR"
  | "INTERNAL"
  | "UNSUPPORTED_METHOD";

export interface RpcError {
  code: ErrorCode;
  message: string;
}

export interface Feed {
  id: string;
  url: string;
  title: string;
  description: string;
  siteUrl: string;
  iconUrl: string;
  lastSuccessAt: string | null;
  lastAttemptAt: string | null;
  lastError: string | null;
  etag: string;
  lastModified: string;
  pollIntervalSeconds: number;
  enabled: boolean;
  unreadCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface Article {
  id: string;
  feedId: string;
  title: string;
  url: string;
  author: string;
  content: string;
  summary: string;
  publishedAt: string | null;
  updatedAt: string | null;
  externalId: string;
  isRead: boolean;
  isStarred: boolean;
  discoveredAt: string;
  feedTitle?: string;
}

export interface Folder {
  id: string;
  name: string;
  createdAt: string;
}

export interface ArticleQuery {
  feedId?: string;
  folderId?: string;
  unreadOnly?: boolean;
  starredOnly?: boolean;
  search?: string;
  limit?: number;
  cursor?: string;
}

export interface ArticleListResult {
  articles: Article[];
  nextCursor: string | null;
}

export interface FeedPreview {
  url: string;
  title: string;
  description: string;
  siteUrl: string;
  articleCount: number;
}

export interface Settings {
  defaultPollIntervalSeconds: number;
  theme: "system" | "light" | "dark";
  articleDensity: "comfortable" | "compact";
  defaultSort: "newest" | "oldest";
  markReadOnOpen: boolean;
  notificationsEnabled: boolean;
}

export interface FeedAddParams {
  url: string;
  confirm?: boolean;
}

export type BackendEventName =
  | "feed.updated"
  | "feed.error"
  | "articles.added"
  | "article.updated"
  | "sync.status";

export interface BackendEvent<T = unknown> {
  event: BackendEventName;
  payload: T;
}

export interface ReaderBackend {
  feeds: {
    list(): Promise<Feed[]>;
    get(id: string): Promise<Feed>;
    preview(url: string): Promise<FeedPreview>;
    add(url: string): Promise<Feed>;
    remove(id: string): Promise<void>;
    refresh(id: string): Promise<void>;
    refreshAll(): Promise<void>;
    setEnabled(id: string, enabled: boolean): Promise<Feed>;
    setPollInterval(id: string, seconds: number): Promise<Feed>;
  };
  articles: {
    list(query: ArticleQuery): Promise<ArticleListResult>;
    get(id: string): Promise<Article>;
    markRead(id: string): Promise<Article>;
    markUnread(id: string): Promise<Article>;
    toggleStar(id: string): Promise<Article>;
  };
  folders: {
    list(): Promise<Folder[]>;
    create(name: string): Promise<Folder>;
    remove(id: string): Promise<void>;
    assignFeed(folderId: string, feedId: string): Promise<void>;
    unassignFeed(folderId: string, feedId: string): Promise<void>;
  };
  settings: {
    get(): Promise<Settings>;
    update(patch: Partial<Settings>): Promise<Settings>;
  };
  system: {
    ping(): Promise<{ ok: true; version: string }>;
    info(): Promise<{ version: string; dbPath: string; protocolVersion: number }>;
  };
  onEvent(handler: (event: BackendEvent) => void): () => void;
}

export const RPC_METHODS = [
  "system.ping",
  "system.handshake",
  "system.info",
  "feeds.list",
  "feeds.get",
  "feeds.preview",
  "feeds.add",
  "feeds.remove",
  "feeds.refresh",
  "feeds.refreshAll",
  "feeds.setEnabled",
  "feeds.setPollInterval",
  "articles.list",
  "articles.get",
  "articles.markRead",
  "articles.markUnread",
  "articles.toggleStar",
  "folders.list",
  "folders.create",
  "folders.remove",
  "folders.assignFeed",
  "folders.unassignFeed",
  "settings.get",
  "settings.update",
] as const;

export type RpcMethod = (typeof RPC_METHODS)[number];
