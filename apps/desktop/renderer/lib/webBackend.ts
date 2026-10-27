import {
  cacheable,
  cachedResponse,
  saveResponse,
  queueReadState,
  pendingChanges,
  removePending,
} from "./offline";
import { readOfflineSession } from "./offlineSession";
import type { BackendEvent, ReaderBackend } from "@rss-reader/shared";

type ErrorBody = { error?: { code?: string; message?: string } };

export class WebBackendError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code?: string,
  ) {
    super(message);
  }
}

async function networkRequest<T>(
  method: string,
  params: unknown = {},
): Promise<T> {
  const response = await fetch("/v1/rpc", {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-RSS-CSRF": "1" },
    body: JSON.stringify({ method, params }),
    signal: AbortSignal.timeout(10_000),
  });
  if (!response.ok) {
    let body: ErrorBody = {};
    try {
      body = (await response.json()) as ErrorBody;
    } catch {
      // Keep the response status as the useful error signal.
    }
    if (response.status === 401)
      window.dispatchEvent(new Event("rss:unauthorized"));
    throw new WebBackendError(
      body.error?.message || `Server request failed (${response.status})`,
      response.status,
      body.error?.code,
    );
  }
  return (await response.json()) as T;
}

export function createWebBackend(account = ""): ReaderBackend {
  let syncing: Promise<void> | null = null;
  async function flush() {
    if (syncing) return syncing;
    syncing = (async () => {
      const pending = await pendingChanges(account).catch(() => []);
      if (!pending.length) return;
      // Verify the server identity before replaying another session's outbox.
      const session = await fetch("/v1/web/session", {
        cache: "no-store",
        credentials: "same-origin",
        signal: AbortSignal.timeout(10_000),
      });
      if (!session.ok || (await session.json()).user?.id !== account) return;
      for (const item of pending) {
        try {
          await networkRequest(item.method, item.params);
        } catch (error) {
          if (!(error instanceof WebBackendError && error.status === 404))
            throw error;
        }
        await removePending(item);
      }
      window.dispatchEvent(new Event("rss:offline-change"));
    })().finally(() => {
      syncing = null;
    });
    return syncing;
  }
  async function request<T>(method: string, params: unknown = {}): Promise<T> {
    try {
      if (!navigator.onLine) throw new TypeError("Offline");
      await flush();
      const value = await networkRequest<T>(method, params);
      if (
        account &&
        readOfflineSession<{ user: { id: string } }>()?.user.id === account &&
        cacheable(method)
      )
        await saveResponse(account, method, params, value).catch(() => {
          window.dispatchEvent(new Event("rss:storage-unavailable"));
        });
      return value;
    } catch (error) {
      // HTTP failures (especially 401/403) must never fall back to private data.
      if (error instanceof WebBackendError) throw error;
      if (
        account &&
        readOfflineSession<{ user: { id: string } }>()?.user.id === account &&
        (error instanceof TypeError ||
          (error instanceof DOMException &&
            ["AbortError", "TimeoutError"].includes(error.name)))
      ) {
        if (method === "articles.markRead" || method === "articles.markUnread")
          return queueReadState(
            account,
            method,
            params as { id: string },
          ) as Promise<T>;
        if (cacheable(method)) {
          const saved = await cachedResponse<T>(account, method, params).catch(
            () => undefined,
          );
          if (saved !== undefined) return saved;
          if (method === "articles.fetchLive") {
            const article = await cachedResponse<T>(
              account,
              "articles.get",
              params,
            ).catch(() => undefined);
            if (article !== undefined) return article;
          }
        }
      }
      throw error;
    }
  }
  async function requestOptional<T>(
    method: string,
    params: unknown = {},
  ): Promise<T | null> {
    try {
      return await request<T>(method, params);
    } catch (error) {
      if (error instanceof WebBackendError && error.status === 404) return null;
      throw error;
    }
  }
  let events: EventSource | null = null;
  const handlers = new Set<(event: BackendEvent) => void>();
  const ensureEvents = () => {
    if (events) return;
    events = new EventSource("/v1/events");
    events.addEventListener("auth-required", () =>
      window.dispatchEvent(new Event("rss:unauthorized")),
    );
    events.addEventListener("auth-unavailable", () =>
      window.dispatchEvent(new Event("rss:auth-unavailable")),
    );
    events.onmessage = (message) => {
      try {
        const event = JSON.parse(message.data) as BackendEvent;
        for (const handler of handlers) handler(event);

      } catch {
        // Ignore malformed or forward-incompatible event payloads.
      }
    };
  };

  return {
    feeds: {
      list: () => request("feeds.list"),
      get: (id) => request("feeds.get", { id }),
      preview: (url) => request("feeds.preview", { url }),
      add: (url) => request("feeds.add", { url }),
      remove: (id) => request("feeds.remove", { id }),
      refresh: (id) => request("feeds.refresh", { id }),
      refreshAll: () => request("feeds.refreshAll"),
      setEnabled: (id, enabled) => request("feeds.setEnabled", { id, enabled }),
      setPollInterval: (id, seconds) =>
        request("feeds.setPollInterval", { id, seconds }),
      exportUrls: () => request("feeds.exportUrls"),
      importUrls: (text) => request("feeds.importUrls", { text }),
    },
    articles: {
      list: (query) => request("articles.list", query),
      get: (id) => request("articles.get", { id }),
      markRead: (id) => request("articles.markRead", { id }),
      markUnread: (id) => request("articles.markUnread", { id }),
      markAllRead: (query) => request("articles.markAllRead", query),
      toggleStar: (id) => request("articles.toggleStar", { id }),
      recrawl: (id) => request("articles.recrawl", { id }),
      fetchLive: (id) => request("articles.fetchLive", { id }),
      // Kept for contract compatibility. The server ignores browser-provided
      // extraction and always returns its canonical article.
      setExtract: (articleId) => request("articles.setExtract", { articleId }),
      pendingExtract: () => Promise.resolve({ articleIds: [] }),
    },
    readLater: {
      add: (url) => request("readLater.add", { url }),
      addFromArticle: (articleId) =>
        request("readLater.addFromArticle", { articleId }),
      list: (filter, search) => request("readLater.list", { filter, search }),
      archive: (id) => request("readLater.archive", { id }),
      unarchive: (id) => request("readLater.unarchive", { id }),
      remove: (id) => request("readLater.remove", { id }),
    },
    sports: {
      teams: () => request("sports.teams.list"),
      seasons: () => request("sports.seasons.list"),
      followedGet: () => request("sports.followed.get"),
      followedSet: (teamIds) => request("sports.followed.set", { teamIds }),
      followedToggle: (teamId) => request("sports.followed.toggle", { teamId }),
      schedule: (params) => request("sports.schedule.list", params),
      dailySchedule: (params) => request("sports.schedule.daily", params),
      gameGet: (gamePk) => request("sports.game.get", { gamePk }),
      gameWatch: (gamePk) => request("sports.game.watch", { gamePk }),
      gameUnwatch: (gamePk) => request("sports.game.unwatch", { gamePk }),
      standings: (params) => request("sports.standings.get", params),
      roster: (params) => request("sports.roster.get", params),
      f1Years: () => request("sports.f1.years.list"),
      f1Races: (params) => request("sports.f1.races.list", params),
      f1RaceGet: (sessionKey) => request("sports.f1.race.get", { sessionKey }),
      f1RaceWatch: (sessionKey) =>
        request("sports.f1.race.watch", { sessionKey }),
      f1RaceUnwatch: (sessionKey) =>
        request("sports.f1.race.unwatch", { sessionKey }),
      f1Standings: (params) => request("sports.f1.standings.get", params),
    },
    stories: {
      list: () => request("stories.list"),
      get: (id) => requestOptional("stories.get", { id }),
      markRead: (id) => request("stories.markRead", { id }),
      markUnread: (id) => request("stories.markUnread", { id }),
      toggleStar: (id) => request("stories.toggleStar", { id }),
      voteArticle: (storyId, articleId, vote) =>
        request("stories.voteArticle", { storyId, articleId, vote }),
      voteStory: (id, vote) => request("stories.voteStory", { id, vote }),
      reindex: () => request("stories.reindex"),
      split: (id) => request("stories.split", { id }),
    },
    folders: {
      list: () => request("folders.list"),
      create: (name) => request("folders.create", { name }),
      remove: (id) => request("folders.remove", { id }),
      assignFeed: (folderId, feedId) =>
        request("folders.assignFeed", { folderId, feedId }),
      unassignFeed: (folderId, feedId) =>
        request("folders.unassignFeed", { folderId, feedId }),
    },
    settings: {
      get: () => request("settings.get"),
      update: (patch) => request("settings.update", patch),
    },
    ai: {
      test: () => request("ai.test"),
      scan: (window) => request("ai.scan", { window }),
      status: () => request("ai.status"),
      logs: (limit) => request("ai.logs", limit === undefined ? {} : { limit }),
      retryFailed: () => request("ai.retryFailed"),
    },
    errors: {
      list: (limit) =>
        request("errors.list", limit === undefined ? {} : { limit }),
    },
    system: {
      ping: () => request("system.ping"),
      info: () => request("system.info"),
    },
    onEvent: (handler) => {
      handlers.add(handler);
      ensureEvents();
      return () => {
        handlers.delete(handler);
        if (handlers.size === 0) {
          events?.close();
          events = null;
        }
      };
    },
  };
}

export function installWebDesktopShim() {
  window.desktop = {
    openExternal: async (url) => {
      window.open(url, "_blank", "noopener,noreferrer");
    },
    // Web activity notifications are scheduled exclusively by the server.
    notify: async () => false,
    focusMainWindow: async () => window.focus(),
    onAddLinkRequested: () => () => undefined,
    onOpenInPane: () => () => undefined,
  };
}
