import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent as ReactDragEvent } from "react";
import type {
  Article,
  ArticleQuery,
  BackendEventName,
  Feed,
  Folder,
  Priority,
  ReaderBackend,
  Settings,
  Story,
} from "@rss-reader/shared";
import { getBackend } from "./lib/backend";
import { formatRelativeTime, sanitizeArticleHtml, stripHtml, decodeHtmlEntities } from "./lib/html";
import { normalizeDroppedUrl } from "./lib/droppedUrl";
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
  adjacentMetaStory,
  adjacentStoryListRow,
  hasNoOtherUnreadStoryMembers,
  isListableStory,
  memberArticle,
  nextStoryVote,
  storyListRowKey,
  storyListRows,
  storiesInSnapshot,
  unreadStorySnapshotIds,
  unreadStoryCount,
  upsertStoryInPlace,
} from "./lib/stories";
import { SettingsPage } from "./views/SettingsPage";
import { ReadLaterView } from "./views/ReadLaterView";
import { SportsView } from "./views/SportsView";
import { PageFrame } from "./components/PageFrame";
import { ReaderBody } from "./components/ReaderBody";
import { BrowserPane } from "./components/BrowserPane";
import { drainPendingExtracts, runFrontendExtract } from "./lib/extractQueue";
import { isFullBleedTab, type ContentTab } from "./lib/readerMode";
import { mlbTeamRouteFromHash } from "./lib/sportsDeepLinks";
import { browserPaneUrl } from "./lib/linkNavigation";
import { GENERIC_ERROR_MESSAGE } from "./lib/errors";
import { scrollListRowToTop } from "./lib/listScroll";
import { shouldReloadArticleListInBackground } from "./lib/listRefresh";

type Selection =
  | { type: "items" }
  | { type: "stories" }
  | { type: "feed"; id: string }
  | { type: "folder"; id: string };

type AppMode = "rss" | "readLater" | "sports";
type View = "reader" | "settings";
type SettingsSection = "server" | "general" | "feeds" | "ai" | "sports" | "errors";
type RssListFilter = "all" | "unread";

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

export function App({
  backend: providedBackend,
  serverAuthoritative = false,
}: {
  backend?: ReaderBackend;
  serverAuthoritative?: boolean;
} = {}) {
  const backend = useMemo(() => {
    if (providedBackend) return providedBackend;
    try {
      return getBackend();
    } catch {
      return null;
    }
  }, [providedBackend]);

  const [connectedAuthority, setConnectedAuthority] = useState(serverAuthoritative);
  useEffect(() => {
    if (serverAuthoritative) {
      setConnectedAuthority(true);
      return;
    }
    if (!backend?.sync) {
      setConnectedAuthority(false);
      return;
    }
    void backend.sync.status().then((status) => setConnectedAuthority(status.connected)).catch(() => undefined);
    return backend.onEvent((event) => {
      if (event.event !== "sync.status") return;
      const payload = event.payload as { connected?: boolean; phase?: string };
      if (typeof payload.connected === "boolean") setConnectedAuthority(payload.connected);
      else if (payload.phase === "connected") setConnectedAuthority(true);
      else if (payload.phase === "disconnected") setConnectedAuthority(false);
    });
  }, [backend, serverAuthoritative]);

  if (!backend) {
    return (
      <div className="app">
        <div className="empty">
          <h2>RSS Reader</h2>
          <p className="error">{GENERIC_ERROR_MESSAGE}</p>
        </div>
      </div>
    );
  }

  return <AppMain backend={backend} serverAuthoritative={serverAuthoritative || connectedAuthority} />;
}

