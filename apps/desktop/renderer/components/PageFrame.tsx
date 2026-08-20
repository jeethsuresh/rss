import { useEffect, useMemo, useState } from "react";

type Props = {
  html: string;
  /** Original article URL — used if document lacks a usable base. */
  pageUrl: string;
  title?: string;
};

/** Renders a saved full web page in a sandboxed iframe (scripts allowed; opaque origin — no parent access). */
export function PageFrame({ html, pageUrl, title = "Article page" }: Props) {
  const [src, setSrc] = useState<string | null>(null);

  const prepared = useMemo(() => ensureBase(html, pageUrl), [html, pageUrl]);

  useEffect(() => {
    const blob = new Blob([prepared], { type: "text/html;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    setSrc(url);
    return () => URL.revokeObjectURL(url);
  }, [prepared]);

  if (!src) {
    return <p className="muted">Loading page…</p>;
  }

  return (
    <iframe
      className="page-frame"
      title={title}
      src={src}
      // Do NOT set allow-same-origin with allow-scripts: blob URLs inherit the
      // app origin, and page scripts can then call history.replaceState / touch
      // the parent (SecurityError + broken Vite HMR when doc URL is blob:).
      sandbox="allow-scripts allow-forms allow-popups allow-popups-to-escape-sandbox allow-downloads"
      referrerPolicy="no-referrer-when-downgrade"
    />
  );
}

function ensureBase(html: string, pageUrl: string): string {
  const href = baseHrefFor(pageUrl);
  if (!href) {
    return html;
  }
  // Always prefer our origin-rooted base so relative CSS/JS resolve reliably.
  const withoutBase = html.replace(/<base\b[^>]*>/gi, "");
  const base = `<base href="${escapeAttr(href)}">`;
  const headMatch = withoutBase.match(/<head[^>]*>/i);
  if (headMatch && headMatch.index != null) {
    const insertAt = headMatch.index + headMatch[0].length;
    return withoutBase.slice(0, insertAt) + base + withoutBase.slice(insertAt);
  }
  const htmlMatch = withoutBase.match(/<html[^>]*>/i);
  if (htmlMatch && htmlMatch.index != null) {
    const insertAt = htmlMatch.index + htmlMatch[0].length;
    return withoutBase.slice(0, insertAt) + `<head>${base}</head>` + withoutBase.slice(insertAt);
  }
  return `<head>${base}</head>${withoutBase}`;
}

/** Prefer site origin so `/styles.css` and relative assets resolve like a normal page load. */
function baseHrefFor(pageUrl: string): string {
  try {
    const u = new URL(pageUrl);
    return `${u.origin}/`;
  } catch {
    return pageUrl;
  }
}

function escapeAttr(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}
