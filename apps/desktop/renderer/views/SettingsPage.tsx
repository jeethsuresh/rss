import { useCallback, useEffect, useRef, useState } from "react";
import type {
  AILogEntry,
  AIStatus,
  AITestResult,
  BackendEventName,
  Feed,
  FeedImportResult,
  ReaderBackend,
  Settings,
} from "@rss-reader/shared";

type Props = {
  backend: ReaderBackend;
  settings: Settings;
  onSettings: (s: Settings) => void;
  onClose: () => void;
  applyTheme: (theme: Settings["theme"]) => void;
};

export function SettingsPage({ backend, settings, onSettings, onClose, applyTheme }: Props) {
  const [feeds, setFeeds] = useState<Feed[]>([]);
  const [feedFilter, setFeedFilter] = useState("");
  const [importText, setImportText] = useState("");
  const [importResult, setImportResult] = useState<FeedImportResult | null>(null);
  const [aiStatus, setAiStatus] = useState<AIStatus | null>(null);
  const [aiTest, setAiTest] = useState<AITestResult | null>(null);
  const [aiLogs, setAiLogs] = useState<AILogEntry[]>([]);
  const logPanelRef = useRef<HTMLDivElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [section, setSection] = useState<"general" | "feeds" | "ai">("general");

  const reloadFeeds = useCallback(async () => {
    setFeeds(await backend.feeds.list());
  }, [backend]);

  useEffect(() => {
    void reloadFeeds();
    void backend.ai.status().then(setAiStatus).catch(() => undefined);
    void backend.ai.logs(200).then(setAiLogs).catch(() => undefined);
  }, [backend, reloadFeeds]);

  useEffect(() => {
    return backend.onEvent((ev) => {
      const name: BackendEventName = ev.event;
      switch (name) {
        case "ai.status":
          setAiStatus(ev.payload as AIStatus);
          break;
        case "ai.log":
          setAiLogs((prev) => [...prev, ev.payload as AILogEntry].slice(-200));
          break;
        case "feed.updated":
        case "feed.error":
          void reloadFeeds();
          break;
        case "articles.added":
        case "article.updated":
        case "story.updated":
        case "sync.status":
          break;
        default: {
          const _exhaustive: never = name;
          void _exhaustive;
          break;
        }
      }
    });
  }, [backend, reloadFeeds]);

  useEffect(() => {
    const panel = logPanelRef.current;
    if (panel) panel.scrollTop = panel.scrollHeight;
  }, [aiLogs]);

  const patch = async (partial: Partial<Settings>) => {
    const next = await backend.settings.update(partial);
    onSettings(next);
    if (partial.theme) applyTheme(next.theme);
  };

  const filtered = feeds.filter((f) => {
    if (f.isReadLater) return false;
    const q = feedFilter.trim().toLowerCase();
    if (!q) return true;
    return f.title.toLowerCase().includes(q) || f.url.toLowerCase().includes(q);
  });

  return (
    <div className="settings-page">
      <header className="toolbar">
        <button className="btn" onClick={onClose}>
          ← Back
        </button>
        <div className="brand">Settings</div>
        <div className="toolbar-spacer" />
      </header>

      <div className="settings-layout">
        <nav className="settings-nav">
          {(
            [
              ["general", "General"],
              ["feeds", "Feeds"],
              ["ai", "AI"],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              className={`nav-item ${section === id ? "active" : ""}`}
              onClick={() => setSection(id)}
            >
              {label}
            </button>
          ))}
        </nav>

        <div className="settings-body">
          {error && <p className="error">{error}</p>}

          {section === "general" && (
            <section className="settings-section">
              <h2>General</h2>
              <label className="field">
                Theme
                <select
                  value={settings.theme}
                  onChange={(e) => void patch({ theme: e.target.value as Settings["theme"] })}
                >
                  <option value="system">System</option>
                  <option value="light">Light</option>
                  <option value="dark">Dark</option>
                </select>
              </label>
              <label className="field">
                Density
                <select
                  value={settings.articleDensity}
                  onChange={(e) =>
                    void patch({ articleDensity: e.target.value as Settings["articleDensity"] })
                  }
                >
                  <option value="comfortable">Comfortable</option>
                  <option value="compact">Compact</option>
                </select>
              </label>
              <label className="field">
                Default poll (seconds)
                <input
                  type="number"
                  min={60}
                  value={settings.defaultPollIntervalSeconds}
                  onChange={(e) => void patch({ defaultPollIntervalSeconds: Number(e.target.value) })}
                />
              </label>
              <label className="check">
                <input
                  type="checkbox"
                  checked={settings.markReadOnOpen}
                  onChange={(e) => void patch({ markReadOnOpen: e.target.checked })}
                />
                Mark read on open
              </label>
              <label className="check">
                <input
                  type="checkbox"
                  checked={settings.notificationsEnabled}
                  onChange={(e) => void patch({ notificationsEnabled: e.target.checked })}
                />
                Desktop notifications
              </label>
              <label className="field">
                Read Later entry
                <select
                  value={settings.readLaterChrome ?? "tabs"}
                  onChange={(e) =>
                    void patch({
                      readLaterChrome: e.target.value as Settings["readLaterChrome"],
                    })
                  }
                >
                  <option value="tabs">Mode tabs (RSS Reader | Read Later)</option>
                  <option value="brandControl">Brand + Read Later control</option>
                </select>
              </label>
            </section>
          )}

          {section === "feeds" && (
            <section className="settings-section">
              <h2>Feed management</h2>
              <div className="settings-row">
                <input
                  className="search"
                  placeholder="Filter feeds…"
                  value={feedFilter}
                  onChange={(e) => setFeedFilter(e.target.value)}
                />
                <button
                  className="btn"
                  onClick={() =>
                    void backend.feeds.exportUrls().then((r) => {
                      void navigator.clipboard.writeText(r.text);
                    })
                  }
                >
                  Export URLs
                </button>
              </div>

              <div className="feed-table">
                {filtered.map((f) => (
                  <div key={f.id} className="feed-table-row">
                    <div>
                      <strong>{f.title || f.url}</strong>
                      <div className="muted">{f.url}</div>
                      <div className="muted">Bad crawls: {f.badCrawlPercent.toFixed(0)}%</div>
                      {f.lastError ? <div className="error">{f.lastError}</div> : null}
                    </div>
                    <div className="feed-table-actions">
                      <span className="muted">{f.unreadCount} unread</span>
                      <button
                        className="btn"
                        onClick={() =>
                          void backend.feeds.setEnabled(f.id, !f.enabled).then(() => reloadFeeds())
                        }
                      >
                        {f.enabled ? "Disable" : "Enable"}
                      </button>
                      <button className="btn" onClick={() => void backend.feeds.refresh(f.id).then(() => reloadFeeds())}>
                        Refresh
                      </button>
                      <button
                        className="btn"
                        onClick={() => {
                          if (!window.confirm(`Remove ${f.title || f.url}?`)) return;
                          void backend.feeds.remove(f.id).then(() => reloadFeeds());
                        }}
                      >
                        Remove
                      </button>
                    </div>
                  </div>
                ))}
              </div>

              <h3>Import URLs</h3>
              <p className="muted">One feed URL per line. Lines starting with # are ignored.</p>
              <textarea
                className="import-area"
                rows={8}
                value={importText}
                onChange={(e) => setImportText(e.target.value)}
                placeholder={"https://example.com/feed.xml\n# comment\nhttps://other.example/atom.xml"}
              />
              <div className="settings-row">
                <button
                  className="btn primary"
                  disabled={busy || !importText.trim()}
                  onClick={() => {
                    setBusy(true);
                    setError(null);
                    void backend.feeds
                      .importUrls(importText)
                      .then((r) => {
                        setImportResult(r);
                        return reloadFeeds();
                      })
                      .catch((e: unknown) => setError(e instanceof Error ? e.message : "Import failed"))
                      .finally(() => setBusy(false));
                  }}
                >
                  Import
                </button>
                {importResult && (
                  <span className="muted">
                    Added {importResult.added}, failed {importResult.failed}
                  </span>
                )}
              </div>
              {importResult?.errors?.length ? (
                <ul className="error-list">
                  {importResult.errors.slice(0, 20).map((e) => (
                    <li key={e}>{e}</li>
                  ))}
                </ul>
              ) : null}
            </section>
          )}

          {section === "ai" && (
            <section className="settings-section">
              <h2>Local AI (LM Studio)</h2>
              <label className="check">
                <input
                  type="checkbox"
                  checked={settings.aiEnabled}
                  onChange={(e) => void patch({ aiEnabled: e.target.checked })}
                />
                Enable AI triage
              </label>
              <label className="field">
                Base URL
                <input
                  value={settings.aiBaseUrl}
                  onChange={(e) => void patch({ aiBaseUrl: e.target.value })}
                  placeholder="http://127.0.0.1:1234/v1"
                />
              </label>
              <label className="field">
                Model id (optional)
                <input
                  value={settings.aiModel}
                  onChange={(e) => void patch({ aiModel: e.target.value })}
                  placeholder="Leave blank for LM Studio default"
                />
              </label>
              <div className="settings-row">
                <button
                  className="btn"
                  disabled={busy}
                  onClick={() => {
                    setBusy(true);
                    void backend.ai
                      .test()
                      .then(setAiTest)
                      .catch((e: unknown) => setError(e instanceof Error ? e.message : "Test failed"))
                      .finally(() => setBusy(false));
                  }}
                >
                  Test connection
                </button>
                <button
                  className="btn primary"
                  disabled={busy || !settings.aiEnabled}
                  onClick={() => {
                    setBusy(true);
                    void backend.ai
                      .scan("24h")
                      .then((r) => setAiStatus(r.status))
                      .catch((e: unknown) => setError(e instanceof Error ? e.message : "Scan failed"))
                      .finally(() => setBusy(false));
                  }}
                >
                  Scan last 24 hours
                </button>
                <button
                  className="btn primary"
                  disabled={busy || !settings.aiEnabled}
                  onClick={() => {
                    setBusy(true);
                    void backend.ai
                      .scan("7d")
                      .then((r) => setAiStatus(r.status))
                      .catch((e: unknown) => setError(e instanceof Error ? e.message : "Scan failed"))
                      .finally(() => setBusy(false));
                  }}
                >
                  Scan last 7 days
                </button>
                <button
                  className="btn primary"
                  disabled={busy || !settings.aiEnabled}
                  onClick={() => {
                    setBusy(true);
                    void backend.ai
                      .scan("missed")
                      .then((r) => setAiStatus(r.status))
                      .catch((e: unknown) => setError(e instanceof Error ? e.message : "Scan failed"))
                      .finally(() => setBusy(false));
                  }}
                >
                  Scan missed / skipped
                </button>
              </div>
              {aiTest && (
                <p className={aiTest.ok ? "muted" : "error"}>
                  {aiTest.ok ? `OK — ${aiTest.message}` : aiTest.message}
                  {aiTest.models?.length ? ` · models: ${aiTest.models.slice(0, 5).join(", ")}` : ""}
                </p>
              )}
              {aiStatus && (
                <p className="muted">
                  AI queue: {aiStatus.processed}/{aiStatus.total}
                  {aiStatus.pending > 0 ? ` · pending: ${aiStatus.pending}` : ""}
                  {aiStatus.failed > 0 ? ` · failed: ${aiStatus.failed}` : ""}
                  {aiStatus.running ? " (running)" : ""}
                  {aiStatus.lastError ? ` · last error: ${aiStatus.lastError}` : ""}
                </p>
              )}
              {aiStatus && aiStatus.failed > 0 && (
                <div className="settings-row">
                  <button
                    className="btn"
                    disabled={busy}
                    onClick={() => {
                      setBusy(true);
                      void backend.ai
                        .retryFailed()
                        .then((r) => setAiStatus(r.status))
                        .catch((e: unknown) => setError(e instanceof Error ? e.message : "Retry failed"))
                        .finally(() => setBusy(false));
                    }}
                  >
                    Retry failed
                  </button>
                </div>
              )}
              <div className="ai-log-panel" ref={logPanelRef}>
                {aiLogs.length === 0 ? (
                  <div className="muted">No AI log entries yet.</div>
                ) : (
                  aiLogs.map((entry) => (
                    <div key={entry.id} className="ai-log-line">
                      <span className="ai-log-ts">{new Date(entry.ts).toLocaleTimeString()}</span>
                      <span className={`ai-log-level ai-log-level-${entry.level}`}>{entry.level}</span>
                      <span className="ai-log-message">{entry.message}</span>
                    </div>
                  ))
                )}
              </div>
              <p className="muted">
                When enabled, newly fetched articles are queued for priority scoring and story grouping via
                LM Studio tool calls.
              </p>
            </section>
          )}
        </div>
      </div>
    </div>
  );
}
