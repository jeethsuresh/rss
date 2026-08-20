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

export type Priority = "none" | "low" | "medium" | "high";

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
  isReadLater: boolean;
  crawlAttempts: number;
  crawlFailures: number;
  badCrawlPercent: number;
  createdAt: string;
  updatedAt: string;
}

export type CrawlStatus = "none" | "pending" | "ok" | "failed";

export interface Article {
  id: string;
  feedId: string;
  title: string;
  url: string;
  author: string;
  content: string;
  summary: string;
  rssContent: string;
  crawledContent: string;
  liveContent: string;
  crawlStatus: CrawlStatus;
  crawlError: string;
  crawlUnreliable: boolean;
  publishedAt: string | null;
  updatedAt: string | null;
  externalId: string;
  isRead: boolean;
  isStarred: boolean;
  isReadLater: boolean;
  archivedAt?: string | null;
  priority: Priority;
  storyId?: string;
  discoveredAt: string;
  feedTitle?: string;
}

export interface Story {
  id: string;
  title: string;
  summary: string;
  isRead: boolean;
  isStarred: boolean;
  memberCount: number;
  createdAt: string;
  updatedAt: string;
  articleIds?: string[];
  articles?: Article[];
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

export interface FeedImportResult {
  added: number;
  failed: number;
  errors: string[];
}

export interface Settings {
  defaultPollIntervalSeconds: number;
  theme: "system" | "light" | "dark";
  articleDensity: "comfortable" | "compact";
  defaultSort: "newest" | "oldest";
  markReadOnOpen: boolean;
  notificationsEnabled: boolean;
  aiEnabled: boolean;
  aiBaseUrl: string;
  aiModel: string;
}

export type ReadLaterFilter = "all" | "unread" | "starred" | "archived";

export interface AIStatus {
  running: boolean;
  processed: number;
  total: number;
  pending: number;
  failed: number;
  lastError: string;
}

export interface AILogEntry {
  id: string;
  ts: string;
  level: string;
  articleId?: string;
  message: string;
  detail?: string;
}

export interface AITestResult {
  ok: boolean;
  message: string;
  models?: string[];
}

export type BackendEventName =
  | "feed.updated"
  | "feed.error"
  | "articles.added"
  | "article.updated"
  | "story.updated"
  | "ai.status"
  | "ai.log"
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
    exportUrls(): Promise<{ text: string }>;
    importUrls(text: string): Promise<FeedImportResult>;
  };
  articles: {
    list(query: ArticleQuery): Promise<ArticleListResult>;
    get(id: string): Promise<Article>;
    markRead(id: string): Promise<Article>;
    markUnread(id: string): Promise<Article>;
    toggleStar(id: string): Promise<Article>;
    recrawl(id: string): Promise<Article>;
    fetchLive(id: string): Promise<Article>;
  };
  readLater: {
    add(url: string): Promise<Article>;
    addFromArticle(articleId: string): Promise<Article>;
    list(filter?: ReadLaterFilter, search?: string): Promise<Article[]>;
    archive(id: string): Promise<Article>;
    unarchive(id: string): Promise<Article>;
  };
  stories: {
    list(): Promise<Story[]>;
    get(id: string): Promise<Story>;
    markRead(id: string): Promise<Story>;
    markUnread(id: string): Promise<Story>;
    toggleStar(id: string): Promise<Story>;
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
  ai: {
    test(): Promise<AITestResult>;
    scan(window: "24h" | "7d" | "missed"): Promise<{ queued: boolean; status: AIStatus }>;
    status(): Promise<AIStatus>;
    logs(limit?: number): Promise<AILogEntry[]>;
    retryFailed(): Promise<{ requeued: number; status: AIStatus }>;
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
  "feeds.exportUrls",
  "feeds.importUrls",
  "articles.list",
  "articles.get",
  "articles.markRead",
  "articles.markUnread",
  "articles.toggleStar",
  "articles.recrawl",
  "articles.fetchLive",
  "readLater.add",
  "readLater.addFromArticle",
  "readLater.list",
  "readLater.archive",
  "readLater.unarchive",
  "stories.list",
  "stories.get",
  "stories.markRead",
  "stories.markUnread",
  "stories.toggleStar",
  "folders.list",
  "folders.create",
  "folders.remove",
  "folders.assignFeed",
  "folders.unassignFeed",
  "settings.get",
  "settings.update",
  "ai.test",
  "ai.scan",
  "ai.status",
  "ai.logs",
  "ai.retryFailed",
] as const;

export type RpcMethod = (typeof RPC_METHODS)[number];
