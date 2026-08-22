import { describe, expect, test } from "bun:test";
import { extractReaderArticle, readerSourceHtml } from "./readerMode";

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
