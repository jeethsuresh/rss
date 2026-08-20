import { useCallback, useEffect, useMemo, useState } from "react";
import type {
  Article,
  BackendEventName,
  Feed,
  Folder,
  Priority,
  Settings,
  Story,
} from "@rss-reader/shared";
import { getBackend } from "./lib/backend";
import { formatRelativeTime, sanitizeArticleHtml, stripHtml } from "./lib/html";
import { SettingsPage } from "./views/SettingsPage";
import { PageFrame } from "./components/PageFrame";

type Selection =
  | { type: "all" }
  | { type: "unread" }
  | { type: "starred" }
  | { type: "stories" }
  | { type: "readLater" }
  | { type: "feed"; id: string }
  | { type: "folder"; id: string };

type ContentTab = "primary" | "secondary";

type View = "reader" | "settings";

function priorityBadgeLabel(priority: Priority): string | null {
  switch (priority) {
    case "high":
      return "H";
    case "medium":
      return "M";
    case "low":
      return "L";
    case "none":
      return null;
    default: {
      const _exhaustive: never = priority;
      return _exhaustive;
    }
  }
}

function PriorityBadge({ priority }: { priority: Priority }) {
  const label = priorityBadgeLabel(priority);
  if (!label) return null;
  const cls =
    priority === "high" || priority === "medium" ? `priority-badge ${priority}` : "priority-badge";
  return <span className={cls}>{label}</span>;
}

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
  const [stories, setStories] = useState<Story[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [selected, setSelected] = useState<Selection>({ type: "unread" });
  const [activeId, setActiveId] = useState<string | null>(null);
  const [activeStoryId, setActiveStoryId] = useState<string | null>(null);
  const [activeStory, setActiveStory] = useState<Story | null>(null);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [search, setSearch] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [showAdd, setShowAdd] = useState(false);
  const [addUrl, setAddUrl] = useState("");
  const [view, setView] = useState<View>("reader");
  const [expandedMemberIds, setExpandedMemberIds] = useState<Record<string, boolean>>({});
  const [contentTab, setContentTab] = useState<ContentTab>("primary");
  const [contentBusy, setContentBusy] = useState(false);

  const active = articles.find((a) => a.id === activeId) ?? null;
  const isStoriesMode = selected.type === "stories";

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
      if (selected.type === "stories") return;
      if (selected.type === "readLater") {
        const list = await backend.readLater.list();
        setArticles(list ?? []);
        setNextCursor(null);
        setActiveId((id) => {
          if (id && list.some((a) => a.id === id)) return id;
          return list[0]?.id ?? null;
        });
        return;
      }
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

  const loadStories = useCallback(async () => {
    const list = await backend.stories.list();
    setStories(list ?? []);
    setActiveStoryId((id) => {
      if (id && list.some((s) => s.id === id)) return id;
      return list[0]?.id ?? null;
    });
  }, [backend]);

  const reloadContent = useCallback(
    (append = false) => {
      if (selected.type === "stories") {
        void loadStories();
      } else {
        void loadArticles(append);
      }
    },
    [selected.type, loadStories, loadArticles],
  );

  const refreshAll = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      await backend.feeds.refreshAll();
      await loadFeeds();
      if (selected.type === "stories") {
        await loadStories();
      } else {
        await loadArticles(false);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Refresh failed");
    } finally {
      setBusy(false);
    }
  }, [backend, loadFeeds, loadArticles, loadStories, selected.type]);

  useEffect(() => {
    void (async () => {
      try {
        await backend.system.ping();
        await loadFeeds();
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to start");
      }
    })();
  }, []);

  useEffect(() => {
    if (selected.type === "stories") {
      void loadStories();
    } else {
      void loadArticles(false);
    }
  }, [selected]);

  useEffect(() => {
    if (selected.type !== "stories") {
      void loadArticles(false);
    }
  }, [search]);

  useEffect(() => {
    if (!isStoriesMode || !activeStoryId) {
      if (!isStoriesMode) setActiveStory(null);
      return;
    }
    void backend.stories.get(activeStoryId).then(setActiveStory).catch(() => setActiveStory(null));
  }, [backend, isStoriesMode, activeStoryId]);

  useEffect(() => {
    return backend.onEvent((event) => {
      const name: BackendEventName = event.event;
      switch (name) {
        case "articles.added":
        case "feed.updated":
        case "feed.error":
          void loadFeeds();
          reloadContent(false);
          break;
        case "article.updated": {
          void loadFeeds();
          const payload = event.payload as {
            articleId?: string;
            priority?: Priority;
            isRead?: boolean;
            isStarred?: boolean;
          };
          if (payload.articleId) {
            setArticles((prev) =>
              prev.map((a) =>
                a.id === payload.articleId
                  ? {
                      ...a,
                      ...(payload.priority !== undefined ? { priority: payload.priority } : {}),
                      ...(payload.isRead !== undefined ? { isRead: payload.isRead } : {}),
                      ...(payload.isStarred !== undefined ? { isStarred: payload.isStarred } : {}),
                    }
                  : a,
              ),
            );
            setActiveStory((prev) => {
              if (!prev?.articles) return prev;
              return {
                ...prev,
                articles: prev.articles.map((a) =>
                  a.id === payload.articleId
                    ? {
                        ...a,
                        ...(payload.priority !== undefined ? { priority: payload.priority } : {}),
                        ...(payload.isRead !== undefined ? { isRead: payload.isRead } : {}),
                        ...(payload.isStarred !== undefined ? { isStarred: payload.isStarred } : {}),
                      }
                    : a,
                ),
              };
            });
          }
          break;
        }
        case "story.updated": {
          void loadStories();
          const payload = event.payload as { storyId?: string };
          if (payload.storyId && payload.storyId === activeStoryId) {
            void backend.stories.get(payload.storyId).then(setActiveStory);
          }
          break;
        }
        case "ai.status":
          break;
        case "ai.log":
          break;
        case "sync.status":
          break;
        default: {
          const _exhaustive: never = name;
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
  }, [backend, loadFeeds, reloadContent, loadStories, activeStoryId, settings]);

  const patchArticle = useCallback((updated: Article) => {
    setArticles((prev) => prev.map((a) => (a.id === updated.id ? { ...a, ...updated } : a)));
  }, []);

  const patchStory = useCallback((updated: Story) => {
    setStories((prev) => prev.map((s) => (s.id === updated.id ? { ...s, ...updated } : s)));
    setActiveStory((prev) => (prev?.id === updated.id ? updated : prev));
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

  const selectStory = useCallback(
    async (story: Story) => {
      setActiveStoryId(story.id);
      setExpandedMemberIds({});
      try {
        const full = await backend.stories.get(story.id);
        setActiveStory(full);
        if (settings?.markReadOnOpen && !full.isRead) {
          const updated = await backend.stories.markRead(full.id);
          patchStory(updated);
        }
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load story");
      }
    },
    [backend, settings?.markReadOnOpen, patchStory],
  );

  const moveSelection = useCallback(
    (delta: number) => {
      if (isStoriesMode) {
        if (stories.length === 0) return;
        const idx = stories.findIndex((s) => s.id === activeStoryId);
        const from = idx < 0 ? (delta > 0 ? -1 : 0) : idx;
        const to = Math.min(stories.length - 1, Math.max(0, from + delta));
        const next = stories[to];
        if (next && next.id !== activeStoryId) {
          void selectStory(next);
        }
        return;
      }
      if (articles.length === 0) return;
      const idx = articles.findIndex((a) => a.id === activeId);
      const from = idx < 0 ? (delta > 0 ? -1 : 0) : idx;
      const to = Math.min(articles.length - 1, Math.max(0, from + delta));
      const next = articles[to];
      if (next && next.id !== activeId) {
        void selectArticle(next);
      }
    },
    [isStoriesMode, stories, activeStoryId, selectStory, articles, activeId, selectArticle],
  );

  useEffect(() => {
    setContentTab("primary");
  }, [activeId]);

  const addReadLaterLink = async () => {
    const url = window.prompt("URL to save for later");
    if (!url?.trim()) return;
    setBusy(true);
    setError(null);
    try {
      await backend.readLater.add(url.trim());
      setSelected({ type: "readLater" });
      await loadArticles(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not save link");
    } finally {
      setBusy(false);
    }
  };

  const handleContentTab = useCallback(
    async (tab: ContentTab) => {
      setContentTab(tab);
      if (!active) return;
      const needsLiveFetch = active.isReadLater && tab === "primary" && !active.liveContent;
      if (!needsLiveFetch) return;
      setContentBusy(true);
      try {
        const updated = await backend.articles.fetchLive(active.id);
        patchArticle(updated);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to fetch live content");
      } finally {
        setContentBusy(false);
      }
    },
    [active, backend, patchArticle],
  );

  const recrawlActive = useCallback(async () => {
    if (!active) return;
    setContentBusy(true);
    setError(null);
    try {
      const updated = await backend.articles.recrawl(active.id);
      patchArticle(updated);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Recrawl failed");
    } finally {
      setContentBusy(false);
    }
  }, [active, backend, patchArticle]);

  useEffect(() => {
    if (!active?.isReadLater || contentTab !== "primary" || active.liveContent) return;
    setContentBusy(true);
    void backend.articles
      .fetchLive(active.id)
      .then((updated) => patchArticle(updated))
      .catch((e: unknown) =>
        setError(e instanceof Error ? e.message : "Failed to fetch live content"),
      )
      .finally(() => setContentBusy(false));
  }, [active?.id, active?.isReadLater, active?.liveContent, contentTab, backend, patchArticle]);

  const renderContentTabs = (article: Article) => {
    const primaryLabel = article.isReadLater ? "Live" : "Feed";
    const secondaryLabel = article.isReadLater ? "Saved crawl" : "Full page";
    return (
      <div className="content-tabs">
        <button
          type="button"
          className={`content-tab ${contentTab === "primary" ? "active" : ""}`}
          onClick={() => void handleContentTab("primary")}
        >
          {primaryLabel}
        </button>
        <button
          type="button"
          className={`content-tab ${contentTab === "secondary" ? "active" : ""}`}
          onClick={() => void handleContentTab("secondary")}
        >
          {secondaryLabel}
        </button>
      </div>
    );
  };

  const renderContentBody = (article: Article) => {
    let bodyHtml: string | null = null;
    let statusMessage: string | null = null;
    let asFullPage = false;

    if (contentTab === "primary") {
      if (article.isReadLater) {
        if (article.liveContent) {
          bodyHtml = article.liveContent;
          asFullPage = true;
        } else if (contentBusy) {
          statusMessage = "Fetching live page…";
        } else {
          statusMessage = "No live page yet.";
        }
      } else {
        bodyHtml = article.rssContent || article.content || article.summary || null;
        asFullPage = false;
      }
    } else if (article.crawlStatus === "pending") {
      statusMessage = "Crawl in progress…";
    } else if (article.crawlStatus === "failed" && !article.crawledContent) {
      statusMessage = article.crawlError || "Crawl failed.";
    } else if (article.crawledContent) {
      bodyHtml = article.crawledContent;
      asFullPage = true;
    } else if (article.crawlStatus === "none") {
      statusMessage = "No crawled page yet.";
    } else {
      statusMessage = "No crawled page available.";
    }

    if (bodyHtml && asFullPage) {
      return (
        <div className="reader-page-wrap">
          <PageFrame html={bodyHtml} pageUrl={article.url} title={article.title || "Article page"} />
          {contentTab === "secondary" && (
            <div className="reader-actions" style={{ marginTop: 8 }}>
              <button className="btn" disabled={contentBusy} onClick={() => void recrawlActive()}>
                {contentBusy ? "Re-crawling…" : "Re-crawl page"}
              </button>
            </div>
          )}
        </div>
      );
    }

    return bodyHtml ? (
      <div
        className="reader-body"
        dangerouslySetInnerHTML={{
          __html: sanitizeArticleHtml(bodyHtml),
        }}
      />
    ) : (
      <div className="reader-body">
        <p className="muted">{statusMessage ?? "No content"}</p>
        {contentTab === "secondary" && (
          <button className="btn" disabled={contentBusy} onClick={() => void recrawlActive()}>
            {contentBusy ? "Retrying…" : "Retry crawl"}
          </button>
        )}
      </div>
    );
  };

  const visibleFeeds = feeds.filter((f) => !f.isReadLater);

  const toggleMemberExpand = useCallback((articleId: string) => {
    setExpandedMemberIds((prev) => ({ ...prev, [articleId]: !prev[articleId] }));
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA") {
        if (e.key === "Escape") (e.target as HTMLElement).blur();
        return;
      }

      if (view === "settings") {
        if (e.key === "Escape") setView("reader");
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
          if (!isStoriesMode && active?.url) void window.desktop.openExternal(active.url);
          break;
        case "r":
          if (isStoriesMode && activeStory) {
            void backend.stories.markRead(activeStory.id).then((updated) => {
              patchStory(updated);
            });
          } else if (active) {
            void backend.articles.markRead(active.id).then((updated) => {
              patchArticle(updated);
              void loadFeeds();
            });
          }
          break;
        case "u":
          if (isStoriesMode && activeStory) {
            void backend.stories.markUnread(activeStory.id).then((updated) => {
              patchStory(updated);
            });
          } else if (active) {
            void backend.articles.markUnread(active.id).then((updated) => {
              patchArticle(updated);
              void loadFeeds();
            });
          }
          break;
        case "s":
          if (isStoriesMode && activeStory) {
            void backend.stories.toggleStar(activeStory.id).then((updated) => {
              patchStory(updated);
            });
          } else if (active) {
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
  }, [
    view,
    active,
    activeStory,
    isStoriesMode,
    backend,
    refreshAll,
    loadFeeds,
    patchArticle,
    patchStory,
    moveSelection,
  ]);

  const addFeed = async () => {
    setBusy(true);
    setError(null);
    try {
      await backend.feeds.add(addUrl.trim());
      setShowAdd(false);
      setAddUrl("");
      await loadFeeds();
      reloadContent(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not add feed");
    } finally {
      setBusy(false);
    }
  };

  if (view === "settings" && settings) {
    return (
      <SettingsPage
        backend={backend}
        settings={settings}
        onSettings={setSettings}
        onClose={() => setView("reader")}
        applyTheme={applyTheme}
      />
    );
  }

  const totalUnread = visibleFeeds.reduce((n, f) => n + f.unreadCount, 0);
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
          disabled={isStoriesMode || selected.type === "readLater"}
        />
        <button className="btn" onClick={() => void refreshAll()} disabled={busy}>
          {busy ? "Refreshing…" : "Refresh"}
        </button>
        <button className="btn" onClick={() => setView("settings")}>
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
          <button
            className={`nav-item ${selected.type === "stories" ? "active" : ""}`}
            onClick={() => setSelected({ type: "stories" })}
          >
            <span>Stories</span>
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

          <div className="section-label">Read later</div>
          <button
            className={`nav-item ${selected.type === "readLater" ? "active" : ""}`}
            onClick={() => setSelected({ type: "readLater" })}
          >
            <span>Read later</span>
          </button>
          <button className="nav-item" onClick={() => void addReadLaterLink()}>
            + Add link
          </button>

          <div className="section-label">Feeds</div>
          {visibleFeeds.length === 0 && (
            <div className="empty" style={{ height: "auto", padding: 12 }}>
              No feeds yet
            </div>
          )}
          {visibleFeeds.map((feed) => (
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
          {isStoriesMode ? (
            stories.length === 0 ? (
              <div className="empty">
                <h2>No stories yet</h2>
                <p>Enable AI triage in Settings to cluster related articles.</p>
              </div>
            ) : (
              stories.map((story) => (
                <button
                  key={story.id}
                  className={`article-row ${story.id === activeStoryId ? "active" : ""} ${story.isRead ? "" : "unread"}`}
                  onClick={() => void selectStory(story)}
                >
                  <div className="article-meta">
                    <span>{story.memberCount} article{story.memberCount === 1 ? "" : "s"}</span>
                    <span>{formatRelativeTime(story.updatedAt ?? story.createdAt)}</span>
                    {story.isStarred ? <span>★</span> : null}
                  </div>
                  <h3 className="article-title">{story.title || "(untitled story)"}</h3>
                  <p className="article-summary">{story.summary || ""}</p>
                </button>
              ))
            )
          ) : articles.length === 0 ? (
            <div className="empty">
              <h2>Nothing here</h2>
              <p>
                {selected.type === "readLater"
                  ? "Save a link with + Add link in the sidebar."
                  : "Add a feed or widen your filters."}
              </p>
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
                <h3 className="article-title">
                  <PriorityBadge priority={article.priority} />
                  {article.title || "(untitled)"}
                </h3>
                <p className="article-summary">{stripHtml(article.summary || article.content)}</p>
              </button>
            ))
          )}
          {!isStoriesMode && nextCursor && (
            <button className="btn" style={{ width: "100%", marginTop: 8 }} onClick={() => void loadArticles(true)}>
              Load more
            </button>
          )}
        </section>

        <section className="pane reader-pane">
          {isStoriesMode ? (
            !activeStory ? (
              <div className="empty">
                <h2>Stories</h2>
                <p>Select a story to read grouped coverage. Shortcuts: j/k, r, u, s, f</p>
              </div>
            ) : (
              <article className="reader">
                <div className="reader-kicker">
                  {activeStory.memberCount} article{activeStory.memberCount === 1 ? "" : "s"}
                  {activeStory.updatedAt
                    ? ` · ${new Date(activeStory.updatedAt).toLocaleString()}`
                    : ""}
                </div>
                <h1>{activeStory.title || "(untitled story)"}</h1>
                {activeStory.summary ? <p className="article-summary">{activeStory.summary}</p> : null}
                <div className="reader-actions">
                  <button
                    className="btn"
                    onClick={() =>
                      void backend.stories[activeStory.isRead ? "markUnread" : "markRead"](activeStory.id).then(
                        (updated) => {
                          patchStory(updated);
                        },
                      )
                    }
                  >
                    {activeStory.isRead ? "Mark unread" : "Mark read"}
                  </button>
                  <button
                    className="btn"
                    onClick={() =>
                      void backend.stories.toggleStar(activeStory.id).then((updated) => {
                        patchStory(updated);
                      })
                    }
                  >
                    {activeStory.isStarred ? "Unstar" : "Star"}
                  </button>
                </div>
                {activeStory.articles && activeStory.articles.length > 0 ? (
                  <div className="story-members">
                    {activeStory.articles.map((member) => {
                      const expanded = expandedMemberIds[member.id] === true;
                      return (
                        <div key={member.id} className="story-member">
                          <button
                            type="button"
                            className="story-member-head"
                            onClick={() => toggleMemberExpand(member.id)}
                          >
                            <span>{expanded ? "▾" : "▸"}</span>
                            <span>
                              {member.feedTitle || "Feed"}
                              {" · "}
                              {member.title || "(untitled)"}
                              {priorityBadgeLabel(member.priority) ? (
                                <>
                                  {" · "}
                                  <PriorityBadge priority={member.priority} />
                                </>
                              ) : null}
                            </span>
                          </button>
                          {expanded ? (
                            <div
                              className="story-member-body"
                              dangerouslySetInnerHTML={{
                                __html: sanitizeArticleHtml(
                                  member.content || member.summary || "<p>No content</p>",
                                ),
                              }}
                            />
                          ) : null}
                        </div>
                      );
                    })}
                  </div>
                ) : null}
              </article>
            )
          ) : !active ? (
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
              {renderContentTabs(active)}
              <h1>
                <PriorityBadge priority={active.priority} />
                {active.title || "(untitled)"}
              </h1>
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
              {renderContentBody(active)}
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
    </div>
  );
}
