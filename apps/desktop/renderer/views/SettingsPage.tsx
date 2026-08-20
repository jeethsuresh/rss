import { useCallback, useEffect, useRef, useState, type MouseEvent as ReactMouseEvent } from "react";
import type {
  AILogEntry,
  AIStatus,
  AITestResult,
  BackendEventName,
  Feed,
  FeedImportResult,
  MlbTeam,
  ReaderBackend,
  Settings,
} from "@rss-reader/shared";

type Props = {
  backend: ReaderBackend;
  settings: Settings;
  onSettings: (s: Settings) => void;
  onClose: () => void;
  applyTheme: (theme: Settings["theme"]) => void;
  initialSection?: "general" | "feeds" | "ai" | "sports";
};

type SettingsSection = "general" | "feeds" | "ai" | "sports";

export function SettingsPage({
  backend,
  settings,
  onSettings,
  onClose,
  applyTheme,
  initialSection = "general",
}: Props) {
  const [feeds, setFeeds] = useState<Feed[]>([]);
  const [feedFilter, setFeedFilter] = useState("");
  const [importText, setImportText] = useState("");
  const [importResult, setImportResult] = useState<FeedImportResult | null>(null);
  const [aiStatus, setAiStatus] = useState<AIStatus | null>(null);
  const [aiTest, setAiTest] = useState<AITestResult | null>(null);
  const [aiLogs, setAiLogs] = useState<AILogEntry[]>([]);
  const logPanelRef = useRef<HTMLDivElement>(null);
  const [busy, setBusy] = useState(false);
  const [aiTesting, setAiTesting] = useState(false);
  const [generalSaving, setGeneralSaving] = useState(false);
  const [aiSaving, setAiSaving] = useState(false);
  const [tokensSaving, setTokensSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [section, setSection] = useState<SettingsSection>(initialSection);
  const [mlbTeams, setMlbTeams] = useState<MlbTeam[]>([]);
  const [followedIds, setFollowedIds] = useState<number[]>([]);
  const [teamFilter, setTeamFilter] = useState("");

  const [pollDraft, setPollDraft] = useState(String(settings.defaultPollIntervalSeconds));
  const [aiBaseUrlDraft, setAiBaseUrlDraft] = useState(settings.aiBaseUrl);
  const [aiModelDraft, setAiModelDraft] = useState(settings.aiModel);
  const [pandaTokenDraft, setPandaTokenDraft] = useState(settings.pandaScoreApiToken ?? "");
  const [stratzTokenDraft, setStratzTokenDraft] = useState(settings.stratzApiToken ?? "");

  // Refs so Save always reads the latest draft even if a blur/re-render races the click.
  const pollDraftRef = useRef(pollDraft);
  const aiBaseUrlDraftRef = useRef(aiBaseUrlDraft);
  const aiModelDraftRef = useRef(aiModelDraft);
  const pandaTokenDraftRef = useRef(pandaTokenDraft);
  const stratzTokenDraftRef = useRef(stratzTokenDraft);
  pollDraftRef.current = pollDraft;
  aiBaseUrlDraftRef.current = aiBaseUrlDraft;
  aiModelDraftRef.current = aiModelDraft;
  pandaTokenDraftRef.current = pandaTokenDraft;
  stratzTokenDraftRef.current = stratzTokenDraft;

  const pollParsed = Number(pollDraft);
  const pollValid = Number.isFinite(pollParsed) && pollParsed >= 60;
  const generalDirty = pollValid && pollParsed !== settings.defaultPollIntervalSeconds;
  const aiDirty =
    aiBaseUrlDraft.trim() !== (settings.aiBaseUrl ?? "").trim() ||
    aiModelDraft.trim() !== (settings.aiModel ?? "").trim();
  const sportsTokensDirty =
    pandaTokenDraft.trim() !== (settings.pandaScoreApiToken ?? "").trim() ||
    stratzTokenDraft.trim() !== (settings.stratzApiToken ?? "").trim();

  const reloadFeeds = useCallback(async () => {
    setFeeds(await backend.feeds.list());
  }, [backend]);

  useEffect(() => {
    void reloadFeeds();
    void backend.ai.status().then(setAiStatus).catch(() => undefined);
    void backend.ai.logs(200).then(setAiLogs).catch(() => undefined);
  }, [backend, reloadFeeds]);

  useEffect(() => {
    if (section !== "sports") return;
    void Promise.all([backend.sports.teams(), backend.sports.followedGet()])
      .then(([t, f]) => {
        setMlbTeams(t ?? []);
        setFollowedIds(f ?? []);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load MLB teams"));
  }, [backend, section]);

  useEffect(() => {
    setSection(initialSection);
  }, [initialSection]);

  // Sync drafts from props only when that field group is clean (never clobber unsaved edits).
  useEffect(() => {
    if (!generalDirty) setPollDraft(String(settings.defaultPollIntervalSeconds));
  }, [settings.defaultPollIntervalSeconds, generalDirty]);

  useEffect(() => {
    if (!aiDirty) {
      setAiBaseUrlDraft(settings.aiBaseUrl);
      setAiModelDraft(settings.aiModel);
    }
  }, [settings.aiBaseUrl, settings.aiModel, aiDirty]);

  useEffect(() => {
    if (!sportsTokensDirty) {
      setPandaTokenDraft(settings.pandaScoreApiToken ?? "");
      setStratzTokenDraft(settings.stratzApiToken ?? "");
    }
  }, [settings.pandaScoreApiToken, settings.stratzApiToken, sportsTokensDirty]);

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
        case "article.removed":
        case "story.updated":
        case "sync.status":
        case "sports.game.updated":
        case "sports.f1.race.updated":
        case "sports.dota.match.updated":
        case "sports.refresh":
        case "sports.cache.updated":
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
    return next;
  };

  /** Prevent input blur from disabling the Save button before click fires. */
  const keepFocusForSave = (e: ReactMouseEvent) => {
    e.preventDefault();
  };

  const saveGeneral = async () => {
    const raw = pollDraftRef.current;
    const n = Number(raw);
    if (!Number.isFinite(n) || n < 60) {
      setError("Poll interval must be at least 60 seconds");
      return;
    }
    setError(null);
    setGeneralSaving(true);
    try {
      await patch({ defaultPollIntervalSeconds: n });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not save settings");
    } finally {
      setGeneralSaving(false);
    }
  };

  const saveAiConnection = async () => {
    setError(null);
    setAiSaving(true);
    try {
      await patch({
        aiBaseUrl: aiBaseUrlDraftRef.current.trim(),
        aiModel: aiModelDraftRef.current.trim(),
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not save AI settings");
    } finally {
      setAiSaving(false);
    }
  };

  const testAiConnection = async () => {
    setError(null);
    setAiTest(null);
    setAiTesting(true);
    try {
      const url = aiBaseUrlDraftRef.current.trim();
      const model = aiModelDraftRef.current.trim();
      const needsSave =
        url !== (settings.aiBaseUrl ?? "").trim() || model !== (settings.aiModel ?? "").trim();
      if (needsSave) {
        await patch({ aiBaseUrl: url, aiModel: model });
      }
      const result = await backend.ai.test();
      setAiTest(result);
    } catch (e) {
      setAiTest({
        ok: false,
        message: e instanceof Error ? e.message : "Test failed",
      });
    } finally {
      setAiTesting(false);
    }
  };

  const saveSportsTokens = async () => {
    setError(null);
    setTokensSaving(true);
    try {
      await patch({
        pandaScoreApiToken: pandaTokenDraftRef.current.trim(),
        stratzApiToken: stratzTokenDraftRef.current.trim(),
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not save API tokens");
    } finally {
      setTokensSaving(false);
    }
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
        <button type="button" className="btn" onClick={onClose}>
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
              ["sports", "Sports"],
              ["ai", "AI"],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              type="button"
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
              <p className="muted">
                Dropdowns and toggles save immediately. Number fields keep a local draft until you
                press Save.
              </p>
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
                  step={60}
                  value={pollDraft}
                  onChange={(e) => setPollDraft(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      void saveGeneral();
                    }
                  }}
                />
              </label>
              {!pollValid ? (
                <p className="error">Enter a number ≥ 60.</p>
              ) : (
                <div className="settings-row">
                  <button
                    type="button"
                    className="btn"
                    disabled={generalSaving || !generalDirty}
                    onMouseDown={keepFocusForSave}
                    onClick={() => void saveGeneral()}
                  >
                    {generalSaving ? "Saving…" : "Save poll interval"}
                  </button>
                  {generalDirty ? <span className="muted">Unsaved changes</span> : null}
                </div>
              )}
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
                  type="button"
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
                        type="button"
                        className="btn"
                        onClick={() =>
                          void backend.feeds.setEnabled(f.id, !f.enabled).then(() => reloadFeeds())
                        }
                      >
                        {f.enabled ? "Disable" : "Enable"}
                      </button>
                      <button
                        type="button"
                        className="btn"
                        onClick={() => void backend.feeds.refresh(f.id).then(() => reloadFeeds())}
                      >
                        Refresh
                      </button>
                      <button
                        type="button"
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
                  type="button"
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
                  {busy ? "Importing…" : "Import"}
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

          {section === "sports" && (
            <section className="settings-section">
              <h2>Dota 2 APIs</h2>
              <p className="muted">
                Edit tokens, then Save. Stored locally in the app database (env vars remain a fallback
                if empty).
              </p>
              <label className="field field-wide">
                PandaScore API token
                <input
                  type="password"
                  autoComplete="off"
                  spellCheck={false}
                  value={pandaTokenDraft}
                  onChange={(e) => setPandaTokenDraft(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      void saveSportsTokens();
                    }
                  }}
                  placeholder="Required for Dota events & matches"
                />
              </label>
              <label className="field field-wide">
                STRATZ API token
                <input
                  type="password"
                  autoComplete="off"
                  spellCheck={false}
                  value={stratzTokenDraft}
                  onChange={(e) => setStratzTokenDraft(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      void saveSportsTokens();
                    }
                  }}
                  placeholder="Optional — game heroes, bans, player stats"
                />
              </label>
              <div className="settings-row">
                <button
                  type="button"
                  className="btn primary"
                  disabled={tokensSaving || !sportsTokensDirty}
                  onMouseDown={keepFocusForSave}
                  onClick={() => void saveSportsTokens()}
                >
                  {tokensSaving ? "Saving…" : "Save tokens"}
                </button>
                {sportsTokensDirty ? <span className="muted">Unsaved changes</span> : null}
                {!sportsTokensDirty && !tokensSaving ? (
                  <span className="muted">
                    {pandaTokenDraft.trim() ? "PandaScore set" : "PandaScore empty"}
                    {" · "}
                    {stratzTokenDraft.trim() ? "STRATZ set" : "STRATZ empty"}
                  </span>
                ) : null}
              </div>

              <h2>Baseball (MLB)</h2>
              <p className="muted">
                Follow one or more MLB teams. Schedules and live games appear under Baseball in the Sports
                tab.
              </p>
              <input
                className="search"
                placeholder="Filter teams…"
                value={teamFilter}
                onChange={(e) => setTeamFilter(e.target.value)}
              />
              <div className="feed-table sports-team-table">
                {mlbTeams
                  .filter((t) => {
                    const q = teamFilter.trim().toLowerCase();
                    if (!q) return true;
                    return (
                      t.name.toLowerCase().includes(q) ||
                      t.abbreviation.toLowerCase().includes(q) ||
                      (t.shortName ?? "").toLowerCase().includes(q)
                    );
                  })
                  .map((t) => {
                    const on = followedIds.includes(t.id);
                    return (
                      <label key={t.id} className="feed-table-row sports-team-check">
                        <input
                          type="checkbox"
                          checked={on}
                          onChange={() => {
                            void backend.sports.followedToggle(t.id).then((ids) => setFollowedIds(ids ?? []));
                          }}
                        />
                        {t.logoUrl ? <img src={t.logoUrl} alt="" className="sports-logo" /> : null}
                        <div>
                          <strong>{t.name}</strong>
                          <div className="muted">{t.abbreviation}</div>
                        </div>
                      </label>
                    );
                  })}
              </div>
            </section>
          )}

          {section === "ai" && (
            <section className="settings-section">
              <h2>Local AI (LM Studio)</h2>
              <p className="muted">
                Point the reader at a local OpenAI-compatible server. Nothing is written until you press
                Save (or Test, which saves first if needed).
              </p>
              <label className="check">
                <input
                  type="checkbox"
                  checked={settings.aiEnabled}
                  onChange={(e) => void patch({ aiEnabled: e.target.checked })}
                />
                Enable AI triage
              </label>
              <label className="field field-wide">
                Base URL
                <input
                  type="url"
                  inputMode="url"
                  autoComplete="off"
                  spellCheck={false}
                  value={aiBaseUrlDraft}
                  onChange={(e) => {
                    setAiBaseUrlDraft(e.target.value);
                    setAiTest(null);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      void saveAiConnection();
                    }
                  }}
                  placeholder="http://127.0.0.1:1234/v1"
                />
              </label>
              <label className="field field-wide">
                Model id (optional)
                <input
                  autoComplete="off"
                  spellCheck={false}
                  value={aiModelDraft}
                  onChange={(e) => {
                    setAiModelDraft(e.target.value);
                    setAiTest(null);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      void saveAiConnection();
                    }
                  }}
                  placeholder="Leave blank for LM Studio default"
                />
              </label>
              <div className="settings-row ai-connection-actions">
                <button
                  type="button"
                  className="btn"
                  disabled={aiSaving || !aiDirty}
                  onMouseDown={keepFocusForSave}
                  onClick={() => void saveAiConnection()}
                >
                  {aiSaving ? "Saving…" : "Save connection"}
                </button>
                <button
                  type="button"
                  className="btn primary"
                  disabled={aiTesting || aiSaving}
                  onMouseDown={keepFocusForSave}
                  onClick={() => void testAiConnection()}
                >
                  {aiTesting ? "Testing…" : "Test connection"}
                </button>
                {aiDirty ? <span className="muted">Unsaved changes</span> : null}
              </div>
              <div
                className={`ai-test-status ${
                  aiTesting
                    ? "ai-test-status-pending"
                    : aiTest
                      ? aiTest.ok
                        ? "ai-test-status-ok"
                        : "ai-test-status-error"
                      : ""
                }`}
                role="status"
                aria-live="polite"
              >
                {aiTesting ? (
                  <>
                    <span className="sports-spinner-dot" aria-hidden />
                    <span>Testing connection to {aiBaseUrlDraft.trim() || "AI server"}…</span>
                  </>
                ) : aiTest ? (
                  <span>
                    {aiTest.ok ? "Connected — " : "Failed — "}
                    {aiTest.message}
                    {aiTest.models?.length
                      ? ` · models: ${aiTest.models.slice(0, 5).join(", ")}`
                      : ""}
                  </span>
                ) : (
                  <span className="muted">
                    Save your URL, then test to verify the server is reachable.
                  </span>
                )}
              </div>
              <div className="settings-row">
                <button
                  type="button"
                  className="btn primary"
                  disabled={busy || aiTesting || !settings.aiEnabled}
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
                  type="button"
                  className="btn primary"
                  disabled={busy || aiTesting || !settings.aiEnabled}
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
                  type="button"
                  className="btn primary"
                  disabled={busy || aiTesting || !settings.aiEnabled}
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
                    type="button"
                    className="btn"
                    disabled={busy || aiTesting}
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
                    <div
                      key={entry.id}
                      className={`ai-log-line ${entry.level === "error" ? "ai-log-line-error" : ""}`}
                    >
                      <span className="ai-log-ts">{new Date(entry.ts).toLocaleTimeString()}</span>
                      <span className={`ai-log-level ai-log-level-${entry.level}`}>{entry.level}</span>
                      <span className="ai-log-message">
                        {entry.message}
                        {entry.detail && !entry.message.includes(entry.detail) ? (
                          <span className="ai-log-detail"> — {entry.detail}</span>
                        ) : null}
                        {entry.articleId ? (
                          <span className="ai-log-article"> · {entry.articleId.slice(0, 8)}</span>
                        ) : null}
                      </span>
                    </div>
                  ))
                )}
              </div>
              <p className="muted">
                When enabled, newly fetched articles are queued for priority scoring and story grouping via
                your local model. No cloud calls.
              </p>
            </section>
          )}
        </div>
      </div>
    </div>
  );
}
