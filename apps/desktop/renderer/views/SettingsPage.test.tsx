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

function render(backend: Partial<ReaderBackend>, initialSection: "server" | "general", session?: { username: string; onLogout: () => Promise<void> }) {
  return renderToStaticMarkup(createElement(SettingsPage, {
    backend: backend as ReaderBackend,
    session,
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

  test("hosts the web account and logout in Login settings without a second sign-in form", () => {
    const html = render({}, "server", { username: "reader", onLogout: async () => undefined });
    expect(html).toContain("Login");
    expect(html).toContain("Signed in as <strong>reader</strong>");
    expect(html).toContain("Log out");
    expect(html).not.toContain("Password");
    expect(html).not.toContain("Server sync");
  });

  test("does not offer a second server login inside the hosted web app", () => {
    const html = render({}, "general");
    expect(html).not.toContain("Server sync");
  });
});
