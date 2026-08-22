import { Readability } from "@mozilla/readability";
import type { Article } from "@rss-reader/shared";
import { parseHTML } from "linkedom";
import { sanitizeArticleHtml, stripHtml } from "./html";

export type ContentTab = "primary" | "reader" | "secondary";

export type ReaderArticle = {
  title: string;
  byline: string;
  contentHtml: string;
};

export function readerSourceHtml(
  article: Pick<Article, "crawledContent" | "liveContent" | "rssContent" | "content" | "summary">,
): string | null {
  const crawled = article.crawledContent.trim();
  if (crawled) return crawled;
  const live = article.liveContent.trim();
  if (live) return live;
  const rss = (article.rssContent || article.content || article.summary).trim();
  return rss || null;
}

function parseHtmlDocument(html: string): Document {
  if (typeof DOMParser !== "undefined") {
    return new DOMParser().parseFromString(html, "text/html");
  }
  return parseHTML(html).document as unknown as Document;
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
