import { useEffect, useMemo, useState } from "react";

type Props = {
  html: string;
  /** Original article URL — used if document lacks a usable base. */
  pageUrl: string;
  title?: string;
};

/** Renders a saved full web page in a sandboxed iframe (scripts allowed; no parent access). */
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
      sandbox="allow-scripts allow-forms allow-popups allow-popups-to-escape-sandbox allow-downloads"
      referrerPolicy="no-referrer"
    />
  );
}

function ensureBase(html: string, pageUrl: string): string {
  if (!pageUrl || /<base\s/i.test(html)) {
    return html;
  }
  const base = `<base href="${escapeAttr(pageUrl)}">`;
  const headMatch = html.match(/<head[^>]*>/i);
  if (headMatch && headMatch.index != null) {
    const insertAt = headMatch.index + headMatch[0].length;
    return html.slice(0, insertAt) + base + html.slice(insertAt);
  }
  const htmlMatch = html.match(/<html[^>]*>/i);
  if (htmlMatch && htmlMatch.index != null) {
    const insertAt = htmlMatch.index + htmlMatch[0].length;
    return html.slice(0, insertAt) + `<head>${base}</head>` + html.slice(insertAt);
  }
  return `<head>${base}</head>${html}`;
}

function escapeAttr(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}
