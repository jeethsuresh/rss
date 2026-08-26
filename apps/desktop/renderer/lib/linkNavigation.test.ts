import { describe, expect, test } from "bun:test";
import { browserPaneUrl } from "./linkNavigation";

describe("browser pane link navigation", () => {
  test("does not capture app-owned hash routes", () => {
    expect(
      browserPaneUrl("#/sports/mlb/teams/111?season=2026", "https://example.com/article"),
    ).toBeNull();
  });

  test("does not capture document fragment links", () => {
    expect(browserPaneUrl("#box-score", "https://example.com/article")).toBeNull();
  });

  test("resolves article-relative web links", () => {
    expect(browserPaneUrl("/next", "https://example.com/article")).toBe(
      "https://example.com/next",
    );
  });

  test("rejects non-web schemes", () => {
    expect(browserPaneUrl("javascript:alert(1)", "https://example.com/article")).toBeNull();
  });
});
