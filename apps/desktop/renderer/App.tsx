import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent as ReactDragEvent } from "react";
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
import { formatRelativeTime, sanitizeArticleHtml, stripHtml, decodeHtmlEntities } from "./lib/html";
import {
  extractDropText,
  isEditableDropTarget,
  normalizeDroppedUrl,
} from "./lib/droppedUrl";
import {
  FEED_DRAG_MIME,
  feedIdFromDropData,
  feedsForFolder,
  feedsNotInFolder,
  folderNameWithUnread,
  folderUnreadCount,
  isFeedDragTypes,
  isFolderCollapsed,
  mergeFolderMemberships,
  normalizeFolderName,
  toggleCollapsedFolder,
  unassignedFeeds,
  withFeedAssigned,
  withFeedUnassigned,
  withFolderExpanded,
} from "./lib/folders";
import {
  adjacentStoryListRow,
  memberArticle,
  nextStoryVote,
  storyListRowKey,
  storyListRows,
  upsertStoryInPlace,
} from "./lib/stories";
import { SettingsPage } from "./views/SettingsPage";
import { ReadLaterView } from "./views/ReadLaterView";
import { SportsView } from "./views/SportsView";
import { PageFrame } from "./components/PageFrame";
import { ReaderBody } from "./components/ReaderBody";
import { isFullBleedTab, type ContentTab } from "./lib/readerMode";

type Selection =
  | { type: "all" }
  | { type: "unread" }
  | { type: "starred" }
  | { type: "stories" }
  | { type: "feed"; id: string }
  | { type: "folder"; id: string };

