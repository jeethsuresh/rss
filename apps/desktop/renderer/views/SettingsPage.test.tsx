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

function render(backend: Partial<ReaderBackend>, initialSection: "server" | "general" | "ai" | "permissions") {
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

  test("keeps desktop notification controls in Permissions", () => {
    const backend = { sync: {} as NonNullable<ReaderBackend["sync"]> };
    expect(render(backend, "permissions")).toContain("Desktop notifications");
    expect(render(backend, "general")).not.toContain("Desktop notifications");
    expect(render({}, "general")).not.toContain("account bar");
  });

  test("lets the desktop edit local AI settings", () => {
    const html = render({ sync: {} as NonNullable<ReaderBackend["sync"]> }, "ai");
    expect(html).toContain("Local AI (LM Studio)");
    expect(html).toContain("Save connection");
    expect(html).toContain("Enable AI triage");
  });

  test("locks AI settings in the hosted web app", () => {
    const html = render({}, "ai");
    expect(html).toContain("Server AI");
    expect(html).toContain("cannot be changed from the browser");
    expect(html).toContain("disabled");
    expect(html).not.toContain("Save connection");
    expect(html).not.toContain("Local AI (LM Studio)");
  });
});
