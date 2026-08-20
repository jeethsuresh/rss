import { useCallback, useEffect, useState } from "react";
import type { Article, ReadLaterFilter, ReaderBackend } from "@rss-reader/shared";
import { PageFrame } from "../components/PageFrame";
import { formatRelativeTime, stripHtml, decodeHtmlEntities } from "../lib/html";

type ContentTab = "primary" | "secondary";

type Props = {
  backend: ReaderBackend;
  search: string;
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

const FILTERS: { id: ReadLaterFilter; label: string }[] = [
  { id: "all", label: "All" },
  { id: "unread", label: "Unread" },
  { id: "starred", label: "Starred" },
  { id: "archived", label: "Archived" },
];

export function ReadLaterView({ backend, search, focusArticleId, onFocusConsumed }: Props) {
  const [filter, setFilter] = useState<ReadLaterFilter>("all");
  const [articles, setArticles] = useState<Article[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [contentBusy, setContentBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [contentTab, setContentTab] = useState<ContentTab>("primary");

  const active = articles.find((a) => a.id === activeId) ?? null;

  const load = useCallback(async () => {
    const list = (await backend.readLater.list(filter, search.trim() || undefined)) ?? [];
    setArticles(list);
    setActiveId((id) => {
      if (id && list.some((a) => a.id === id)) return id;
      return list[0]?.id ?? null;
    });
  }, [backend, filter, search]);

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

  const patchLocal = (updated: Article) => {
    setArticles((prev) => prev.map((a) => (a.id === updated.id ? updated : a)));
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
          <PageFrame
            html={bodyHtml}
            pageUrl={article.url}
            title={decodeHtmlEntities(article.title || "Article page")}
          />
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

  return (
    <div className="layout">
      <aside className="pane sidebar">
        {FILTERS.map((f) => (
          <button
            key={f.id}
            type="button"
            className={`nav-item ${filter === f.id ? "active" : ""}`}
            onClick={() => setFilter(f.id)}
          >
            <span>{f.label}</span>
          </button>
        ))}
      </aside>

      <section className="pane article-list">
        {error && <p className="error" style={{ padding: "8px 12px" }}>{error}</p>}
        {articles.length === 0 ? (
          <div className="empty">
            <h2>Nothing here</h2>
            <p>Paste a URL in the toolbar, or send an article from RSS Reader.</p>
          </div>
        ) : (
          articles.map((a) => (
            <button
              key={a.id}
              type="button"
              className={`article-row ${a.id === activeId ? "active" : ""} ${a.isRead ? "" : "unread"}`}
              onClick={() => setActiveId(a.id)}
            >
              <div className="article-meta">
                <span>{hostOf(a.url)}</span>
                <span>{formatRelativeTime(a.discoveredAt)}</span>
                {a.isStarred ? <span>★</span> : null}
              </div>
              <h3 className="article-title">{decodeHtmlEntities(a.title || "(untitled)")}</h3>
              <p className="article-summary">{stripHtml(a.summary || a.url)}</p>
            </button>
          ))
        )}
      </section>

      <section className="pane reader-pane">
        {!active ? (
          <div className="empty">
            <h2>Read Later</h2>
            <p>Select a saved link to read.</p>
          </div>
        ) : (
          <article className="reader reader-fullbleed">
            <div className="reader-toolbar">
              <div className="content-tabs">
                <button
                  type="button"
                  className={`content-tab ${contentTab === "primary" ? "active" : ""}`}
                  onClick={() => setContentTab("primary")}
                >
                  Live
                </button>
                <button
                  type="button"
                  className={`content-tab ${contentTab === "secondary" ? "active" : ""}`}
                  onClick={() => setContentTab("secondary")}
                >
                  Saved crawl
                </button>
              </div>
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
                {contentTab === "secondary" && (
                  <button className="btn" disabled={contentBusy} onClick={() => void recrawl()}>
                    {contentBusy ? "Re-crawling…" : "Re-crawl page"}
                  </button>
                )}
              </div>
            </div>
            {renderBody(active)}
          </article>
        )}
      </section>
    </div>
  );
}
