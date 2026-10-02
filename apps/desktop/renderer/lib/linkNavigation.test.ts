import { describe, expect, test } from "bun:test";
import { browserPaneUrl, isFileLink, safeHttpUrl } from "./linkNavigation";

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

  test("does not treat app hash routes as local files", () => {
    expect(isFileLink("https://rss.jeeth.dev/#/sports/mlb/teams/141")).toBe(false);
    expect(isFileLink("file:///")).toBe(true);
    expect(isFileLink("file:///#/sports/mlb/teams/141?season=2026")).toBe(true);
  });

  test("accepts only absolute http(s) image URLs", () => {
    expect(safeHttpUrl("https://www.mlbstatic.com/team-logos/141.svg")).toBe(
      "https://www.mlbstatic.com/team-logos/141.svg",
    );
    expect(safeHttpUrl("file:///")).toBeNull();
    expect(safeHttpUrl("/team-logos/141.svg")).toBeNull();
  });
});
