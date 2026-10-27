// Run after build:renderer. This fixture server tests the real production frontend
// without a network feed, credentials, or a production database.
import { chromium } from "playwright";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { resolve, extname } from "node:path";
import assert from "node:assert/strict";
const root = resolve("apps/desktop/dist-renderer");
const output = resolve(process.env.PWA_SCREENSHOTS || ".cache/pwa");
await (await import("node:fs/promises")).mkdir(output, { recursive: true });
const settings = {
  defaultPollIntervalSeconds: 3600,
  theme: "light",
  articleDensity: "comfortable",
  defaultSort: "newest",
  markReadOnOpen: false,
  notificationsEnabled: false,
  aiEnabled: false,
  aiBaseUrl: "",
  aiModel: "",
  readLaterChrome: "tabs",
};
const articles = Object.fromEntries(
  ["alice", "bob"].map((user) => [
    user,
    {
      id: `${user}-article`,
      feedId: "test-feed",
      title: `${user}'s saved article`,
      url: "https://example.com/article",
      author: "Fixture author",
      content: "<p>Readable offline content for this account.</p>",
      summary: "An article for offline testing.",
      rssContent: "<p>Readable offline content for this account.</p>",
      crawledContent: "<p>Readable offline content for this account.</p>",
      readerContent: "<p>Readable offline content for this account.</p>",
      liveContent: "<p>Readable offline content for this account.</p>",
      crawlStatus: "ok",
      extractStatus: "ok",
      crawlError: "",
      crawlUnreliable: false,
      publishedAt: "2026-10-26T12:00:00Z",
      updatedAt: null,
      externalId: user,
      isRead: false,
      isStarred: false,
      isReadLater: false,
      priority: "none",
      discoveredAt: "2026-10-26T12:00:00Z",
      feedTitle: "Offline test feed",
    },
  ]),
);
let rejectSession = false;
const server = createServer(async (request, response) => {
  const path = new URL(request.url, "http://localhost").pathname;
  const account = request.headers.cookie?.match(/account=(alice|bob)/)?.[1];
  const json = (value, status = 200) => {
    response.writeHead(status, {
      "Content-Type": "application/json",
      "Cache-Control": "no-store",
    });
    response.end(JSON.stringify(value));
  };
  if (path === "/v1/web/config")
    return json({
      federated: false,
      registrationEnabled: false,
      version: "test",
    });
  if (path === "/v1/web/session")
    return account && !rejectSession
      ? json({ user: { id: account, username: account } })
      : json({}, 401);
  if (path === "/v1/web/logout") {
    response.setHeader("Set-Cookie", "account=; Max-Age=0; Path=/");
    return json({});
  }
  if (path === "/v1/events") {
    response.writeHead(200, { "Content-Type": "text/event-stream" });
    response.write(": ready\n\n");
    return;
  }
  if (path === "/v1/rpc") {
    if (!account || rejectSession) return json({}, 401);
    let body = "";
    for await (const chunk of request) body += chunk;
    const { method, params = {} } = JSON.parse(body),
      article = articles[account];
    const feed = {
      id: "test-feed",
      title: "Offline test feed",
      url: "https://example.com/feed.xml",
      unreadCount: article.isRead ? 0 : 1,
      enabled: true,
      isReadLater: false,
    };
    if (method === "system.ping") return json({ ok: true });
    if (method === "system.info") return json({ version: "test" });
    if (method === "settings.get") return json(settings);
    if (method === "feeds.list") return json([feed]);
    if (method === "articles.list")
      return json({ articles: [article], nextCursor: null });
    if (method === "articles.get" || method === "articles.fetchLive")
      return json(article);
    if (method === "articles.markRead" || method === "articles.markUnread") {
      article.isRead = method.endsWith("markRead");
      return json(article);
    }
    if (method === "readLater.list")
      return json([{ ...article, isReadLater: true }]);
    if (method === "stories.get")
      return json({
        id: "story",
        title: "Offline story",
        articles: [article],
        memberCount: 1,
        isRead: false,
      });
    if (
      method === "folders.list" ||
      method === "stories.list" ||
      method.endsWith(".list")
    )
      return json([]);
    if (method === "sports.schedule.daily") return json([]);
    if (method.endsWith(".get")) return json(null);
    return json({});
  }
  const file = path === "/" || path === "/login" ? "/index.html" : path;
  try {
    const target = resolve(root, "." + file);
    assert(target.startsWith(root + "/"));
    const content = await readFile(target);
    response.writeHead(200, {
      "Content-Type":
        {
          ".html": "text/html",
          ".js": "application/javascript",
          ".css": "text/css",
          ".png": "image/png",
          ".webmanifest": "application/manifest+json",
        }[extname(target)] || "application/octet-stream",
      "Cache-Control": "no-cache",
    });
    response.end(content);
  } catch {
    response.writeHead(404);
    response.end();
  }
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const baseURL = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch({
  headless: true,
  ...(process.env.PWA_BROWSER_PATH
    ? { executablePath: process.env.PWA_BROWSER_PATH }
    : {}),
});
const context = await browser.newContext({ hasTouch: true });
await context.addCookies([{ name: "account", value: "alice", url: baseURL }]);
const page = await context.newPage();
page.setDefaultTimeout(15000);
const errors = [];
page.on("pageerror", (error) => errors.push(error.message));
async function readyWorker() {
  await page.evaluate(async () => {
    await navigator.serviceWorker.ready;
    if (!navigator.serviceWorker.controller)
      await new Promise((resolve) =>
        navigator.serviceWorker.addEventListener("controllerchange", resolve, {
          once: true,
        }),
      );
  });
}
try {
  for (const [width, height] of [
    [320, 900],
    [393, 852],
    [540, 960],
    [768, 1024],
    [1024, 768],
    [1280, 400],
    [1920, 1080],
  ]) {
    await page.setViewportSize({ width, height });
    await page.goto(baseURL);
    await page.locator(".article-row").first().waitFor();
    await page.locator(".article-row").first().click();
    await page.locator(".reader-pane .reader h1").waitFor();
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      true,
    );
    const reader = await page.locator(".reader-pane").boundingBox();
    assert(
      reader.width > 180 && reader.height > 80,
      `Usable reader at ${width}x${height}`,
    );
    await page.screenshot({ path: `${output}/rss-${width}x${height}.png` });
    if (width <= 1100) {
      await page.getByRole("button", { name: "Feeds", exact: true }).click();
      await page.getByRole("button", { name: /Offline test feed/ }).waitFor();
      await page
        .getByRole("button", { name: "Article list", exact: true })
        .click();
    }
    await page.getByRole("tab", { name: /Read Later/ }).click();
    await page.locator(".article-row").first().click();
    await page.locator(".reader-pane .reader-body").waitFor();
    if (width <= 1100) {
      await page.getByRole("button", { name: "Filters", exact: true }).click();
      await page
        .getByRole("button", { name: "Archived", exact: true })
        .waitFor();
      await page
        .getByRole("button", { name: "Article list", exact: true })
        .click();
    }
    await page.getByRole("tab", { name: "Sports", exact: true }).click();
    await page.locator(".sports-layout").waitFor();
    if (width <= 1100) {
      await page
        .getByRole("button", { name: "Leagues & teams", exact: true })
        .click();
      await page.locator(".sports-layout > .sidebar").waitFor();
      await page
        .getByRole("button", { name: "Dashboard", exact: true })
        .click();
    }
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      true,
    );
    await page.getByRole("button", { name: "Settings", exact: true }).click();
    await page.getByRole("button", { name: "General", exact: true }).waitFor();
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      true,
    );
    console.log(
      `PASS RSS, Read Later, Sports, Settings layout ${width}x${height}`,
    );
  }
  await page.setViewportSize({ width: 393, height: 852 });
  await page.goto(baseURL);
  await page.locator(".article-row").first().click();
  await page.locator(".reader-pane .reader h1").waitFor();
  await readyWorker();
  await context.setOffline(true);
  await page.reload();
  await page.locator(".article-row").first().click();
  await page.locator(".reader-pane .reader h1").waitFor();
  assert.equal(
    await page.locator(".reader-pane .reader h1").textContent(),
    "alice's saved article",
  );
  await page
    .locator(".reader-actions")
    .getByRole("button", { name: "Mark read", exact: true })
    .click();
  await page.waitForFunction(async () => {
    const db = await new Promise((resolve, reject) => {
      const request = indexedDB.open("rss-offline");
      request.onsuccess = () => resolve(request.result);
      request.onerror = reject;
    });
    return new Promise((resolve) => {
      const request = db.transaction("pending").objectStore("pending").count();
      request.onsuccess = () => {
        db.close();
        resolve(request.result === 1);
      };
    });
  });
  await page.reload();
  await page.locator(".article-row").first().click();
  await page
    .locator(".reader-actions")
    .getByRole("button", { name: "Mark unread", exact: true })
    .waitFor();
  console.log("PASS offline reading, durable read-state outbox, second reload");
  await context.setOffline(false);
  await page.waitForFunction(async () => {
    const response = await fetch("/v1/rpc", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        method: "articles.get",
        params: { id: "alice-article" },
      }),
    });
    return (await response.json()).isRead;
  });
  console.log("PASS reconnect replays read state");
  rejectSession = true;
  await page.reload();
  await page.getByRole("heading", { name: "Sign in", exact: true }).waitFor();
  assert.equal(
    await page.evaluate(() => localStorage.getItem("rss:offline-session")),
    null,
  );
  rejectSession = false;
  await context.addCookies([{ name: "account", value: "bob", url: baseURL }]);
  await page.goto(baseURL);
  await page.locator(".article-row").first().waitFor();
  assert(
    (await page.locator(".article-row").first().textContent()).includes(
      "bob's saved article",
    ),
  );
  assert(
    !(await page.locator(".article-list").textContent()).includes(
      "alice's saved article",
    ),
  );
  console.log(
    "PASS revoked session and account switch do not reveal previous account",
  );
  assert.deepEqual(errors, []);
} finally {
  await browser.close();
  server.closeAllConnections();
  await new Promise((resolve) => server.close(resolve));
}
