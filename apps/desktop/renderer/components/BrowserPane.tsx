import { createElement, useEffect, useRef, useState, type FormEvent } from "react";
import { GENERIC_ERROR_MESSAGE } from "../lib/errors";

type WebviewElement = HTMLElement & {
  src: string;
  canGoBack(): boolean;
  canGoForward(): boolean;
  goBack(): void;
  goForward(): void;
  reload(): void;
  loadURL(url: string): Promise<void>;
  getURL(): string;
};

type Props = {
  initialUrl: string;
  onClose: () => void;
  onSave: (url: string) => Promise<void>;
};

function normalizedUrl(value: string): string | null {
  try {
    const url = new URL(value.trim());
    return url.protocol === "http:" || url.protocol === "https:" ? url.href : null;
  } catch {
    return null;
  }
}

export function BrowserPane({ initialUrl, onClose, onSave }: Props) {
  const webviewRef = useRef<WebviewElement | null>(null);
  const [url, setUrl] = useState(initialUrl);
  const [draft, setDraft] = useState(initialUrl);
  const [canGoBack, setCanGoBack] = useState(false);
  const [canGoForward, setCanGoForward] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const webview = webviewRef.current;
    if (!webview) return;
    const sync = () => {
      const next = webview.getURL?.() || initialUrl;
      setUrl(next);
      setDraft(next);
      setCanGoBack(webview.canGoBack());
      setCanGoForward(webview.canGoForward());
    };
    const started = () => setLoading(true);
    const stopped = () => {
      setLoading(false);
      sync();
    };
    const popup = (event: Event & { url?: string }) => {
      const next = event.url && normalizedUrl(event.url);
      if (next) void webview.loadURL(next);
    };
    webview.addEventListener("dom-ready", sync);
    webview.addEventListener("did-navigate", sync);
    webview.addEventListener("did-navigate-in-page", sync);
    webview.addEventListener("did-start-loading", started);
    webview.addEventListener("did-stop-loading", stopped);
    webview.addEventListener("new-window", popup as EventListener);
    return () => {
      webview.removeEventListener("dom-ready", sync);
      webview.removeEventListener("did-navigate", sync);
      webview.removeEventListener("did-navigate-in-page", sync);
      webview.removeEventListener("did-start-loading", started);
      webview.removeEventListener("did-stop-loading", stopped);
      webview.removeEventListener("new-window", popup as EventListener);
    };
  }, [initialUrl]);

  const navigate = (event: FormEvent) => {
    event.preventDefault();
    const next = normalizedUrl(draft);
    if (!next) {
      setError("Enter a valid http(s) URL");
      return;
    }
    setError(null);
    void webviewRef.current?.loadURL(next);
  };

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      await onSave(url);
    } catch (e) {
      setError(GENERIC_ERROR_MESSAGE);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="browser-pane">
      <div className="browser-toolbar">
        <button className="btn browser-icon" disabled={!canGoBack} onClick={() => webviewRef.current?.goBack()} aria-label="Back">←</button>
        <button className="btn browser-icon" disabled={!canGoForward} onClick={() => webviewRef.current?.goForward()} aria-label="Forward">→</button>
        <button className="btn browser-icon" onClick={() => webviewRef.current?.reload()} aria-label={loading ? "Reloading" : "Refresh"}>↻</button>
        <form className="browser-address" onSubmit={navigate}>
          <input aria-label="URL" value={draft} onChange={(event) => setDraft(event.target.value)} spellCheck={false} />
        </form>
        <button className="btn" disabled={saving} onClick={() => void save()}>{saving ? "Saving…" : "Save to Read Later"}</button>
        <button className="btn primary" onClick={onClose}>Close</button>
      </div>
      {error ? <div className="browser-error">{error}</div> : null}
      <div className="browser-surface">
        {createElement("webview", {
          ref: (node: WebviewElement | null) => { webviewRef.current = node; },
          className: "browser-webview",
          src: initialUrl,
          allowpopups: "true",
        })}
      </div>
    </div>
  );
}
