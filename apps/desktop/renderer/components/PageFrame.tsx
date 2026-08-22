import { useEffect, useMemo, useState } from "react";
import { PAGE_FRAME_SANDBOX, preparePageFrameHtml } from "../lib/pageFrame";

type Props = {
  html: string;
  /** Original article URL — used if document lacks a usable base. */
  pageUrl: string;
  title?: string;
};

/** Renders a saved full web page in a sandboxed iframe (no scripts; CSS/images via base href). */
export function PageFrame({ html, pageUrl, title = "Article page" }: Props) {
  const [src, setSrc] = useState<string | null>(null);

  const prepared = useMemo(() => preparePageFrameHtml(html, pageUrl), [html, pageUrl]);

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
      sandbox={PAGE_FRAME_SANDBOX}
      referrerPolicy="no-referrer-when-downgrade"
    />
  );
}