function AppMain({ backend, serverAuthoritative }: { backend: ReaderBackend; serverAuthoritative: boolean }) {
  const [feeds, setFeeds] = useState<Feed[]>([]);
  const [folders, setFolders] = useState<Folder[]>([]);
  const [articles, setArticles] = useState<Article[]>([]);
  const [stories, setStories] = useState<Story[]>([]);
  const [unreadStorySnapshot, setUnreadStorySnapshot] = useState<string[] | null>(null);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [articleListRevision, setArticleListRevision] = useState(0);
  const [selected, setSelected] = useState<Selection>({ type: "items" });
  const [rssListFilter, setRssListFilter] = useState<RssListFilter>("unread");
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
  const articleListRef = useRef<HTMLElement | null>(null);
  const lastScrolledListRowKeyRef = useRef<string | null>(null);
  const loadMoreSentinelRef = useRef<HTMLDivElement | null>(null);
  const paginationInFlightRef = useRef(false);
  const articleListGenerationRef = useRef(0);
  const pendingArticleDeepLinkRef = useRef<Article | null>(null);
  const [view, setView] = useState<View>("reader");
  const [settingsSection, setSettingsSection] = useState<SettingsSection>("general");
  const [appMode, setAppMode] = useState<AppMode>(() =>
    mlbTeamRouteFromHash(window.location.hash) ? "sports" : "rss",
  );
  const [readLaterFocusId, setReadLaterFocusId] = useState<string | null>(null);
  const [storyMemberId, setStoryMemberId] = useState<string | null>(null);
  const [contentTab, setContentTab] = useState<ContentTab>("primary");
  const [contentBusy, setContentBusy] = useState(false);
  const [markAllBusy, setMarkAllBusy] = useState(false);
  const [browserUrl, setBrowserUrl] = useState<string | null>(null);
  const foldersRef = useRef<Folder[]>([]);
  const [collapsedFolderIds, setCollapsedFolderIds] = useState<Set<string>>(() => new Set());

  useEffect(() => {
    const openSportsDeepLink = () => {
      if (mlbTeamRouteFromHash(window.location.hash)) setAppMode("sports");
    };
    window.addEventListener("hashchange", openSportsDeepLink);
    return () => window.removeEventListener("hashchange", openSportsDeepLink);
  }, []);

  const isStoriesMode = selected.type === "stories";
  const effectiveRssListFilter = rssListFilter;
  const storyMember = isStoriesMode ? memberArticle(activeStory, storyMemberId) : null;
  const active = isStoriesMode ? storyMember : (articles.find((a) => a.id === activeId) ?? null);
  const storyUnread = unreadStoryCount(stories);
  const visibleStories =
    isStoriesMode && effectiveRssListFilter === "unread"
      ? storiesInSnapshot(stories, unreadStorySnapshot ?? unreadStorySnapshotIds(stories))
      : stories;
  const storyRows = isStoriesMode ? storyListRows(visibleStories, activeStory) : [];

  const articleScopeQuery = useMemo<ArticleQuery>(
    () => ({
      unreadOnly: effectiveRssListFilter === "unread" ? true : undefined,
      feedId: selected.type === "feed" ? selected.id : undefined,
      folderId: selected.type === "folder" ? selected.id : undefined,
      search: search.trim() || undefined,
    }),
    [effectiveRssListFilter, selected, search],
  );

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
      const generation = append
        ? articleListGenerationRef.current
        : articleListGenerationRef.current + 1;
      if (!append) {
        articleListGenerationRef.current = generation;
        paginationInFlightRef.current = false;
        setLoadingMore(false);
      }
      const query = {
        ...articleScopeQuery,
        limit: 50,
        cursor: append ? nextCursor ?? undefined : undefined,
      };
      const res = await backend.articles.list(query);
      if (generation !== articleListGenerationRef.current) return;
      let list = res.articles ?? [];
      const pendingArticle = append ? null : pendingArticleDeepLinkRef.current;
      if (pendingArticle && !list.some((article) => article.id === pendingArticle.id)) {
        list = [pendingArticle, ...list];
      }
      setArticles((prev) => (append ? [...prev, ...list] : list));
      setNextCursor(res.nextCursor);
      if (!append) {
        setArticleListRevision((revision) => revision + 1);
        setActiveId((id) => {
          if (pendingArticle) return pendingArticle.id;
          if (id && list.some((a) => a.id === id)) return id;
          return list[0]?.id ?? null;
        });
        pendingArticleDeepLinkRef.current = null;
      }
    },
    [backend, selected.type, articleScopeQuery, nextCursor],
  );

  const loadNextArticlePage = useCallback(async () => {
    if (selected.type === "stories" || !nextCursor || paginationInFlightRef.current) return;
    const generation = articleListGenerationRef.current;
    paginationInFlightRef.current = true;
    setLoadingMore(true);
    try {
      await loadArticles(true);
    } catch {
      setError(GENERIC_ERROR_MESSAGE);
    } finally {
      if (generation === articleListGenerationRef.current) {
        paginationInFlightRef.current = false;
        setLoadingMore(false);
      }
    }
  }, [selected.type, nextCursor, loadArticles]);

  useEffect(() => {
    const root = articleListRef.current;
    const sentinel = loadMoreSentinelRef.current;
    if (!root || !sentinel || isStoriesMode || !nextCursor) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          void loadNextArticlePage();
        }
      },
      { root, rootMargin: "0px 0px 80px 0px" },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [isStoriesMode, nextCursor, loadNextArticlePage]);

  const loadStories = useCallback(async (resetUnreadSnapshot = false) => {
    const list = (await backend.stories.list() ?? []).filter(isListableStory);
    setStories(list);
    if (resetUnreadSnapshot) {
      setUnreadStorySnapshot(unreadStorySnapshotIds(list));
    }
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
        await loadStories(effectiveRssListFilter === "unread");
      } else {
        await loadArticles(false);
      }
    } catch (e) {
      setError(GENERIC_ERROR_MESSAGE);
    } finally {
      setBusy(false);
    }
  }, [backend, loadFeeds, loadArticles, loadStories, selected.type, effectiveRssListFilter]);

  useEffect(() => {
    void (async () => {
      try {
        await backend.system.ping();
        await Promise.all([loadFeeds(), loadStories()]);
        if (!serverAuthoritative) void drainPendingExtracts(backend);
      } catch (e) {
        setError(GENERIC_ERROR_MESSAGE);
      }
    })();
  }, [backend, loadFeeds, loadStories, serverAuthoritative]);

  useEffect(() => {
    if (selected.type === "stories") {
      void loadStories(rssListFilter === "unread");
    } else {
      setUnreadStorySnapshot(null);
      void loadArticles(false);
    }
  }, [selected, rssListFilter]);

  useEffect(() => {
    if (selected.type !== "stories") {
      void loadArticles(false);
    }
  }, [search]);

  useEffect(() => {
    if (!isStoriesMode) return;
    setActiveStoryId((id) => {
      if (id && visibleStories.some((story) => story.id === id)) return id;
      return visibleStories[0]?.id ?? null;
    });
  }, [isStoriesMode, effectiveRssListFilter, stories, unreadStorySnapshot]);

  useEffect(() => {
    if (!isStoriesMode || !activeStoryId) {
      if (!isStoriesMode) {
        setActiveStory(null);
        setStoryMemberId(null);
      }
      return;
    }
    void backend.stories
      .get(activeStoryId)
      .then((full) => {
        if (!full || !isListableStory(full)) {
          setActiveStory(null);
          setStoryMemberId(null);
          void loadStories();
          return;
        }
        setActiveStory(full);
      })
      .catch(() => setActiveStory(null));
  }, [backend, isStoriesMode, activeStoryId, loadStories]);

  useEffect(() => {
    return backend.onEvent((event) => {
      const name: BackendEventName = event.event;
      switch (name) {
        case "articles.added":
        case "feed.updated":
        case "feed.error":
          void loadFeeds();
          if (shouldReloadArticleListInBackground(effectiveRssListFilter)) {
            reloadContent(false);
          }
          break;
        case "article.updated": {
          void loadFeeds();
          const payload = event.payload as {
            articleId?: string;
            priority?: Priority;
            isRead?: boolean;
            isStarred?: boolean;
            all?: boolean;
            extractStatus?: string;
            crawlStatus?: string;
          };
          if (payload.isRead !== undefined || payload.all) {
            void loadStories();
          }
          if (payload.articleId) {
            if (payload.extractStatus === "js" && !serverAuthoritative) {
              void runFrontendExtract(backend, payload.articleId);
            }
            setArticles((prev) =>
              prev.map((a) =>
                a.id === payload.articleId
                  ? {
                      ...a,
                      ...(payload.priority !== undefined ? { priority: payload.priority } : {}),
                      ...(payload.isRead !== undefined ? { isRead: payload.isRead } : {}),
                      ...(payload.isStarred !== undefined ? { isStarred: payload.isStarred } : {}),
                      ...(payload.extractStatus !== undefined
                        ? { extractStatus: payload.extractStatus as Article["extractStatus"] }
                        : {}),
                      ...(payload.crawlStatus !== undefined
                        ? { crawlStatus: payload.crawlStatus as Article["crawlStatus"] }
                        : {}),
                    }
                  : a,
              ),
            );
            setActiveStory((prev) => {
              if (!prev?.articles) return prev;
              const nextArticles = prev.articles.map((a) =>
                a.id === payload.articleId
                  ? {
                      ...a,
                      ...(payload.priority !== undefined ? { priority: payload.priority } : {}),
                      ...(payload.isRead !== undefined ? { isRead: payload.isRead } : {}),
                      ...(payload.isStarred !== undefined ? { isStarred: payload.isStarred } : {}),
                    }
                  : a,
              );
              return {
                ...prev,
                ...(payload.isRead !== undefined
                  ? { isRead: nextArticles.every((article) => article.isRead) }
                  : {}),
                articles: nextArticles,
              };
            });
          }
          break;
        }
        case "article.removed": {
          void loadFeeds();
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
              if (!isListableStory(full)) {
                if (payload.storyId === activeStoryId) {
                  setActiveStory(null);
                  setStoryMemberId(null);
                  void loadStories();
                }
                return;
              }
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
          void Promise.all([loadFeeds(), loadStories()]).then(() => {
            if (shouldReloadArticleListInBackground(effectiveRssListFilter)) {
              reloadContent(false);
            }
          });
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
  }, [backend, loadFeeds, reloadContent, loadStories, activeStoryId, settings, effectiveRssListFilter]);

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

  useEffect(() => {
    if (!serverAuthoritative || isStoriesMode || !activeId) return;
    let cancelled = false;
    void backend.articles
      .get(activeId)
      .then((article) => {
        if (!cancelled) patchArticle(article);
      })
      .catch(() => {
        if (!cancelled) setError(GENERIC_ERROR_MESSAGE);
      });
    return () => {
      cancelled = true;
    };
  }, [backend, serverAuthoritative, isStoriesMode, activeId, articleListRevision, patchArticle]);

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
          if (!full) {
            setActiveStory(null);
            return;
          }
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
        setError(GENERIC_ERROR_MESSAGE);
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
        setError(GENERIC_ERROR_MESSAGE);
      }
    },
    [backend, activeStory, patchStory],
  );

  const splitActiveStory = useCallback(async () => {
    if (!activeStory) return;
    try {
      const { storyIds } = await backend.stories.split(activeStory.id);
      await loadStories();
      if (storyIds?.[0]) {
        setActiveStoryId(storyIds[0]);
        setStoryMemberId(null);
      }
    } catch (e) {
      setError(GENERIC_ERROR_MESSAGE);
    }
  }, [backend, activeStory, loadStories]);

  const selectArticle = useCallback(
    (article: Article) => {
      setActiveId(article.id);
      if (settings?.markReadOnOpen && !article.isRead) {
        patchArticle({ ...article, isRead: true });
        void backend.articles
          .markRead(article.id)
          .then((updated) => {
            patchArticle(updated);
            void loadFeeds();
          })
          .catch(() => {
            patchArticle(article);
            setError(GENERIC_ERROR_MESSAGE);
          });
      }
    },
    [backend, settings?.markReadOnOpen, patchArticle, loadFeeds],
  );

  const selectStory = useCallback(
    (story: Story) => {
      setActiveStoryId(story.id);
      setStoryMemberId(null);
      setActiveId(null);
      setActiveStory((current) => (current?.id === story.id ? current : story));
      if (settings?.markReadOnOpen && !story.isRead) {
        patchStory({ ...story, isRead: true });
        void backend.stories
          .markRead(story.id)
          .then(patchStory)
          .catch(() => {
            patchStory(story);
            setError(GENERIC_ERROR_MESSAGE);
          });
      }
    },
    [backend, settings?.markReadOnOpen, patchStory],
  );

  const selectStoryMember = useCallback(
    (article: Article) => {
      setStoryMemberId(article.id);
      setActiveId(article.id);
      if (settings?.markReadOnOpen && !article.isRead) {
        patchArticle({ ...article, isRead: true });
        void backend.articles
          .markRead(article.id)
          .then((updated) => {
            patchArticle(updated);
            void loadFeeds();
          })
          .catch(() => {
            patchArticle(article);
            setError(GENERIC_ERROR_MESSAGE);
          });
      }
    },
    [backend, settings?.markReadOnOpen, patchArticle, loadFeeds],
  );

  const moveSelection = useCallback(
    (delta: number) => {
      if (isStoriesMode) {
        if (
          delta > 0 &&
          storyMemberId &&
          hasNoOtherUnreadStoryMembers(activeStory, storyMemberId)
        ) {
          const nextStory = adjacentMetaStory(visibleStories, activeStoryId, 1);
          if (nextStory && nextStory.id !== activeStoryId) {
            void selectStory(nextStory);
          }
          return;
        }
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
      visibleStories,
      selectStory,
      selectStoryMember,
      articles,
      activeId,
      selectArticle,
    ],
  );

  const moveMetaStory = useCallback(
    (delta: number) => {
      if (!isStoriesMode) return;
      const next = adjacentMetaStory(visibleStories, activeStoryId, delta);
      if (!next || next.id === activeStoryId) return;
      void selectStory(next);
    },
    [isStoriesMode, visibleStories, activeStoryId, selectStory],
  );

  useEffect(() => {
    const key = isStoriesMode
      ? storyMemberId
        ? storyListRowKey({ kind: "member", storyId: activeStoryId ?? "", articleId: storyMemberId })
        : activeStoryId
          ? storyListRowKey({ kind: "story", storyId: activeStoryId })
          : null
      : activeId
        ? `article:${activeId}`
        : null;
    if (!key || lastScrolledListRowKeyRef.current === key) return;
    const frame = window.requestAnimationFrame(() => {
      if (scrollListRowToTop(articleListRef.current, key)) {
        lastScrolledListRowKeyRef.current = key;
      }
    });
    return () => window.cancelAnimationFrame(frame);
  }, [isStoriesMode, activeStoryId, storyMemberId, activeId, storyRows]);

  useEffect(() => {
    setContentTab("primary");
    setBrowserUrl(null);
  }, [activeId]);

  useEffect(() => window.desktop.onOpenInPane(setBrowserUrl), []);

  useEffect(() => {
    const openLinksInPane = (event: MouseEvent) => {
      const anchor = (event.target as Element | null)?.closest?.("a[href]") as HTMLAnchorElement | null;
      if (!anchor) return;
      const url = browserPaneUrl(
        anchor.getAttribute("href"),
        active?.url || window.location.href,
      );
      if (!url) return;
      event.preventDefault();
      event.stopPropagation();
      setBrowserUrl(url);
    };
    document.addEventListener("click", openLinksInPane, true);
    return () => document.removeEventListener("click", openLinksInPane, true);
  }, [active?.url]);

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
      setError(GENERIC_ERROR_MESSAGE);
    } finally {
      setBusy(false);
    }
  };

  const addReadLaterUrl = async () => {
    const result = normalizeDroppedUrl(rlAddUrl);
    if (!result.ok) {
      setError("Enter a valid http(s) URL");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const saved = await backend.readLater.add(result.url);
      setRlAddUrl("");
      setRlSearch("");
      setReadLaterFocusId(saved.id);
      setAppMode("readLater");
    } catch (e) {
      setError(GENERIC_ERROR_MESSAGE);
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

  const openAddLinkModal = useCallback(async (attempted: string) => {
    try {
      await window.desktop.focusMainWindow();
    } catch {
      // browser / missing bridge
    }
    setDropModal({ attempted, draft: attempted, error: null });
  }, []);

  const handleAddLinkRequested = useCallback(
    (raw: string) => {
      void openAddLinkModal(raw.trim());
    },
    [openAddLinkModal],
  );

  const undoDroppedSave = useCallback(async () => {
    if (!toast) return;
    const id = toast.undoId;
    clearToastTimer();
    setToast(null);
    try {
      await backend.readLater.remove(id);
    } catch (e) {
      setError(GENERIC_ERROR_MESSAGE);
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
          ? { ...prev, error: GENERIC_ERROR_MESSAGE }
          : prev,
      );
    } finally {
      setBusy(false);
    }
  }, [dropModal, saveDroppedUrl]);

  useEffect(() => {
    const unsub =
      typeof window.desktop?.onAddLinkRequested === "function"
        ? window.desktop.onAddLinkRequested(handleAddLinkRequested)
        : () => undefined;
    return () => {
      unsub();
      clearToastTimer();
    };
  }, [clearToastTimer, handleAddLinkRequested]);

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
        setError(GENERIC_ERROR_MESSAGE);
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
      setError(GENERIC_ERROR_MESSAGE);
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
        setError(GENERIC_ERROR_MESSAGE),
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
          onNavigate={setBrowserUrl}
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
      statusMessage = "Crawl failed.";
    } else if (article.crawledContent) {
      bodyHtml = article.crawledContent;
      asFullPage = true;
    } else if (article.crawlStatus === "none") {
      statusMessage = "No crawled page yet.";
    } else {
      statusMessage = "No crawled page available.";
    }

    if (bodyHtml && asFullPage && !serverAuthoritative) {
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
        onClick={(event) => {
          const anchor = (event.target as Element).closest("a[href]") as HTMLAnchorElement | null;
          if (!anchor) return;
          const url = browserPaneUrl(anchor.getAttribute("href"), article.url);
          if (!url) return;
          event.preventDefault();
          setBrowserUrl(url);
        }}
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

      if (e.shiftKey && e.key.toLowerCase() === "j") {
        e.preventDefault();
        moveMetaStory(1);
        return;
      }

      switch (e.key) {
        case "j":
          e.preventDefault();
          moveSelection(1);
          break;
        case "k":
          e.preventDefault();
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
    moveMetaStory,
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
      setError(GENERIC_ERROR_MESSAGE);
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
      setError(GENERIC_ERROR_MESSAGE);
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
      setError(GENERIC_ERROR_MESSAGE);
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
      setError(GENERIC_ERROR_MESSAGE);
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
  const readLaterUnread = feeds.find((feed) => feed.isReadLater)?.unreadCount ?? 0;
  const densityClass = `density-${settings?.articleDensity ?? "comfortable"}`;

  const changeRssListFilter = (next: RssListFilter) => {
    setRssListFilter(next);
  };

  const markCurrentListRead = async () => {
    setMarkAllBusy(true);
    setError(null);
    try {
      if (isStoriesMode) {
        const unreadStories = stories.filter((story) => !story.isRead);
        await Promise.all(unreadStories.map((story) => backend.stories.markRead(story.id)));
        await loadStories();
      } else {
        await backend.articles.markAllRead(articleScopeQuery);
        setArticles((current) => current.map((article) => ({ ...article, isRead: true })));
        await loadFeeds();
      }
    } catch (e) {
      setError(GENERIC_ERROR_MESSAGE);
    } finally {
      setMarkAllBusy(false);
    }
  };

  const openArticleFromSettings = useCallback(async (articleId: string) => {
    const article = await backend.articles.get(articleId);
    setBrowserUrl(null);
    setSettingsSection("general");
    if (article.isReadLater) {
      setReadLaterFocusId(article.id);
      setAppMode("readLater");
      setView("reader");
      return;
    }

    const listWillReload =
      selected.type !== "items" || rssListFilter !== "all" || search.trim() !== "";
    pendingArticleDeepLinkRef.current = article;
    setArticles((prev) => [article, ...prev.filter((item) => item.id !== article.id)]);
    setActiveId(article.id);
    if (selected.type !== "items") {
      setSelected({ type: "items" });
    }
    setRssListFilter("all");
    setSearch("");
    setAppMode("rss");
    setView("reader");
    if (!listWillReload) {
      pendingArticleDeepLinkRef.current = null;
    }
  }, [backend, selected.type, rssListFilter, search]);

  const dropFixModal = dropModal ? (
    <div className="modal-backdrop" onClick={() => setDropModal(null)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2>Add link to Read Later</h2>
        <p className="modal-hint">Paste or edit a web link, then add it to your saved reading list.</p>
        <label className="modal-label" htmlFor="drop-attempted">
          Clipboard text
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
          onOpenArticle={openArticleFromSettings}
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
            RSS Reader ({totalUnread})
          </button>
          <button
            type="button"
            role="tab"
            className={`mode-tab ${appMode === "readLater" ? "active" : ""}`}
            aria-selected={appMode === "readLater"}
            onClick={() => setAppMode("readLater")}
          >
            Read Later ({readLaterUnread})
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
              onChange={(e) => {
                setRlAddUrl(e.target.value);
                if (error === "Enter a valid http(s) URL") setError(null);
              }}
              disabled={busy}
              aria-invalid={error === "Enter a valid http(s) URL"}
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
      </header>

      {error && <p className="error toolbar-error">{error}</p>}

      {appMode === "readLater" ? (
        <ReadLaterView
          backend={backend}
          serverAuthoritative={serverAuthoritative}
          search={rlSearch}
          unreadCount={readLaterUnread}
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
        <aside className="pane sidebar rss-sidebar">
          <div className="rss-sidebar-content">
          <button
            className={`nav-item ${selected.type === "items" ? "active" : ""}`}
            onClick={() => setSelected({ type: "items" })}
          >
            <span>Items</span>
            <span className="count">{totalUnread || ""}</span>
          </button>
          <button
            className={`nav-item ${selected.type === "stories" ? "active" : ""}`}
            onClick={() => setSelected({ type: "stories" })}
          >
            <span>Stories</span>
            <span className="count">{storyUnread || ""}</span>
          </button>

          <div className="section-label-row">
            <div className="section-label">Feeds</div>
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
                        title={feed.lastError ? "Feed refresh failed" : feed.url}
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

          {visibleFeeds.length === 0 && (
            <div className="empty" style={{ height: "auto", padding: 12 }}>
              No feeds yet
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
                title={feed.lastError ? "Feed refresh failed" : feed.url}
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
          <div className="rss-sidebar-footer">
            <button type="button" className="btn primary rss-sidebar-add-feed" onClick={() => setShowAdd(true)}>
              Add feed
            </button>
          </div>
        </aside>

        <section ref={articleListRef} className="pane article-list">
          <div className="article-list-toolbar">
            <div className="article-list-filter" role="group" aria-label="Filter RSS list">
              <button
                type="button"
                className={effectiveRssListFilter === "all" ? "active" : ""}
                aria-pressed={effectiveRssListFilter === "all"}
                onClick={() => changeRssListFilter("all")}
              >
                All
              </button>
              <button
                type="button"
                className={effectiveRssListFilter === "unread" ? "active" : ""}
                aria-pressed={effectiveRssListFilter === "unread"}
                onClick={() => changeRssListFilter("unread")}
              >
                Unread
              </button>
            </div>
            <button
              type="button"
              className="btn article-list-mark-read"
              disabled={markAllBusy}
              onClick={() => void markCurrentListRead()}
            >
              {markAllBusy ? "Marking…" : "Mark All as Read"}
            </button>
          </div>
          {isStoriesMode ? (
            visibleStories.length === 0 ? (
              <div className="empty">
                <h2>{effectiveRssListFilter === "unread" ? "No unread stories" : "No stories yet"}</h2>
                <p>
                  {effectiveRssListFilter === "unread"
                    ? "Everything in this list has been read."
                    : "Related RSS articles group here after feeds refresh. AI triage can still override groups when enabled."}
                </p>
              </div>
            ) : (
              storyRows.map((row) => {
                switch (row.kind) {
                  case "story": {
                    const story = visibleStories.find((s) => s.id === row.storyId);
                    if (!story) return null;
                    const isActive = story.id === activeStoryId && !storyMemberId;
                    return (
                      <button
                        key={storyListRowKey(row)}
                        data-list-row-key={storyListRowKey(row)}
                        className={`article-row ${isActive ? "active" : ""} ${story.isRead ? "" : "unread"}`}
                        onClick={() => void selectStory(story)}
                      >
                        <div className="article-meta">
                          <span>
                            {story.memberCount} article{story.memberCount === 1 ? "" : "s"}
                          </span>
                          <span>{formatRelativeTime(story.updatedAt ?? story.createdAt)}</span>
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
                        data-list-row-key={storyListRowKey(row)}
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
                data-list-row-key={`article:${article.id}`}
                className={`article-row ${article.id === activeId ? "active" : ""} ${article.isRead ? "" : "unread"}`}
                onClick={() => void selectArticle(article)}
              >
                <div className="article-meta">
                  <span>{article.feedTitle}</span>
                  <span>{formatRelativeTime(article.publishedAt ?? article.discoveredAt)}</span>
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
            <div ref={loadMoreSentinelRef} className="article-list-pagination" role="status">
              {loadingMore ? "Loading more…" : null}
            </div>
          )}
        </section>

        <section className="pane reader-pane">
          {browserUrl ? (
            <BrowserPane
              initialUrl={browserUrl}
              onClose={() => setBrowserUrl(null)}
              onSave={async (url) => { await saveDroppedUrl(url); }}
            />
          ) : isStoriesMode && !storyMemberId ? (
            !activeStory ? (
              <div className="empty">
                <h2>Stories</h2>
                <p>Select a story to read grouped coverage. Shortcuts: j/k, r, u, f</p>
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
                  <button type="button" className="btn" onClick={() => void splitActiveStory()}>
                    Split
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
              <p>Select an article to read. Shortcuts: j/k, o, r, u, f, /</p>
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
                  {!active.isReadLater && active.url ? (
                    <button className="btn" disabled={busy} onClick={() => void addReadLaterFromActive()}>
                      Send to Read Later
                    </button>
                  ) : null}
                  {active.url && (
                    <button className="btn primary" onClick={() => setBrowserUrl(active.url)}>
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
