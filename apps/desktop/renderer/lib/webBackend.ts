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

async function request<T>(method: string, params: unknown = {}): Promise<T> {
  const response = await fetch("/v1/rpc", {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-RSS-CSRF": "1" },
    body: JSON.stringify({ method, params }),
  });
  if (!response.ok) {
    let body: ErrorBody = {};
    try {
      body = (await response.json()) as ErrorBody;
    } catch {
      // Keep the response status as the useful error signal.
    }
    if (response.status === 401) window.dispatchEvent(new Event("rss:unauthorized"));
    throw new WebBackendError(
      body.error?.message || `Server request failed (${response.status})`,
      response.status,
      body.error?.code,
    );
  }
  return (await response.json()) as T;
}

export function createWebBackend(): ReaderBackend {
  let events: EventSource | null = null;
  const handlers = new Set<(event: BackendEvent) => void>();
  const ensureEvents = () => {
    if (events) return;
    events = new EventSource("/v1/events");
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
      setPollInterval: (id, seconds) => request("feeds.setPollInterval", { id, seconds }),
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
      addFromArticle: (articleId) => request("readLater.addFromArticle", { articleId }),
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
      f1RaceWatch: (sessionKey) => request("sports.f1.race.watch", { sessionKey }),
      f1RaceUnwatch: (sessionKey) => request("sports.f1.race.unwatch", { sessionKey }),
      f1Standings: (params) => request("sports.f1.standings.get", params),
    },
    stories: {
      list: () => request("stories.list"),
      get: (id) => request("stories.get", { id }),
      markRead: (id) => request("stories.markRead", { id }),
      markUnread: (id) => request("stories.markUnread", { id }),
      toggleStar: (id) => request("stories.toggleStar", { id }),
      voteArticle: (storyId, articleId, vote) => request("stories.voteArticle", { storyId, articleId, vote }),
      voteStory: (id, vote) => request("stories.voteStory", { id, vote }),
      reindex: () => request("stories.reindex"),
      split: (id) => request("stories.split", { id }),
    },
    folders: {
      list: () => request("folders.list"),
      create: (name) => request("folders.create", { name }),
      remove: (id) => request("folders.remove", { id }),
      assignFeed: (folderId, feedId) => request("folders.assignFeed", { folderId, feedId }),
      unassignFeed: (folderId, feedId) => request("folders.unassignFeed", { folderId, feedId }),
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
    errors: { list: (limit) => request("errors.list", limit === undefined ? {} : { limit }) },
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
    notify: async (title, body) => {
      if (!("Notification" in window)) return false;
      if (Notification.permission === "default") await Notification.requestPermission();
      if (Notification.permission !== "granted") return false;
      new Notification(title, { body });
      return true;
    },
    focusMainWindow: async () => window.focus(),
    onAddLinkRequested: () => () => undefined,
    onOpenInPane: () => () => undefined,
  };
}
