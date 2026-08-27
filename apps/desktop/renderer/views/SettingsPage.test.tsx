import { describe, expect, test } from "bun:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import type { ReaderBackend, Settings } from "@rss-reader/shared";
import { SettingsPage } from "./SettingsPage";

const settings: Settings = {
  defaultPollIntervalSeconds: 3600,
  theme: "system",
  articleDensity: "comfortable",
  defaultSort: "newest",
  markReadOnOpen: true,
  notificationsEnabled: false,
  aiEnabled: false,
  aiBaseUrl: "",
  aiModel: "",
};

function render(backend: Partial<ReaderBackend>, initialSection: "server" | "general") {
  return renderToStaticMarkup(createElement(SettingsPage, {
    backend: backend as ReaderBackend,
    settings,
    onSettings: () => undefined,
    onClose: () => undefined,
    onOpenArticle: async () => undefined,
    applyTheme: () => undefined,
    initialSection,
  }));
}

describe("desktop server settings", () => {
  test("shows server login controls when the sync bridge is available", () => {
    const html = render({ sync: {} as NonNullable<ReaderBackend["sync"]> }, "server");
    expect(html).toContain("Server sync");
    expect(html).toContain("Server URL");
    expect(html).toContain("Username");
    expect(html).toContain("Password");
    expect(html).toContain("Sign in");
    expect(html).toContain("Create account");
  });

  test("does not offer a second server login inside the hosted web app", () => {
    const html = render({}, "general");
    expect(html).not.toContain("Server sync");
  });
});
