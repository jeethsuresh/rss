import { describe, expect, test } from "bun:test";
import { browserPaneUrl, isFileLink, safeHttpUrl, shouldOpenLinkInPane } from "./linkNavigation";

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

describe("browser link actions", () => {
  const click = { defaultPrevented: false, button: 0, ctrlKey: false, metaKey: false, shiftKey: false, altKey: false };
  const link = (target = "", download = false) => ({ target, hasAttribute: (name: string) => name === "download" && download });

  test("ordinary and self-targeted links open in the pane", () => {
    expect(shouldOpenLinkInPane(click, link())).toBe(true);
    expect(shouldOpenLinkInPane(click, link("_self"))).toBe(true);
  });

  test("explicit new tabs and other browser targets are not intercepted", () => {
    for (const target of ["_blank", "_BLANK", "article-tab", "_parent", "_top"]) {
      expect(shouldOpenLinkInPane(click, link(target))).toBe(false);
    }
  });

  test("modified and non-primary clicks keep native browser behavior", () => {
    for (const key of ["ctrlKey", "metaKey", "shiftKey", "altKey"] as const) {
      expect(shouldOpenLinkInPane({ ...click, [key]: true }, link())).toBe(false);
    }
    expect(shouldOpenLinkInPane({ ...click, button: 1 }, link())).toBe(false);
    expect(shouldOpenLinkInPane({ ...click, button: 2 }, link())).toBe(false);
  });

  test("handled events and downloads are not redirected", () => {
    expect(shouldOpenLinkInPane({ ...click, defaultPrevented: true }, link())).toBe(false);
    expect(shouldOpenLinkInPane(click, link("", true))).toBe(false);
  });
});
