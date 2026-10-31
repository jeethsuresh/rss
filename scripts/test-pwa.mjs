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
let rejectLogout = false;
let federated = false;
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
      federated,
      registrationEnabled: false,
      version: "test",
    });
  if (path === "/v1/web/session")
    return account && !rejectSession
      ? json({ user: { id: account, username: account } })
      : json({}, 401);
  if (path === "/v1/web/logout") {
    if (rejectLogout) return json({ error: { message: "Unavailable" } }, 503);
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
const mainNavigation = page.getByRole("navigation", { name: "Main navigation", exact: true });
async function navigateTo(label) {
  if (await mainNavigation.isVisible()) {
    await mainNavigation.getByRole("button", { name: label, exact: true }).click();
  } else if (label === "Settings") {
    await page.getByRole("button", { name: "Settings", exact: true }).click();
  } else {
    await page.getByRole("tab", { name: new RegExp(label === "RSS" ? "RSS Reader" : label) }).click();
  }
}
async function checkBottomNavigation(active, height) {
  await page.waitForFunction(() => {
    const bar = document.querySelector('.mobile-bottom-navigation')?.getBoundingClientRect();
    return bar && Math.abs(bar.bottom - innerHeight) <= 1;
  });
  const bar = await mainNavigation.boundingBox();
  assert(bar && Math.abs(bar.y + bar.height - height) <= 1, "Bottom navigation stays at the viewport edge");
  assert.equal(await mainNavigation.locator('[aria-current="page"]').textContent(), active);
  for (const button of await mainNavigation.getByRole("button").all()) {
    const box = await button.boundingBox();
    assert(box.width >= 44 && box.height >= 44, "Navigation has touch-sized targets");
  }
  const content = await page.locator('.app > .layout, .app > .settings-page').boundingBox();
  assert(content && content.y + content.height <= bar.y + 1, "Content fits above the bottom bar");
}
try {
  for (const [width, height] of [
    [320, 568],
    [320, 900],
    [393, 852],
    [540, 960],
    [852, 393],
    [768, 1024],
    [1024, 768],
    [1280, 400],
    [1920, 1080],
  ]) {
    await page.setViewportSize({ width, height });
    await page.goto(baseURL);
    await page.locator(".article-row").first().waitFor();
    assert.equal(await page.locator(".web-session-bar").count(), 0);
    const mobile = width <= 680 || (width <= 1100 && height <= 500);
    assert.equal(await mainNavigation.isVisible(), mobile);
    if (mobile) {
      assert.deepEqual(await mainNavigation.getByRole("button").allTextContents(), ["RSS", "Read Later", "Sports", "Settings"]);
      assert.equal(await page.getByRole("tablist", { name: "App mode" }).isVisible(), false);
      await checkBottomNavigation("RSS", height);
      await page.screenshot({ path: `${output}/rss-list-${width}x${height}.png` });
    }
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
    if (mobile) {
      await checkBottomNavigation("RSS", height);
      await page.getByRole("button", { name: "Article list", exact: true }).click();
      await page.locator(".article-row").first().waitFor();
    }
    if (width <= 1100) {
      await page.getByRole("button", { name: "Feeds", exact: true }).click();
      await page.getByRole("button", { name: /Offline test feed/ }).waitFor();
      if (mobile) {
        await checkBottomNavigation("RSS", height);
        await page.screenshot({ path: `${output}/rss-sidebar-${width}x${height}.png` });
        await page.getByRole("button", { name: /Offline test feed/ }).click();
        await page.locator(".article-row").first().waitFor();
        assert.equal(await page.locator(".rss-sidebar").isVisible(), false);
      }
    }
    // Account controls share the sidebar footer with Add feed, including on phones.
    if (width <= 1100 && mobile) {
      await page.getByRole("button", { name: "Feeds", exact: true }).click();
    }
    const footer = page.locator(".rss-sidebar-footer");
    assert.equal(await footer.locator(".sidebar-account-name").textContent(), "alice");
    const addFeed = await footer.getByRole("button", { name: "Add feed", exact: true }).boundingBox();
    const account = await footer.locator(".sidebar-account-row").boundingBox();
    assert(account.y >= addFeed.y + addFeed.height && account.y + account.height <= height,
      `Account row fits below Add feed at ${width}x${height}`);
    for (const button of await footer.locator(".sidebar-account-action").all()) {
      const box = await button.boundingBox();
      assert(box.width >= 44 && box.height >= 44, "Account icons have touch-sized targets");
      assert.equal((await button.textContent()).trim(), "", "Account actions are icon-only");
    }
    await footer.getByRole("button", { name: "App permissions and installation", exact: true }).focus();
    await page.keyboard.press("Enter");
    await page.getByRole("heading", { name: "Permissions", exact: true }).waitFor();
    await page.getByRole("heading", { name: "App", exact: true }).waitFor();
    assert.equal(await page.getByRole("heading", { name: "iPhone and iPad · Safari", exact: true }).count(), 1);
    assert.equal(await page.getByRole("heading", { name: "Android · Chrome", exact: true }).count(), 1);
    assert((await page.locator(".pwa-panel").textContent()).includes("Open as Web App"));
    await page.screenshot({ path: `${output}/rss-permissions-${width}x${height}.png` });
    if (mobile) await checkBottomNavigation("Settings", height);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    await page.getByRole("button", { name: "← Back", exact: true }).click();
    if (width <= 1100) {
      await page.getByRole("button", { name: "Article list", exact: true }).click();
    }
    await navigateTo("Read Later");
    await page.locator(".article-row").first().click();
    await page.locator(".reader-pane .reader-body").waitFor();
    if (mobile) {
      await checkBottomNavigation("Read Later", height);
      await page.getByRole("button", { name: "Article list", exact: true }).click();
    }
    if (width <= 1100) {
      await page.getByRole("button", { name: "Filters", exact: true }).click();
      await page
        .getByRole("button", { name: "Archived", exact: true })
        .waitFor();
      await page
        .getByRole("button", { name: "Article list", exact: true })
        .click();
    }
    await navigateTo("Sports");
    await page.locator(".sports-layout").waitFor();
    if (mobile) await checkBottomNavigation("Sports", height);
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
    await navigateTo("Settings");
    await page.getByRole("button", { name: "General", exact: true }).waitFor();
    await page.getByRole("button", { name: "Permissions", exact: true }).click();
    await page.getByRole("heading", { name: "App", exact: true }).waitFor();
    if (mobile) {
      await checkBottomNavigation("Settings", height);
      await page.locator(".settings-body").evaluate(element => element.scrollTop = element.scrollHeight);
      await checkBottomNavigation("Settings", height);
      await page.screenshot({ path: `${output}/rss-settings-${width}x${height}.png` });
      await navigateTo("RSS");
      await page.locator(".article-row").first().waitFor();
      await checkBottomNavigation("RSS", height);
    }
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
  await checkBottomNavigation("RSS", 852);
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
  // Device state stays mounted when Permissions is closed.
  await page.evaluate(() => window.dispatchEvent(new Event("rss:storage-unavailable")));
  await navigateTo("Settings");
  await page.getByRole("button", { name: "Permissions", exact: true }).click();
  await page.getByText("Device storage is unavailable or full. New data may not be available offline.", { exact: true }).waitFor();
  await page.evaluate(() => {
    Notification.requestPermission = async () => "denied";
    navigator.storage.persist = async () => false;
  });
  await page.getByRole("button", { name: "Enable notifications", exact: true }).click();
  await page.getByText("Notifications are off. You can change permission in your browser settings.", { exact: true }).waitFor();
  await page.getByRole("button", { name: "Keep data on this device", exact: true }).click();
  await page.getByText("This browser manages storage. Keep a separate backup of important data.", { exact: true }).waitFor();
  for (const failed of [false, true]) {
    await page.evaluate(failed => {
      const event = new Event("beforeinstallprompt", { cancelable: true });
      event.prompt = async () => {
        if (failed) throw new Error("Install unavailable");
        window.installPromptUsed = true;
      };
      event.userChoice = Promise.resolve({ outcome: "dismissed" });
      window.dispatchEvent(event);
    }, failed);
    await page.getByRole("button", { name: "Install app", exact: true }).click();
    await page.getByRole("button", { name: "Install app", exact: true }).waitFor({ state: "hidden" });
    if (failed) {
      await page.getByText("Installation could not be started. Use the steps below to add News to your Home Screen.", { exact: true }).waitFor();
    } else {
      assert.equal(await page.evaluate(() => window.installPromptUsed), true);
    }
  }
  await page.getByRole("button", { name: "← Back", exact: true }).click();
  await page.getByRole("button", { name: "Feeds", exact: true }).click();
  rejectLogout = true;
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await page.getByText("Connection unavailable. Saved content is available on this device.", { exact: true }).waitFor();
  await page.getByRole("button", { name: "Sign out", exact: true }).waitFor();
  assert.equal(await page.getByRole("button", { name: "Sign out", exact: true }).isEnabled(), true);
  rejectLogout = false;
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await page.getByRole("heading", { name: "Sign in", exact: true }).waitFor();
  federated = true;
  await context.addCookies([{ name: "account", value: "alice", url: baseURL }]);
  await page.goto(baseURL);
  await page.locator(".article-row").first().waitFor();
  await page.getByRole("button", { name: "Feeds", exact: true }).click();
  await page.getByRole("button", { name: "Sign out of News", exact: true }).click();
  await page.getByRole("heading", { name: "Welcome to News", exact: true }).waitFor();
  assert.equal(new URL(page.url()).pathname + new URL(page.url()).search, "/login?auto=off");
  console.log("PASS persistent device controls, denied permissions, install prompts, failed sign out and local/federated icon sign out");
  assert.deepEqual(errors, []);
} finally {
  await browser.close();
  server.closeAllConnections();
  await new Promise((resolve) => server.close(resolve));
}
