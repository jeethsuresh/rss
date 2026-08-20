import { contextBridge, ipcRenderer } from "electron";
import type { BackendEvent, ReaderBackend } from "@rss-reader/shared";

function request<T>(method: string, params?: unknown): Promise<T> {
  return ipcRenderer.invoke("backend:request", method, params ?? {}) as Promise<T>;
}

const api: ReaderBackend = {
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
    toggleStar: (id) => request("articles.toggleStar", { id }),
    recrawl: (id) => request("articles.recrawl", { id }),
    fetchLive: (id) => request("articles.fetchLive", { id }),
  },
  readLater: {
    add: (url) => request("readLater.add", { url }),
    list: () => request("readLater.list"),
  },
  stories: {
    list: () => request("stories.list"),
    get: (id) => request("stories.get", { id }),
    markRead: (id) => request("stories.markRead", { id }),
    markUnread: (id) => request("stories.markUnread", { id }),
    toggleStar: (id) => request("stories.toggleStar", { id }),
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
    logs: (limit) => request("ai.logs", limit !== undefined ? { limit } : {}),
    retryFailed: () => request("ai.retryFailed"),
  },
  system: {
    ping: () => request("system.ping"),
    info: () => request("system.info"),
  },
  onEvent: (handler) => {
    const listener = (_: Electron.IpcRendererEvent, event: BackendEvent) => handler(event);
    ipcRenderer.on("backend:event", listener);
    return () => ipcRenderer.removeListener("backend:event", listener);
  },
};

contextBridge.exposeInMainWorld("rss", api);
contextBridge.exposeInMainWorld("desktop", {
  openExternal: (url: string) => ipcRenderer.invoke("shell:openExternal", url),
  notify: (title: string, body: string) => ipcRenderer.invoke("app:notify", title, body),
});
