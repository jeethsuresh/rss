import { describe, expect, test } from "bun:test";
import { sanitizeArticleHtml, stripHtml } from "./html";

describe("html sanitization", () => {
  test("strips scripts", () => {
    const html = sanitizeArticleHtml(`<p>Hi</p><script>alert(1)</script>`);
    expect(html).toContain("Hi");
    expect(html.toLowerCase()).not.toContain("<script");
  });

  test("stripHtml returns text", () => {
    expect(stripHtml("<b>Hello</b> world")).toBe("Hello world");
  });
});
