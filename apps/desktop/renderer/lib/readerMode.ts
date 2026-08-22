import { Readability } from "@mozilla/readability";
import type { Article } from "@rss-reader/shared";
import { sanitizeArticleHtml, stripHtml } from "./html";

export type ContentTab = "primary" | "reader" | "secondary";

export function isFullBleedTab(tab: ContentTab, isReadLater: boolean): boolean {
  switch (tab) {
    case "reader":
      return false;
    case "secondary":
      return true;
    case "primary":
      return isReadLater;
    default: {
      const _exhaustive: never = tab;
      return _exhaustive;
    }
  }
}

export type ReaderArticle = {
  title: string;
  byline: string;
  contentHtml: string;
};

export function readerSourceHtml(
  article: Pick<
    Article,
    "crawledContent" | "liveContent" | "rssContent" | "content" | "summary" | "readerContent" | "extractStatus"
  >,
): string | null {
  if (article.extractStatus === "ok") {
    const stored = (article.readerContent ?? "").trim();
    if (stored) return stored;
  }
  const crawled = article.crawledContent.trim();
  if (crawled) return crawled;
  const live = article.liveContent.trim();
  if (live) return live;
  const rss = (article.rssContent || article.content || article.summary).trim();
  return rss || null;
}

function parseHtmlDocument(html: string): Document {
  return new DOMParser().parseFromString(html, "text/html");
}

function injectBase(doc: Document, pageUrl: string): void {
  if (!pageUrl) return;
  const base = doc.createElement("base");
  base.setAttribute("href", pageUrl);
  const head = doc.head ?? doc.querySelector("head");
  if (head) {
    head.insertBefore(base, head.firstChild);
    return;
  }
  const htmlEl = doc.documentElement;
  if (htmlEl) {
    htmlEl.insertBefore(base, htmlEl.firstChild);
  }
}

export type ReaderPaneModel =
  | { kind: "status"; message: string; recrawl: boolean }
  | { kind: "article"; byline: string; contentHtml: string };

export function readerPaneModel(article: Article): ReaderPaneModel {
  if (article.extractStatus === "ok" && (article.readerContent ?? "").trim()) {
    const contentHtml = sanitizeArticleHtml(article.readerContent ?? "");
    if (!stripHtml(contentHtml)) {
      return { kind: "status", message: "Couldn't extract an article.", recrawl: true };
    }
    return { kind: "article", byline: "", contentHtml };
  }
  const source = readerSourceHtml(article);
  if (!source) {
    if (article.crawlStatus === "pending") {
      return { kind: "status", message: "Crawl in progress…", recrawl: false };
    }
    return { kind: "status", message: "No page to extract yet.", recrawl: true };
  }
  const extracted = extractReaderArticle(source, article.url);
  const contentHtml = extracted?.contentHtml || sanitizeArticleHtml(source);
  if (!stripHtml(contentHtml)) {
    return { kind: "status", message: "Couldn't extract an article.", recrawl: true };
  }
  return { kind: "article", byline: extracted?.byline ?? "", contentHtml };
}

export function extractReaderArticle(html: string, pageUrl: string): ReaderArticle | null {
  if (!html.trim()) return null;
  const doc = parseHtmlDocument(html);
  injectBase(doc, pageUrl);
  let parsed: ReturnType<Readability["parse"]> = null;
  try {
    parsed = new Readability(doc, { charThreshold: 140 }).parse();
  } catch {
    return null;
  }
  const raw = parsed?.content;
  if (typeof raw !== "string" || !raw.trim()) return null;
  const contentHtml = sanitizeArticleHtml(raw);
  if (!stripHtml(contentHtml)) return null;
  return {
    title: parsed?.title?.trim() ?? "",
    byline: parsed?.byline?.trim() ?? "",
    contentHtml,
  };
}
