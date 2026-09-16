import { describe, expect, test } from "bun:test";
import { shouldReloadArticleListInBackground } from "./listRefresh";

describe("article list background refresh", () => {
  test("keeps the unread view as a stable reading snapshot", () => {
    expect(shouldReloadArticleListInBackground("unread")).toBe(false);
  });

  test("allows the all-items view to receive background additions", () => {
    expect(shouldReloadArticleListInBackground("all")).toBe(true);
  });
});
