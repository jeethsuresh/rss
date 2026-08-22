import { beforeAll, describe, expect, test } from "bun:test";
import type { Article } from "@rss-reader/shared";
import { parseHTML } from "linkedom";
import { extractReaderArticle, isFullBleedTab, readerPaneModel, readerSourceHtml } from "./readerMode";

beforeAll(() => {
  class LinkedomParser {
    parseFromString(html: string, _type: string) {
      return parseHTML(html).document;
    }
  }
  (globalThis as { DOMParser?: typeof LinkedomParser }).DOMParser = LinkedomParser;
});

function article(partial: Partial<Article> = {}): Article {
  return {
    id: "a1",
    feedId: "feed",
    title: "Rural broadband funding",
    url: "https://www.cbc.ca/news/x",
    author: "",
    content: "",
    summary: "",
    rssContent: "",
    crawledContent: "",
    liveContent: "",
    crawlStatus: "none",
    crawlError: "",
    crawlUnreliable: false,
    publishedAt: null,
    updatedAt: null,
    externalId: "a1",
    isRead: false,
    isStarred: false,
    isReadLater: false,
    priority: "none",
    discoveredAt: "2026-08-22T12:00:00.000Z",
    ...partial,
  };
}

const noisyPage = `<!doctype html><html><body>
<nav>Home Sports Weather</nav>
<article>
  <h1>Rural broadband funding</h1>
  <p>By Jane Doe</p>
  <p>${"The province announced new funding for rural broadband so towns can finally get reliable service. ".repeat(8)}</p>
  <p>${"Officials said construction starts next spring and households can apply for subsidies. ".repeat(8)}</p>
</article>
<aside>Subscribe to our newsletter<script>fetch("https://subscriptions.cbc.ca/api/newsletter/get_active_newsletters?source=")</script></aside>
</body></html>`;

describe("readerSourceHtml", () => {
  test("prefers stored readability extract when status is ok", () => {
    expect(
      readerSourceHtml({
        extractStatus: "ok",
        readerContent: "<p>stored</p>",
        crawledContent: "<p>crawl</p>",
        liveContent: "<p>live</p>",
        rssContent: "<p>rss</p>",
        content: "",
        summary: "",
      }),
    ).toBe("<p>stored</p>");
  });

  test("prefers crawled, then live, then RSS fields", () => {
    expect(
      readerSourceHtml({
        crawledContent: "<p>crawl</p>",
        liveContent: "<p>live</p>",
        rssContent: "<p>rss</p>",
        content: "<p>content</p>",
        summary: "sum",
      }),
    ).toBe("<p>crawl</p>");
    expect(
      readerSourceHtml({
        crawledContent: "  ",
        liveContent: "<p>live</p>",
        rssContent: "<p>rss</p>",
        content: "",
        summary: "",
      }),
    ).toBe("<p>live</p>");
    expect(
      readerSourceHtml({
        crawledContent: "",
        liveContent: "",
        rssContent: "",
        content: "<p>content</p>",
        summary: "sum",
      }),
    ).toBe("<p>content</p>");
    expect(
      readerSourceHtml({
        crawledContent: "",
        liveContent: "",
        rssContent: "",
        content: "",
        summary: "",
      }),
    ).toBeNull();
  });
});

describe("extractReaderArticle", () => {
  test("returns null for empty html", () => {
    expect(extractReaderArticle("", "https://www.cbc.ca/news/x")).toBeNull();
  });

  test("extracts article text and drops newsletter chrome", () => {
    const result = extractReaderArticle(noisyPage, "https://www.cbc.ca/news/x");
    expect(result).not.toBeNull();
    expect(result!.contentHtml.toLowerCase()).not.toContain("<script");
    expect(result!.contentHtml.toLowerCase()).toContain("rural broadband");
    expect(result!.contentHtml.toLowerCase()).not.toContain("get_active_newsletters");
  });
});

describe("readerPaneModel", () => {
  test("shows pending when crawl is in progress and there is no HTML", () => {
    const model = readerPaneModel(article({ crawlStatus: "pending" }));
    expect(model).toEqual({
      kind: "status",
      message: "Crawl in progress…",
      recrawl: false,
    });
  });

  test("asks to recrawl when there is no page HTML", () => {
    const model = readerPaneModel(article({ crawlStatus: "none" }));
    expect(model).toEqual({
      kind: "status",
      message: "No page to extract yet.",
      recrawl: true,
    });
  });

  test("falls back to sanitized RSS HTML when Readability returns null", () => {
    const model = readerPaneModel(article({ rssContent: "<p>Just a blurb.</p><script>alert(1)</script>" }));
    if (model.kind !== "article") throw new Error("expected article");
    expect(model.contentHtml).toContain("Just a blurb");
    expect(model.contentHtml.toLowerCase()).not.toContain("<script");
    expect(model.byline).toBe("");
  });

  test("extracts crawled pages into article HTML", () => {
    const model = readerPaneModel(article({ crawledContent: noisyPage, crawlStatus: "ok" }));
    if (model.kind !== "article") throw new Error("expected article");
    expect(model.contentHtml.toLowerCase()).toContain("rural broadband");
    expect(model.contentHtml.toLowerCase()).not.toContain("get_active_newsletters");
  });
});

describe("isFullBleedTab", () => {
  test("Reader is never full-bleed; Live/Saved crawl and Full page are", () => {
    expect(isFullBleedTab("reader", false)).toBe(false);
    expect(isFullBleedTab("reader", true)).toBe(false);
    expect(isFullBleedTab("primary", false)).toBe(false);
    expect(isFullBleedTab("primary", true)).toBe(true);
    expect(isFullBleedTab("secondary", false)).toBe(true);
    expect(isFullBleedTab("secondary", true)).toBe(true);
  });
});