type AppMode = "rss" | "readLater" | "sports";
type View = "reader" | "settings";
type SettingsSection = "general" | "feeds" | "ai" | "sports";

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
  const [rlSearch, setRlSearch] = useState("");
  const [rlAddUrl, setRlAddUrl] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [showAdd, setShowAdd] = useState(false);
  const [addUrl, setAddUrl] = useState("");
  const [showFolderCreate, setShowFolderCreate] = useState(false);
  const [folderName, setFolderName] = useState("");
  const [assignFolderId, setAssignFolderId] = useState<string | null>(null);
  const [assignFeedId, setAssignFeedId] = useState("");
  const [dropModal, setDropModal] = useState<{ attempted: string; draft: string; error: string | null } | null>(
    null,
  );
  const [toast, setToast] = useState<{ message: string; undoId: string } | null>(null);
  const toastTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [view, setView] = useState<View>("reader");
  const [settingsSection, setSettingsSection] = useState<SettingsSection>("general");
  const [appMode, setAppMode] = useState<AppMode>("rss");
  const [readLaterFocusId, setReadLaterFocusId] = useState<string | null>(null);
  const [storyMemberId, setStoryMemberId] = useState<string | null>(null);
  const [contentTab, setContentTab] = useState<ContentTab>("primary");
  const [contentBusy, setContentBusy] = useState(false);
  const foldersRef = useRef<Folder[]>([]);
  const [collapsedFolderIds, setCollapsedFolderIds] = useState<Set<string>>(() => new Set());

  const isStoriesMode = selected.type === "stories";
  const storyMember = isStoriesMode ? memberArticle(activeStory, storyMemberId) : null;
  const active = isStoriesMode ? storyMember : (articles.find((a) => a.id === activeId) ?? null);
  const storyRows = isStoriesMode ? storyListRows(stories, activeStory) : [];

  const applyTheme = useCallback((theme: Settings["theme"]) => {
    const root = document.documentElement;
    if (theme === "system") {
      const dark = window.matchMedia("(prefers-color-scheme: dark)").matches;
      root.dataset.theme = dark ? "dark" : "light";
    } else {
      root.dataset.theme = theme;
    }
  }, []);

  const loadFeeds = useCallback(async (change?: { type: "assign" | "unassign"; folderId: string; feedId: string }) => {
    const [f, foldersList, s] = await Promise.all([
      backend.feeds.list(),
      backend.folders.list(),
      backend.settings.get(),
    ]);
    let nextFolders = mergeFolderMemberships(foldersList ?? [], foldersRef.current);
    if (change) {
      switch (change.type) {
        case "assign":
          nextFolders = withFeedAssigned(nextFolders, change.folderId, change.feedId);
          break;
        case "unassign":
          nextFolders = withFeedUnassigned(nextFolders, change.folderId, change.feedId);
          break;
        default: {
          const _exhaustive: never = change.type;
          return _exhaustive;
        }
      }
    }
    setFeeds(f ?? []);
    setFolders(nextFolders);
    foldersRef.current = nextFolders;
    setSettings(s);
    applyTheme(s.theme);
    return nextFolders;
  }, [backend, applyTheme]);

  const loadArticles = useCallback(
    async (append = false) => {
      if (selected.type === "stories") return;
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
      if (!isStoriesMode) {
        setActiveStory(null);
        setStoryMemberId(null);
      }
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
        case "article.removed": {
          const payload = event.payload as { articleId?: string };
          if (payload.articleId) {
            setArticles((prev) => prev.filter((a) => a.id !== payload.articleId));
            setActiveId((id) => (id === payload.articleId ? null : id));
          }
          break;
        }
        case "story.updated": {
          const payload = event.payload as { storyId?: string };
          if (!payload.storyId) {
            void loadStories();
            break;
          }
          void backend.stories
            .get(payload.storyId)
            .then((full) => {
              setStories((prev) => upsertStoryInPlace(prev, full));
              if (payload.storyId === activeStoryId) {
                setActiveStory((prev) =>
                  prev?.id === full.id
                    ? { ...prev, ...full, articles: prev.articles, articleIds: prev.articleIds }
                    : full,
                );
              }
            })
            .catch(() => {
              void loadStories();
            });
          break;
        }
        case "ai.status":
          break;
        case "ai.log":
          break;
        case "sync.status":
          break;
        case "sports.game.updated":
          break;
        case "sports.f1.race.updated":
          break;
        case "sports.refresh":
          break;
        case "sports.cache.updated":
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
    setActiveStory((prev) => {
      if (!prev?.articles) return prev;
      return {
        ...prev,
        articles: prev.articles.map((a) => (a.id === updated.id ? { ...a, ...updated } : a)),
      };
    });
  }, []);

  const patchStory = useCallback((updated: Story) => {
    setStories((prev) => prev.map((s) => (s.id === updated.id ? { ...s, ...updated } : s)));
    setActiveStory((prev) =>
      prev?.id === updated.id
        ? { ...prev, ...updated, articles: prev.articles, articleIds: prev.articleIds }
        : prev,
    );
  }, []);

  const refreshStoriesAfterVote = useCallback(
    async (updated: Story) => {
      await loadStories();
      if (updated.memberCount >= 2) {
        try {
          const full = await backend.stories.get(updated.id);
          setActiveStoryId(updated.id);
          setActiveStory(full);
        } catch {
          setActiveStory(null);
        }
        return;
      }
      if (activeStoryId === updated.id) {
        setStoryMemberId(null);
      }
    },
    [backend, loadStories, activeStoryId],
  );

  const voteStoryArticle = useCallback(
    async (storyId: string, articleId: string, clicked: "up" | "down") => {
      const current = activeStory?.id === storyId ? activeStory.articleVotes?.[articleId] : undefined;
      try {
        const updated = await backend.stories.voteArticle(storyId, articleId, nextStoryVote(current, clicked));
        await refreshStoriesAfterVote(updated);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to vote");
      }
    },
    [backend, activeStory, refreshStoriesAfterVote],
  );

  const voteActiveStory = useCallback(
    async (clicked: "up" | "down") => {
      if (!activeStory) return;
      try {
        const updated = await backend.stories.voteStory(activeStory.id, nextStoryVote(activeStory.vote, clicked));
        patchStory(updated);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to vote");
      }
    },
    [backend, activeStory, patchStory],
  );

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
      setStoryMemberId(null);
      setActiveId(null);
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

  const selectStoryMember = useCallback(
    async (article: Article) => {
      setStoryMemberId(article.id);
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
      if (isStoriesMode) {
        const currentKey = storyMemberId
          ? storyListRowKey({ kind: "member", storyId: activeStoryId ?? "", articleId: storyMemberId })
          : activeStoryId
            ? storyListRowKey({ kind: "story", storyId: activeStoryId })
            : null;
        const next = adjacentStoryListRow(storyRows, currentKey, delta);
        if (!next) return;
        switch (next.kind) {
          case "story": {
            const story = stories.find((s) => s.id === next.storyId);
            if (story && (story.id !== activeStoryId || storyMemberId)) {
              void selectStory(story);
            }
            break;
          }
          case "member": {
            const member = memberArticle(activeStory, next.articleId);
            if (member && member.id !== storyMemberId) {
              void selectStoryMember(member);
            }
            break;
          }
          default: {
            const _exhaustive: never = next;
            return _exhaustive;
          }
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
    [
      isStoriesMode,
      storyRows,
      storyMemberId,
      activeStoryId,
      stories,
      activeStory,
      selectStory,
      selectStoryMember,
      articles,
      activeId,
      selectArticle,
    ],
  );

  useEffect(() => {
    setContentTab("primary");
  }, [activeId]);

  const addReadLaterFromActive = async () => {
    if (!active || active.isReadLater || !active.url) return;
    setBusy(true);
    setError(null);
    try {
      const saved = await backend.readLater.addFromArticle(active.id);
      setReadLaterFocusId(saved.id);
      setAppMode("readLater");
      setView("reader");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not send to Read Later");
    } finally {
      setBusy(false);
    }
  };

  const addReadLaterUrl = async () => {
    const url = rlAddUrl.trim();
    if (!url) return;
    setBusy(true);
    setError(null);
    try {
      const saved = await backend.readLater.add(url);
      setRlAddUrl("");
      setRlSearch("");
      setReadLaterFocusId(saved.id);
      setAppMode("readLater");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not save link");
    } finally {
      setBusy(false);
    }
  };

  const clearToastTimer = useCallback(() => {
    if (toastTimerRef.current) {
      clearTimeout(toastTimerRef.current);
      toastTimerRef.current = null;
    }
  }, []);

  const showSavedToast = useCallback(
    (articleId: string) => {
      clearToastTimer();
      setToast({ message: "Saved to Read Later", undoId: articleId });
      toastTimerRef.current = setTimeout(() => {
        setToast(null);
        toastTimerRef.current = null;
      }, 7000);
    },
    [clearToastTimer],
  );

  const saveDroppedUrl = useCallback(
    async (url: string) => {
      const saved = await backend.readLater.add(url);
      showSavedToast(saved.id);
      return saved;
    },
    [backend, showSavedToast],
  );

  const openInvalidDropModal = useCallback(async (attempted: string) => {
    try {
      await window.desktop.focusMainWindow();
    } catch {
      // browser / missing bridge
    }
    setDropModal({ attempted, draft: attempted, error: null });
  }, []);

  const handleDroppedText = useCallback(
    (raw: string) => {
      const result = normalizeDroppedUrl(raw);
      if (result.ok) {
        void saveDroppedUrl(result.url).catch((e) => {
          setError(e instanceof Error ? e.message : "Could not save link");
        });
        return;
      }
      void openInvalidDropModal(result.attempted || raw.trim());
    },
    [openInvalidDropModal, saveDroppedUrl],
  );

  const undoDroppedSave = useCallback(async () => {
    if (!toast) return;
    const id = toast.undoId;
    clearToastTimer();
    setToast(null);
    try {
      await backend.readLater.remove(id);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not undo");
    }
  }, [backend, clearToastTimer, toast]);

  const submitDropModal = useCallback(async () => {
    if (!dropModal) return;
    const result = normalizeDroppedUrl(dropModal.draft);
    if (!result.ok) {
      setDropModal((prev) =>
        prev ? { ...prev, error: "Enter a valid http(s) URL" } : prev,
      );
      return;
    }
    setBusy(true);
    try {
      await saveDroppedUrl(result.url);
      setDropModal(null);
    } catch (e) {
      setDropModal((prev) =>
        prev
          ? { ...prev, error: e instanceof Error ? e.message : "Could not save link" }
          : prev,
      );
    } finally {
      setBusy(false);
    }
  }, [dropModal, saveDroppedUrl]);

  useEffect(() => {
    const onDragOver = (e: DragEvent) => {
      if (!e.dataTransfer) return;
      if (isFeedDragTypes(e.dataTransfer.types)) return;
      if (isEditableDropTarget(e.target)) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = "copy";
    };
    const onDrop = (e: DragEvent) => {
      if (!e.dataTransfer) return;
      if (isFeedDragTypes(e.dataTransfer.types)) return;
      if (isEditableDropTarget(e.target)) return;
      const text = extractDropText(e.dataTransfer);
      if (!text.trim()) return;
      e.preventDefault();
      handleDroppedText(text);
    };
    window.addEventListener("dragover", onDragOver);
    window.addEventListener("drop", onDrop);
    const unsub =
      typeof window.desktop?.onDroppedText === "function"
        ? window.desktop.onDroppedText(handleDroppedText)
        : () => undefined;
    return () => {
      window.removeEventListener("dragover", onDragOver);
      window.removeEventListener("drop", onDrop);
      unsub();
      clearToastTimer();
    };
  }, [clearToastTimer, handleDroppedText]);

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
          className={`content-tab ${contentTab === "reader" ? "active" : ""}`}
          onClick={() => void handleContentTab("reader")}
        >
          Reader
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
    if (contentTab === "reader") {
      return (
        <ReaderBody
          article={article}
          contentBusy={contentBusy}
          onRecrawl={() => void recrawlActive()}
        />
      );
    }

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
          <PageFrame
            html={bodyHtml}
            pageUrl={article.url}
            title={decodeHtmlEntities(article.title || "Article page")}
          />
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
  const sidebarFeeds = unassignedFeeds(feeds, folders);
  const selectedFolder = selected.type === "folder" ? (folders.find((folder) => folder.id === selected.id) ?? null) : null;
  const assignFolder = folders.find((folder) => folder.id === assignFolderId) ?? null;
  const assignableFeeds = assignFolder ? feedsNotInFolder(feeds, assignFolder) : [];

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
      if (appMode === "readLater") {
        if (e.key === "/") {
          e.preventDefault();
          document.getElementById("rl-search-input")?.focus();
        }
        return;
      }
      if (appMode === "sports") return;

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
          if (isStoriesMode && !storyMemberId && activeStory) {
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
          if (isStoriesMode && !storyMemberId && activeStory) {
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
          if (isStoriesMode && !storyMemberId && activeStory) {
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
    appMode,
    active,
    activeStory,
    isStoriesMode,
    backend,
    refreshAll,
    loadFeeds,
    patchArticle,
    patchStory,
    moveSelection,
    storyMemberId,
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

  const createFolder = async () => {
    const name = normalizeFolderName(folderName);
    if (!name) return;
    setBusy(true);
    setError(null);
    try {
      const created = await backend.folders.create(name);
      setShowFolderCreate(false);
      setFolderName("");
      await loadFeeds();
      setSelected({ type: "folder", id: created.id });
      setCollapsedFolderIds((prev) => withFolderExpanded(prev, created.id));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not create folder");
    } finally {
      setBusy(false);
    }
  };

  const assignFeedToFolder = async (folderId: string, feedId: string) => {
    if (!folderId || !feedId) return false;
    setBusy(true);
    setError(null);
    try {
      await backend.folders.assignFeed(folderId, feedId);
      await loadFeeds({ type: "assign", folderId, feedId });
      setCollapsedFolderIds((prev) => withFolderExpanded(prev, folderId));
      return true;
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not add feed to folder");
      return false;
    } finally {
      setBusy(false);
    }
  };

  const unassignFeedFromFolder = async (folderId: string, feedId: string) => {
    setBusy(true);
    setError(null);
    try {
      await backend.folders.unassignFeed(folderId, feedId);
      await loadFeeds({ type: "unassign", folderId, feedId });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not remove feed from folder");
    } finally {
      setBusy(false);
    }
  };

  const openAssignModal = (folderId: string) => {
    const folder = folders.find((item) => item.id === folderId);
    if (!folder) return;
    const options = feedsNotInFolder(feeds, folder);
    setError(null);
    setAssignFolderId(folderId);
    setAssignFeedId(options[0]?.id ?? "");
    setCollapsedFolderIds((prev) => withFolderExpanded(prev, folderId));
  };

  const onFeedDragStart = (event: ReactDragEvent, feedId: string) => {
    event.dataTransfer.setData(FEED_DRAG_MIME, feedId);
    event.dataTransfer.effectAllowed = "copyMove";
  };

  const onFolderDragOver = (event: ReactDragEvent) => {
    if (!isFeedDragTypes(event.dataTransfer.types)) return;
    event.preventDefault();
    event.stopPropagation();
    event.dataTransfer.dropEffect = "move";
  };

  const onFolderDrop = (event: ReactDragEvent, folderId: string) => {
    if (!isFeedDragTypes(event.dataTransfer.types)) return;
    event.preventDefault();
    event.stopPropagation();
    const feedId = feedIdFromDropData((type) => event.dataTransfer.getData(type));
    if (!feedId) return;
    void assignFeedToFolder(folderId, feedId);
  };

  const totalUnread = visibleFeeds.reduce((n, f) => n + f.unreadCount, 0);
  const densityClass = settings?.articleDensity === "compact" ? "density-compact" : "";

  const dropFixModal = dropModal ? (
    <div className="modal-backdrop" onClick={() => setDropModal(null)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2>Fix link for Read Later</h2>
        <p className="modal-hint">That drop wasn’t a valid URL. Edit it and try again.</p>
        <label className="modal-label" htmlFor="drop-attempted">
          Attempted
        </label>
        <pre id="drop-attempted" className="modal-attempted">
          {dropModal.attempted || "(empty)"}
        </pre>
        <label className="modal-label" htmlFor="drop-url">
          URL
        </label>
        <input
          id="drop-url"
          autoFocus
          placeholder="https://example.com/article"
          value={dropModal.draft}
          onChange={(e) =>
            setDropModal((prev) =>
              prev ? { ...prev, draft: e.target.value, error: null } : prev,
            )
          }
          onKeyDown={(e) => {
            if (e.key === "Enter") void submitDropModal();
          }}
        />
        {dropModal.error ? <p className="error">{dropModal.error}</p> : null}
        <div className="modal-actions">
          <button type="button" className="btn" onClick={() => setDropModal(null)}>
            Cancel
          </button>
          <button
            type="button"
            className="btn primary"
            disabled={busy || !dropModal.draft.trim()}
            onClick={() => void submitDropModal()}
          >
            Add
          </button>
        </div>
      </div>
    </div>
  ) : null;

  const saveToast = toast ? (
    <div className="toast" role="status">
      <span>{toast.message}</span>
      <button type="button" className="toast-undo" onClick={() => void undoDroppedSave()}>
        Undo
      </button>
    </div>
  ) : null;

  if (view === "settings" && settings) {
    return (
      <>
        <SettingsPage
          backend={backend}
          settings={settings}
          onSettings={setSettings}
          onClose={() => {
            setView("reader");
            setSettingsSection("general");
          }}
          applyTheme={applyTheme}
          initialSection={settingsSection}
        />
        {dropFixModal}
        {saveToast}
      </>
    );
  }
  return (
    <div className={`app ${densityClass}`}>
      <header className="toolbar">
        <div className="mode-tabs" role="tablist" aria-label="App mode">
          <button
            type="button"
            role="tab"
            className={`mode-tab ${appMode === "rss" ? "active" : ""}`}
            aria-selected={appMode === "rss"}
            onClick={() => setAppMode("rss")}
          >
            RSS Reader
          </button>
          <button
            type="button"
            role="tab"
            className={`mode-tab ${appMode === "readLater" ? "active" : ""}`}
            aria-selected={appMode === "readLater"}
            onClick={() => setAppMode("readLater")}
          >
            Read Later
          </button>
          <button
            type="button"
            role="tab"
            className={`mode-tab ${appMode === "sports" ? "active" : ""}`}
            aria-selected={appMode === "sports"}
            onClick={() => setAppMode("sports")}
          >
            Sports
          </button>
        </div>
        <span className={`status-dot ${error ? "error" : ""}`} title={error ?? "Connected"} />
        <div className="toolbar-spacer" />
        {appMode === "rss" ? (
          <input
            id="search-input"
            className="search"
            placeholder="Search articles  (/)"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            disabled={isStoriesMode}
          />
        ) : appMode === "readLater" ? (
          <input
            id="rl-search-input"
            className="search"
            placeholder="Search saved links  (/)"
            value={rlSearch}
            onChange={(e) => setRlSearch(e.target.value)}
          />
        ) : null}
        {appMode === "rss" ? (
          <button className="btn" onClick={() => void refreshAll()} disabled={busy}>
            {busy ? "Refreshing…" : "Refresh"}
          </button>
        ) : appMode === "readLater" ? (
          <form
            className="rl-add-toolbar"
            onSubmit={(e) => {
              e.preventDefault();
              void addReadLaterUrl();
            }}
          >
            <input
              className="search rl-url-input"
              placeholder="https://… paste a URL"
              value={rlAddUrl}
              onChange={(e) => setRlAddUrl(e.target.value)}
              disabled={busy}
            />
            <button className="btn primary" type="submit" disabled={busy || !rlAddUrl.trim()}>
              {busy ? "Adding…" : "Add"}
            </button>
          </form>
        ) : null}
        <button
          className="btn"
          onClick={() => {
            setSettingsSection("general");
            setView("settings");
          }}
        >
          Settings
        </button>
        {appMode === "rss" && (
          <button className="btn primary" onClick={() => setShowAdd(true)}>
            Add feed
          </button>
        )}
      </header>

      {error && <p className="error toolbar-error">{error}</p>}

      {appMode === "readLater" ? (
        <ReadLaterView
          backend={backend}
          search={rlSearch}
          focusArticleId={readLaterFocusId}
          onFocusConsumed={() => setReadLaterFocusId(null)}
        />
      ) : appMode === "sports" ? (
        <SportsView
          backend={backend}
          onOpenSettingsSports={() => {
            setSettingsSection("sports");
            setView("settings");
          }}
        />
      ) : (
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

          <div className="section-label-row">
            <div className="section-label">Folders</div>
            <button
              type="button"
              className="section-add"
              aria-label="New folder"
              title="New folder"
              onClick={() => {
                setError(null);
                setFolderName("");
                setShowFolderCreate(true);
              }}
            >
              +
            </button>
          </div>
          {folders.map((folder) => {
            const nestedFeeds = feedsForFolder(feeds, folder);
            const canAdd = feedsNotInFolder(feeds, folder).length > 0;
            const collapsed = isFolderCollapsed(collapsedFolderIds, folder.id);
            return (
              <div key={folder.id} onDragOver={onFolderDragOver} onDrop={(e) => onFolderDrop(e, folder.id)}>
                <div
                  className={`folder-item ${selected.type === "folder" && selected.id === folder.id ? "active" : ""}`}
                >
                  <button
                    type="button"
                    className="folder-item-chevron"
                    aria-expanded={!collapsed}
                    aria-label={collapsed ? `Expand ${folder.name}` : `Collapse ${folder.name}`}
                    onClick={() => setCollapsedFolderIds((prev) => toggleCollapsedFolder(prev, folder.id))}
                  >
                    {collapsed ? "▸" : "▾"}
                  </button>
                  <button
                    type="button"
                    className="folder-item-name"
                    onClick={() => {
                      setSelected({ type: "folder", id: folder.id });
                      setCollapsedFolderIds((prev) => withFolderExpanded(prev, folder.id));
                    }}
                    title="Drop a feed here to add it to this folder"
                  >
                    {folderNameWithUnread(folder.name, folderUnreadCount(feeds, folder))}
                  </button>
                  <button
                    type="button"
                    className="section-add"
                    aria-label={`Add feed to ${folder.name}`}
                    title="Add feed to folder"
                    disabled={!canAdd || busy}
                    onClick={() => openAssignModal(folder.id)}
                  >
                    +
                  </button>
                </div>
                {!collapsed &&
                  nestedFeeds.map((feed) => (
                    <div
                      key={`${folder.id}-${feed.id}`}
                      className="feed-drag"
                      draggable
                      onDragStart={(e) => onFeedDragStart(e, feed.id)}
                    >
                      <button
                        type="button"
                        className={`nav-item nav-item-nested ${selected.type === "feed" && selected.id === feed.id ? "active" : ""}`}
                        onClick={() => setSelected({ type: "feed", id: feed.id })}
                        onContextMenu={(e) => {
                          e.preventDefault();
                          void unassignFeedFromFolder(folder.id, feed.id);
                        }}
                        title={feed.lastError || feed.url}
                      >
                        <span>
                          {!feed.enabled ? "⏸ " : feed.lastError ? "⚠ " : ""}
                          {feed.title || feed.url}
                        </span>
                        <span className="count">{feed.unreadCount || ""}</span>
                      </button>
                    </div>
                  ))}
              </div>
            );
          })}

          <div className="section-label">Feeds</div>
          {sidebarFeeds.length === 0 && (
            <div className="empty" style={{ height: "auto", padding: 12 }}>
              {visibleFeeds.length === 0 ? "No feeds yet" : "All feeds are in folders"}
            </div>
          )}
          {sidebarFeeds.map((feed) => (
            <div
              key={feed.id}
              className="feed-drag"
              draggable
              onDragStart={(e) => onFeedDragStart(e, feed.id)}
            >
              <button
                type="button"
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
            </div>
          ))}
        </aside>

        <section className="pane article-list">
          {isStoriesMode ? (
            stories.length === 0 ? (
              <div className="empty">
                <h2>No stories yet</h2>
                <p>Related RSS articles group here after feeds refresh. AI triage can still override groups when enabled.</p>
              </div>
            ) : (
              storyRows.map((row) => {
                switch (row.kind) {
                  case "story": {
                    const story = stories.find((s) => s.id === row.storyId);
                    if (!story) return null;
                    const isActive = story.id === activeStoryId && !storyMemberId;
                    return (
                      <button
                        key={storyListRowKey(row)}
                        className={`article-row ${isActive ? "active" : ""} ${story.isRead ? "" : "unread"}`}
                        onClick={() => void selectStory(story)}
                      >
                        <div className="article-meta">
                          <span>
                            {story.memberCount} article{story.memberCount === 1 ? "" : "s"}
                          </span>
                          <span>{formatRelativeTime(story.updatedAt ?? story.createdAt)}</span>
                          {story.isStarred ? <span>★</span> : null}
                        </div>
                        <h3 className="article-title">{decodeHtmlEntities(story.title || "(untitled story)")}</h3>
                        <p className="article-summary">{story.summary || ""}</p>
                      </button>
                    );
                  }
                  case "member": {
                    const member = memberArticle(activeStory, row.articleId);
                    if (!member || !activeStory) return null;
                    const memberVote = activeStory.articleVotes?.[member.id];
                    return (
                      <div
                        key={storyListRowKey(row)}
                        className={`article-row story-member-row ${member.id === storyMemberId ? "active" : ""} ${member.isRead ? "" : "unread"}`}
                      >
                        <button
                          type="button"
                          className="story-member-main"
                          onClick={() => void selectStoryMember(member)}
                        >
                          <div className="article-meta">
                            <span>{member.feedTitle}</span>
                            <span>{formatRelativeTime(member.publishedAt ?? member.discoveredAt)}</span>
                            {member.isStarred ? <span>★</span> : null}
                          </div>
                          <h3 className="article-title">
                            <PriorityBadge priority={member.priority} />
                            {decodeHtmlEntities(member.title || "(untitled)")}
                          </h3>
                          <p className="article-summary">{stripHtml(member.summary || member.content)}</p>
                        </button>
                        <div className="story-votes">
                          <button
                            type="button"
                            className={`story-vote ${memberVote === "up" ? "active" : ""}`}
                            aria-label="Thumbs up"
                            aria-pressed={memberVote === "up"}
                            onClick={() => void voteStoryArticle(activeStory.id, member.id, "up")}
                          >
                            👍
                          </button>
                          <button
                            type="button"
                            className={`story-vote ${memberVote === "down" ? "active" : ""}`}
                            aria-label="Thumbs down"
                            aria-pressed={memberVote === "down"}
                            onClick={() => void voteStoryArticle(activeStory.id, member.id, "down")}
                          >
                            👎
                          </button>
                        </div>
                      </div>
                    );
                  }
                  default: {
                    const _exhaustive: never = row;
                    return _exhaustive;
                  }
                }
              })
            )
          ) : articles.length === 0 ? (
            <div className="empty">
              <h2>{selectedFolder ? "No feeds in this folder" : "Nothing here"}</h2>
              <p>
                {selectedFolder
                  ? "Use + next to the folder name to add a feed."
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
                    {decodeHtmlEntities(article.title || "(untitled)")}
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
          {isStoriesMode && !storyMemberId ? (
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
                <h1>{decodeHtmlEntities(activeStory.title || "(untitled story)")}</h1>
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
                  <button
                    type="button"
                    className={`btn story-vote ${activeStory.vote === "up" ? "active" : ""}`}
                    aria-label="Thumbs up story"
                    aria-pressed={activeStory.vote === "up"}
                    onClick={() => void voteActiveStory("up")}
                  >
                    👍
                  </button>
                  <button
                    type="button"
                    className={`btn story-vote ${activeStory.vote === "down" ? "active" : ""}`}
                    aria-label="Thumbs down story"
                    aria-pressed={activeStory.vote === "down"}
                    onClick={() => void voteActiveStory("down")}
                  >
                    👎
                  </button>
                </div>
              </article>
            )
          ) : !active ? (
            <div className="empty">
              <h2>RSS Reader</h2>
              <p>Select an article to read. Shortcuts: j/k, o, r, u, s, f, /</p>
            </div>
          ) : (
            <article
              className={`reader ${isFullBleedTab(contentTab, active.isReadLater) ? "reader-fullbleed" : ""}`}
            >
              <div className="reader-toolbar">
                {renderContentTabs(active)}
                <div className="reader-actions">
                  <button
                    className="btn"
                    onClick={() =>
                      void backend.articles[active.isRead ? "markUnread" : "markRead"](active.id).then(
                        (updated) => {
                          patchArticle(updated);
                          void loadFeeds();
                        },
                      )
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
                  {!active.isReadLater && active.url ? (
                    <button className="btn" disabled={busy} onClick={() => void addReadLaterFromActive()}>
                      Send to Read Later
                    </button>
                  ) : null}
                  {active.url && (
                    <button className="btn primary" onClick={() => void window.desktop.openExternal(active.url)}>
                      Open original
                    </button>
                  )}
                  {(contentTab === "secondary" || contentTab === "reader") && (
                    <button className="btn" disabled={contentBusy} onClick={() => void recrawlActive()}>
                      {contentBusy ? "Re-crawling…" : "Re-crawl page"}
                    </button>
                  )}
                </div>
              </div>
              {contentTab === "reader" || (contentTab === "primary" && !active.isReadLater) ? (
                <h1>
                  <PriorityBadge priority={active.priority} />
                  {decodeHtmlEntities(active.title || "(untitled)")}
                </h1>
              ) : null}
              {renderContentBody(active)}
            </article>
          )}
        </section>
      </div>
      )}

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

      {showFolderCreate && (
        <div
          className="modal-backdrop"
          onClick={() => {
            setShowFolderCreate(false);
            setError(null);
          }}
        >
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h2>New folder</h2>
            {error && <p className="error">{error}</p>}
            <input
              autoFocus
              placeholder="Folder name"
              value={folderName}
              onChange={(e) => setFolderName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") void createFolder();
              }}
            />
            <div className="modal-actions">
              <button
                type="button"
                className="btn"
                onClick={() => {
                  setShowFolderCreate(false);
                  setError(null);
                }}
              >
                Cancel
              </button>
              <button
                type="button"
                className="btn primary"
                disabled={busy || !normalizeFolderName(folderName)}
                onClick={() => void createFolder()}
              >
                Create
              </button>
            </div>
          </div>
        </div>
      )}

      {assignFolder && (
        <div
          className="modal-backdrop"
          onClick={() => {
            setAssignFolderId(null);
            setError(null);
          }}
        >
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h2>Add feed to {assignFolder.name}</h2>
            {error && <p className="error">{error}</p>}
            {assignableFeeds.length === 0 ? (
              <p className="modal-hint">All feeds are already in this folder.</p>
            ) : (
              <select
                autoFocus
                value={assignFeedId}
                onChange={(e) => setAssignFeedId(e.target.value)}
              >
                {assignableFeeds.map((feed) => (
                  <option key={feed.id} value={feed.id}>
                    {feed.title || feed.url}
                  </option>
                ))}
              </select>
            )}
            <div className="modal-actions">
              <button
                type="button"
                className="btn"
                onClick={() => {
                  setAssignFolderId(null);
                  setError(null);
                }}
              >
                Cancel
              </button>
              <button
                type="button"
                className="btn primary"
                disabled={busy || !assignFeedId || assignableFeeds.length === 0}
                onClick={() => {
                  void assignFeedToFolder(assignFolder.id, assignFeedId).then((ok) => {
                    if (ok) setAssignFolderId(null);
                  });
                }}
              >
                Add
              </button>
            </div>
          </div>
        </div>
      )}

      {dropFixModal}
      {saveToast}
    </div>
  );
}
