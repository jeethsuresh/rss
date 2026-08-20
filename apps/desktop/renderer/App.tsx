import { useCallback, useEffect, useMemo, useState } from "react";
import type { Article, Feed, Folder, Settings } from "@rss-reader/shared";
import { getBackend } from "./lib/backend";
import { formatRelativeTime, sanitizeArticleHtml, stripHtml } from "./lib/html";

type Selection =
  | { type: "all" }
  | { type: "unread" }
  | { type: "starred" }
  | { type: "feed"; id: string }
  | { type: "folder"; id: string };

export function App() {
  const backend = useMemo(() => {
    try {
      return getBackend();
    } catch {
      return null;
    }
  }, []);

  if (!backend) {
    return (
      <div className="app">
        <div className="empty">
          <h2>RSS Reader</h2>
          <p className="error">
            Desktop bridge failed to load. Rebuild with bun dev (preload must be CommonJS).
          </p>
        </div>
      </div>
    );
  }

  return <AppMain backend={backend} />;
}

function AppMain({ backend }: { backend: NonNullable<ReturnType<typeof getBackend>> }) {
  const [feeds, setFeeds] = useState<Feed[]>([]);
  const [folders, setFolders] = useState<Folder[]>([]);
  const [articles, setArticles] = useState<Article[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [selected, setSelected] = useState<Selection>({ type: "unread" });
  const [activeId, setActiveId] = useState<string | null>(null);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [search, setSearch] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [showAdd, setShowAdd] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [addUrl, setAddUrl] = useState("");

  const active = articles.find((a) => a.id === activeId) ?? null;

  const applyTheme = useCallback((theme: Settings["theme"]) => {
    const root = document.documentElement;
    if (theme === "system") {
      const dark = window.matchMedia("(prefers-color-scheme: dark)").matches;
      root.dataset.theme = dark ? "dark" : "light";
    } else {
      root.dataset.theme = theme;
    }
  }, []);

  const loadFeeds = useCallback(async () => {
    const [f, foldersList, s] = await Promise.all([
      backend.feeds.list(),
      backend.folders.list(),
      backend.settings.get(),
    ]);
    setFeeds(f ?? []);
    setFolders(foldersList ?? []);
    setSettings(s);
    applyTheme(s.theme);
  }, [backend, applyTheme]);

  const loadArticles = useCallback(
    async (append = false) => {
      const query = {
        unreadOnly: selected.type === "unread" ? true : undefined,
        starredOnly: selected.type === "starred" ? true : undefined,
        feedId: selected.type === "feed" ? selected.id : undefined,
        folderId: selected.type === "folder" ? selected.id : undefined,
        search: search.trim() || undefined,
        limit: 50,
        cursor: append ? nextCursor ?? undefined : undefined,
      };
      const res = await backend.articles.list(query);
      const list = res.articles ?? [];
      setArticles((prev) => (append ? [...prev, ...list] : list));
      setNextCursor(res.nextCursor);
      if (!append) {
        setActiveId((id) => {
          if (id && list.some((a) => a.id === id)) return id;
          return list[0]?.id ?? null;
        });
      }
    },
    [backend, selected, search, nextCursor],
  );

  const refreshAll = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      await backend.feeds.refreshAll();
      await loadFeeds();
      await loadArticles(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Refresh failed");
    } finally {
      setBusy(false);
    }
  }, [backend, loadFeeds, loadArticles]);

  useEffect(() => {
    void (async () => {
      try {
        await backend.system.ping();
        await loadFeeds();
        await loadArticles(false);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to start");
      }
    })();
  }, []);

  useEffect(() => {
    void loadArticles(false);
  }, [selected, search]);

  useEffect(() => {
    return backend.onEvent((event) => {
      // Do not reload the article list on article.updated — that removes the
      // just-opened item from Unread and breaks j/k navigation.
      switch (event.event) {
        case "articles.added":
        case "feed.updated":
        case "feed.error":
          void loadFeeds();
          void loadArticles(false);
          break;
        case "article.updated":
          void loadFeeds();
          break;
        case "sync.status":
          break;
        default: {
          const _exhaustive: never = event.event;
          void _exhaustive;
          break;
        }
      }
      if (event.event === "articles.added" && settings?.notificationsEnabled) {
        const payload = event.payload as { count?: number; feedId?: string };
        void window.desktop.notify(
          "New articles",
          `${payload.count ?? "Some"} new article(s) arrived`,
        );
      }
    });
  }, [backend, loadFeeds, loadArticles, settings]);

  const patchArticle = useCallback((updated: Article) => {
    setArticles((prev) => prev.map((a) => (a.id === updated.id ? { ...a, ...updated } : a)));
  }, []);

  const selectArticle = useCallback(
    async (article: Article) => {
      setActiveId(article.id);
      if (settings?.markReadOnOpen && !article.isRead) {
        try {
          const updated = await backend.articles.markRead(article.id);
          patchArticle(updated);
          void loadFeeds();
        } catch (e) {
          setError(e instanceof Error ? e.message : "Failed to mark read");
        }
      }
    },
    [backend, settings?.markReadOnOpen, patchArticle, loadFeeds],
  );

  const moveSelection = useCallback(
    (delta: number) => {
      if (articles.length === 0) return;
      const idx = articles.findIndex((a) => a.id === activeId);
      const from = idx < 0 ? (delta > 0 ? -1 : 0) : idx;
      const to = Math.min(articles.length - 1, Math.max(0, from + delta));
      const next = articles[to];
      if (next && next.id !== activeId) {
        void selectArticle(next);
      }
    },
    [articles, activeId, selectArticle],
  );

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA") {
        if (e.key === "Escape") (e.target as HTMLElement).blur();
        return;
      }
      switch (e.key) {
        case "j":
          moveSelection(1);
          break;
        case "k":
          moveSelection(-1);
          break;
        case "o":
          if (active?.url) void window.desktop.openExternal(active.url);
          break;
        case "r":
          if (active) {
            void backend.articles.markRead(active.id).then((updated) => {
              patchArticle(updated);
              void loadFeeds();
            });
          }
          break;
        case "u":
          if (active) {
            void backend.articles.markUnread(active.id).then((updated) => {
              patchArticle(updated);
              void loadFeeds();
            });
          }
          break;
        case "s":
          if (active) {
            void backend.articles.toggleStar(active.id).then((updated) => {
              patchArticle(updated);
            });
          }
          break;
        case "f":
          void refreshAll();
          break;
        case "/":
          e.preventDefault();
          document.getElementById("search-input")?.focus();
          break;
        default:
          break;
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [active, articles, backend, refreshAll, loadFeeds, activeId, patchArticle, selectArticle, moveSelection]);

  const addFeed = async () => {
    setBusy(true);
    setError(null);
    try {
      await backend.feeds.add(addUrl.trim());
      setShowAdd(false);
      setAddUrl("");
      await loadFeeds();
      await loadArticles(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not add feed");
    } finally {
      setBusy(false);
    }
  };

  const totalUnread = feeds.reduce((n, f) => n + f.unreadCount, 0);
  const densityClass = settings?.articleDensity === "compact" ? "density-compact" : "";

  return (
    <div className={`app ${densityClass}`}>
      <header className="toolbar">
        <div className="brand">RSS Reader</div>
        <span className={`status-dot ${error ? "error" : ""}`} title={error ?? "Connected"} />
        <div className="toolbar-spacer" />
        <input
          id="search-input"
          className="search"
          placeholder="Search articles  (/)"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <button className="btn" onClick={() => void refreshAll()} disabled={busy}>
          {busy ? "Refreshing…" : "Refresh"}
        </button>
        <button className="btn" onClick={() => setShowSettings(true)}>
          Settings
        </button>
        <button className="btn primary" onClick={() => setShowAdd(true)}>
          Add feed
        </button>
      </header>

      <div className="layout">
        <aside className="pane sidebar">
          <button
            className={`nav-item ${selected.type === "all" ? "active" : ""}`}
            onClick={() => setSelected({ type: "all" })}
          >
            <span>All</span>
          </button>
          <button
            className={`nav-item ${selected.type === "unread" ? "active" : ""}`}
            onClick={() => setSelected({ type: "unread" })}
          >
            <span>Unread</span>
            <span className="count">{totalUnread || ""}</span>
          </button>
          <button
            className={`nav-item ${selected.type === "starred" ? "active" : ""}`}
            onClick={() => setSelected({ type: "starred" })}
          >
            <span>Starred</span>
          </button>

          <div className="section-label">Folders</div>
          <button
            className="nav-item"
            onClick={() => {
              const name = window.prompt("Folder name");
              if (!name?.trim()) return;
              void backend.folders.create(name.trim()).then(() => loadFeeds());
            }}
          >
            + New folder
          </button>
          {folders.map((folder) => (
            <button
              key={folder.id}
              className={`nav-item ${selected.type === "folder" && selected.id === folder.id ? "active" : ""}`}
              onClick={() => setSelected({ type: "folder", id: folder.id })}
              onContextMenu={(e) => {
                e.preventDefault();
                if (selected.type === "feed") {
                  void backend.folders.assignFeed(folder.id, selected.id).then(() => loadFeeds());
                }
              }}
              title="Right-click to assign the selected feed"
            >
              {folder.name}
            </button>
          ))}

          <div className="section-label">Feeds</div>
          {feeds.length === 0 && <div className="empty" style={{ height: "auto", padding: 12 }}>No feeds yet</div>}
          {feeds.map((feed) => (
            <button
              key={feed.id}
              className={`feed-item ${selected.type === "feed" && selected.id === feed.id ? "active" : ""}`}
              onClick={() => setSelected({ type: "feed", id: feed.id })}
              title={feed.lastError || feed.url}
            >
              <span>
                {!feed.enabled ? "⏸ " : feed.lastError ? "⚠ " : ""}
                {feed.title || feed.url}
              </span>
              <span className="count">{feed.unreadCount || ""}</span>
            </button>
          ))}
        </aside>

        <section className="pane article-list">
          {articles.length === 0 ? (
            <div className="empty">
              <h2>Nothing here</h2>
              <p>Add a feed or widen your filters.</p>
            </div>
          ) : (
            articles.map((article) => (
              <button
                key={article.id}
                className={`article-row ${article.id === activeId ? "active" : ""} ${article.isRead ? "" : "unread"}`}
                onClick={() => void selectArticle(article)}
              >
                <div className="article-meta">
                  <span>{article.feedTitle}</span>
                  <span>{formatRelativeTime(article.publishedAt ?? article.discoveredAt)}</span>
                  {article.isStarred ? <span>★</span> : null}
                </div>
                <h3 className="article-title">{article.title || "(untitled)"}</h3>
                <p className="article-summary">{stripHtml(article.summary || article.content)}</p>
              </button>
            ))
          )}
          {nextCursor && (
            <button className="btn" style={{ width: "100%", marginTop: 8 }} onClick={() => void loadArticles(true)}>
              Load more
            </button>
          )}
        </section>

        <section className="pane reader-pane">
          {!active ? (
            <div className="empty">
              <h2>RSS Reader</h2>
              <p>Select an article to read. Shortcuts: j/k, o, r, u, s, f, /</p>
            </div>
          ) : (
            <article className="reader">
              <div className="reader-kicker">
                {active.feedTitle}
                {active.author ? ` · ${active.author}` : ""}
                {active.publishedAt ? ` · ${new Date(active.publishedAt).toLocaleString()}` : ""}
              </div>
              <h1>{active.title || "(untitled)"}</h1>
              <div className="reader-actions">
                <button
                  className="btn"
                  onClick={() =>
                    void backend.articles[active.isRead ? "markUnread" : "markRead"](active.id).then((updated) => {
                      patchArticle(updated);
                      void loadFeeds();
                    })
                  }
                >
                  {active.isRead ? "Mark unread" : "Mark read"}
                </button>
                <button
                  className="btn"
                  onClick={() =>
                    void backend.articles.toggleStar(active.id).then((updated) => {
                      patchArticle(updated);
                    })
                  }
                >
                  {active.isStarred ? "Unstar" : "Star"}
                </button>
                {active.url && (
                  <button className="btn primary" onClick={() => void window.desktop.openExternal(active.url)}>
                    Open original
                  </button>
                )}
              </div>
              <div
                className="reader-body"
                dangerouslySetInnerHTML={{
                  __html: sanitizeArticleHtml(active.content || active.summary || "<p>No content</p>"),
                }}
              />
            </article>
          )}
        </section>
      </div>

      {showAdd && (
        <div className="modal-backdrop" onClick={() => setShowAdd(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h2>Add feed</h2>
            {error && <p className="error">{error}</p>}
            <input
              autoFocus
              placeholder="https://example.com/feed.xml"
              value={addUrl}
              onChange={(e) => setAddUrl(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") void addFeed();
              }}
            />
            <div className="modal-actions">
              <button className="btn" onClick={() => setShowAdd(false)}>
                Cancel
              </button>
              <button className="btn primary" disabled={busy || !addUrl.trim()} onClick={() => void addFeed()}>
                Add
              </button>
            </div>
          </div>
        </div>
      )}

      {showSettings && settings && (
        <div className="modal-backdrop" onClick={() => setShowSettings(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h2>Settings</h2>
            <label>
              Theme{" "}
              <select
                value={settings.theme}
                onChange={(e) => {
                  const theme = e.target.value as Settings["theme"];
                  void backend.settings.update({ theme }).then((s) => {
                    setSettings(s);
                    applyTheme(s.theme);
                  });
                }}
              >
                <option value="system">System</option>
                <option value="light">Light</option>
                <option value="dark">Dark</option>
              </select>
            </label>
            <div style={{ height: 12 }} />
            <label>
              Density{" "}
              <select
                value={settings.articleDensity}
                onChange={(e) => {
                  const articleDensity = e.target.value as Settings["articleDensity"];
                  void backend.settings.update({ articleDensity }).then(setSettings);
                }}
              >
                <option value="comfortable">Comfortable</option>
                <option value="compact">Compact</option>
              </select>
            </label>
            <div style={{ height: 12 }} />
            <label>
              Default poll (seconds){" "}
              <input
                type="number"
                min={60}
                value={settings.defaultPollIntervalSeconds}
                onChange={(e) => {
                  const defaultPollIntervalSeconds = Number(e.target.value);
                  void backend.settings.update({ defaultPollIntervalSeconds }).then(setSettings);
                }}
              />
            </label>
            <div style={{ height: 12 }} />
            <label>
              <input
                type="checkbox"
                checked={settings.markReadOnOpen}
                onChange={(e) => {
                  void backend.settings.update({ markReadOnOpen: e.target.checked }).then(setSettings);
                }}
              />{" "}
              Mark read on open
            </label>
            <div style={{ height: 8 }} />
            <label>
              <input
                type="checkbox"
                checked={settings.notificationsEnabled}
                onChange={(e) => {
                  void backend.settings.update({ notificationsEnabled: e.target.checked }).then(setSettings);
                }}
              />{" "}
              Desktop notifications
            </label>
            <div style={{ height: 16 }} />
            <div className="modal-actions">
              <button className="btn primary" onClick={() => setShowSettings(false)}>
                Done
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
