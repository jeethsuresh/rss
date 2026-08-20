import { useCallback, useEffect, useState } from "react";
import type { Article, ReadLaterFilter, ReaderBackend } from "@rss-reader/shared";
import { PageFrame } from "../components/PageFrame";
import { formatRelativeTime } from "../lib/html";

type ContentTab = "primary" | "secondary";

type Props = {
  backend: ReaderBackend;
  focusArticleId?: string | null;
  onFocusConsumed?: () => void;
};

function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

export function ReadLaterView({ backend, focusArticleId, onFocusConsumed }: Props) {
  const [filter, setFilter] = useState<ReadLaterFilter>("all");
  const [articles, setArticles] = useState<Article[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [addUrl, setAddUrl] = useState("");
  const [busy, setBusy] = useState(false);
  const [contentBusy, setContentBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [contentTab, setContentTab] = useState<ContentTab>("primary");

  const active = articles.find((a) => a.id === activeId) ?? null;

  const load = useCallback(async () => {
    const list = (await backend.readLater.list(filter)) ?? [];
    setArticles(list);
    setActiveId((id) => {
      if (id && list.some((a) => a.id === id)) return id;
      return list[0]?.id ?? null;
    });
  }, [backend, filter]);

  useEffect(() => {
    void load().catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, [load]);

  useEffect(() => {
    if (!focusArticleId) return;
    setFilter("all");
    setActiveId(focusArticleId);
    void load().finally(() => onFocusConsumed?.());
  }, [focusArticleId, load, onFocusConsumed]);

  useEffect(() => {
    setContentTab("primary");
  }, [activeId]);

  useEffect(() => {
    return backend.onEvent((ev) => {
      if (ev.event === "article.updated" || ev.event === "articles.added") {
        void load();
      }
    });
  }, [backend, load]);

  useEffect(() => {
    if (!active?.id || contentTab !== "primary" || active.liveContent) return;
    let cancelled = false;
    setContentBusy(true);
    void backend.articles
      .fetchLive(active.id)
      .then((updated) => {
        if (cancelled) return;
        setArticles((prev) => prev.map((a) => (a.id === updated.id ? updated : a)));
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "Failed to fetch live page");
      })
      .finally(() => {
        if (!cancelled) setContentBusy(false);
      });
    return () => {
      cancelled = true;
    };
  }, [active?.id, active?.liveContent, contentTab, backend]);

  const addLink = async () => {
    const url = addUrl.trim();
    if (!url) return;
    setBusy(true);
    setError(null);
    try {
      const art = await backend.readLater.add(url);
      setAddUrl("");
      setFilter("all");
      setActiveId(art.id);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not save link");
    } finally {
      setBusy(false);
    }
  };

  const patchLocal = (updated: Article) => {
    setArticles((prev) => prev.map((a) => (a.id === updated.id ? updated : a)));
  };

  const handleTab = async (tab: ContentTab) => {
    setContentTab(tab);
  };

  const recrawl = async () => {
    if (!active) return;
    setContentBusy(true);
    setError(null);
    try {
      const updated = await backend.articles.recrawl(active.id);
      patchLocal(updated);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Recrawl failed");
    } finally {
      setContentBusy(false);
    }
  };

  const renderBody = (article: Article) => {
    let bodyHtml: string | null = null;
    let statusMessage: string | null = null;

    if (contentTab === "primary") {
      if (article.liveContent) {
        bodyHtml = article.liveContent;
      } else if (contentBusy) {
        statusMessage = "Fetching live page…";
      } else {
        statusMessage = "No live page yet.";
      }
    } else if (article.crawlStatus === "pending") {
      statusMessage = "Crawl in progress…";
    } else if (article.crawlStatus === "failed" && !article.crawledContent) {
      statusMessage = article.crawlError || "Crawl failed.";
    } else if (article.crawledContent) {
      bodyHtml = article.crawledContent;
    } else {
      statusMessage = "No crawled page yet.";
    }

    if (bodyHtml) {
      return (
        <div className="reader-page-wrap">
          <PageFrame html={bodyHtml} pageUrl={article.url} title={article.title || "Article page"} />
          {contentTab === "secondary" && (
            <div className="reader-actions" style={{ marginTop: 8 }}>
              <button className="btn" disabled={contentBusy} onClick={() => void recrawl()}>
                {contentBusy ? "Re-crawling…" : "Re-crawl page"}
              </button>
            </div>
          )}
        </div>
      );
    }

    return (
      <div className="reader-body">
        <p className="muted">{statusMessage ?? "No content"}</p>
        {contentTab === "secondary" && (
          <button className="btn" disabled={contentBusy} onClick={() => void recrawl()}>
            {contentBusy ? "Retrying…" : "Retry crawl"}
          </button>
        )}
      </div>
    );
  };

  const filters: { id: ReadLaterFilter; label: string }[] = [
    { id: "all", label: "All" },
    { id: "unread", label: "Unread" },
    { id: "starred", label: "Starred" },
    { id: "archived", label: "Archived" },
  ];

  return (
    <div className="layout read-later-layout">
      <section className="pane article-list">
        <div className="rl-toolbar">
          <div className="content-tabs rl-filters">
            {filters.map((f) => (
              <button
                key={f.id}
                type="button"
                className={`content-tab ${filter === f.id ? "active" : ""}`}
                onClick={() => setFilter(f.id)}
              >
                {f.label}
              </button>
            ))}
          </div>
          <form
            className="rl-add"
            onSubmit={(e) => {
              e.preventDefault();
              void addLink();
            }}
          >
            <input
              className="search"
              placeholder="https://… paste a URL to save"
              value={addUrl}
              onChange={(e) => setAddUrl(e.target.value)}
              disabled={busy}
            />
            <button className="btn primary" type="submit" disabled={busy || !addUrl.trim()}>
              {busy ? "Adding…" : "Add"}
            </button>
          </form>
          {error && <p className="error">{error}</p>}
        </div>

        {articles.length === 0 ? (
          <div className="empty" style={{ height: "auto", padding: 24 }}>
            <p>No Read Later items here.</p>
            <p className="muted">Paste a URL above, or send an article from RSS Reader.</p>
          </div>
        ) : (
          <div className="list">
            {articles.map((a) => (
              <button
                key={a.id}
                type="button"
                className={`article-row ${a.id === activeId ? "active" : ""} ${a.isRead ? "read" : "unread"}`}
                onClick={() => setActiveId(a.id)}
              >
                <div className="article-title">
                  {!a.isRead ? <strong>{a.title || "(untitled)"}</strong> : a.title || "(untitled)"}
                  {a.isStarred ? " ★" : ""}
                </div>
                <div className="article-meta muted">
                  {hostOf(a.url)}
                  {a.discoveredAt ? ` · ${formatRelativeTime(a.discoveredAt)}` : ""}
                </div>
              </button>
            ))}
          </div>
        )}
      </section>

      <section className="pane reader-pane">
        {!active ? (
          <div className="empty">
            <h2>Read Later</h2>
            <p>Select a saved link, or add one above.</p>
          </div>
        ) : (
          <article className="reader">
            <div className="reader-kicker">
              {hostOf(active.url)}
              {active.publishedAt ? ` · ${new Date(active.publishedAt).toLocaleString()}` : ""}
              {active.crawlStatus === "pending" ? " · crawling…" : ""}
            </div>
            <div className="content-tabs">
              <button
                type="button"
                className={`content-tab ${contentTab === "primary" ? "active" : ""}`}
                onClick={() => void handleTab("primary")}
              >
                Live
              </button>
              <button
                type="button"
                className={`content-tab ${contentTab === "secondary" ? "active" : ""}`}
                onClick={() => void handleTab("secondary")}
              >
                Saved crawl
              </button>
            </div>
            <h1>{active.title || "(untitled)"}</h1>
            <div className="reader-actions">
              <button
                className="btn"
                onClick={() =>
                  void backend.articles[active.isRead ? "markUnread" : "markRead"](active.id).then(patchLocal)
                }
              >
                {active.isRead ? "Mark unread" : "Mark read"}
              </button>
              <button
                className="btn"
                onClick={() => void backend.articles.toggleStar(active.id).then(patchLocal)}
              >
                {active.isStarred ? "Unstar" : "Star"}
              </button>
              {active.archivedAt ? (
                <button
                  className="btn"
                  onClick={() =>
                    void backend.readLater.unarchive(active.id).then((updated) => {
                      patchLocal(updated);
                      void load();
                    })
                  }
                >
                  Unarchive
                </button>
              ) : (
                <button
                  className="btn"
                  onClick={() =>
                    void backend.readLater.archive(active.id).then(() => {
                      void load();
                    })
                  }
                >
                  Archive
                </button>
              )}
              {active.url && (
                <button className="btn primary" onClick={() => void window.desktop.openExternal(active.url)}>
                  Open original
                </button>
              )}
            </div>
            {renderBody(active)}
          </article>
        )}
      </section>
    </div>
  );
}
