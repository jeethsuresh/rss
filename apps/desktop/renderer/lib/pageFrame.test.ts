import { describe, expect, test } from "bun:test";
import { PAGE_FRAME_SANDBOX, preparePageFrameHtml } from "./pageFrame";

describe("page frame preview", () => {
  test("sandbox does not run page scripts (opaque origin would CORS-fail fetches)", () => {
    const flags = PAGE_FRAME_SANDBOX.split(/\s+/).filter(Boolean);
    expect(flags).not.toContain("allow-scripts");
    expect(flags).not.toContain("allow-same-origin");
  });

  test("prepared html injects CSP that blocks script and connect", () => {
    const html = preparePageFrameHtml(
      `<html><head></head><body><script>fetch("https://subscriptions.cbc.ca/api/newsletter/get_active_newsletters?source=")</script></body></html>`,
      "https://www.cbc.ca/news/example",
    );
    expect(html).toMatch(/http-equiv=["']Content-Security-Policy["']/i);
    expect(html).toMatch(/script-src\s+'none'/);
    expect(html).toMatch(/connect-src\s+'none'/);
  });

  test("prepared html sets base href to the article origin", () => {
    const html = preparePageFrameHtml(`<html><head></head><body>hi</body></html>`, "https://www.cbc.ca/news/story");
    expect(html).toContain(`<base href="https://www.cbc.ca/">`);
  });
});
