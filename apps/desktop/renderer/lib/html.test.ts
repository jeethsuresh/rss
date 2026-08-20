import { describe, expect, test } from "bun:test";
import { decodeHtmlEntities, sanitizeArticleHtml, stripHtml } from "./html";

describe("html sanitization", () => {
  test("strips scripts", () => {
    const html = sanitizeArticleHtml(`<p>Hi</p><script>alert(1)</script>`);
    expect(html).toContain("Hi");
    expect(html.toLowerCase()).not.toContain("<script");
  });

  test("stripHtml returns text", () => {
    expect(stripHtml("<b>Hello</b> world")).toBe("Hello world");
  });

  test("decodeHtmlEntities decodes apostrophes and quotes", () => {
    expect(decodeHtmlEntities("I like &#x27;em thick")).toBe("I like 'em thick");
    expect(decodeHtmlEntities("Tom &amp; Jerry")).toBe("Tom & Jerry");
  });
});
