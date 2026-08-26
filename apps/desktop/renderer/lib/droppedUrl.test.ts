import { describe, expect, test } from "bun:test";
import { normalizeDroppedUrl } from "./droppedUrl";

describe("normalizeDroppedUrl", () => {
  test("accepts https urls", () => {
    const r = normalizeDroppedUrl("https://example.com/path?q=1");
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.url).toBe("https://example.com/path?q=1");
  });

  test("prefixes scheme-less hosts", () => {
    const r = normalizeDroppedUrl("example.com/x");
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.url).toBe("https://example.com/x");
  });

  test("rejects plain words", () => {
    const r = normalizeDroppedUrl("not a url at all");
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.attempted).toBe("not a url at all");
  });

  test("rejects non-http schemes", () => {
    const r = normalizeDroppedUrl("ftp://example.com/file");
    expect(r.ok).toBe(false);
  });

  test("rejects http urls without a host", () => {
    const r = normalizeDroppedUrl("https:///missing-host");
    expect(r.ok).toBe(false);
  });

  test("rejects malformed urls", () => {
    const r = normalizeDroppedUrl("https://example.com/%zz");
    expect(r.ok).toBe(false);
  });

  test("uses first uri-list line", () => {
    const r = normalizeDroppedUrl("# comment\nhttps://a.example/\nhttps://b.example/");
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.url).toBe("https://a.example/");
  });
});
