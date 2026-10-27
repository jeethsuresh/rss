/* App-shell caching only. Account data belongs in the application's scoped storage. */
const CACHE = "rss-shell-__VERSION__";
const ASSETS = __ASSETS__;
self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE).then((cache) =>
      Promise.all(
        ASSETS.map(async (asset) => {
          const response = await fetch(
            asset === "/index.html" ? "/login" : asset,
            { cache: "reload" },
          );
          if (!response.ok) throw new Error("Shell unavailable");
          // A redirected Response cannot answer a navigation request in manual redirect mode.
          const saved =
            asset === "/index.html"
              ? new Response(await response.arrayBuffer(), {
                  status: 200,
                  headers: response.headers,
                })
              : response;
          await cache.put(asset, saved);
        }),
      ),
    ),
  );
});
self.addEventListener("message", (event) => {
  if (event.data?.type === "ACTIVATE_UPDATE") self.skipWaiting();
  if (event.data?.type === "SET_PUSH_ACCOUNT") event.waitUntil(
    caches.open("rss-push-preferences").then(cache => cache.put("/__push_preferences", new Response(JSON.stringify({ account: String(event.data.account || ""), enabled: event.data.enabled === true })))).then(() => event.ports[0]?.postMessage({ ok: true }))
  );
});
self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys
            .filter(
              (key) =>
                (key.startsWith("rss-shell-") ||
                  key.startsWith("rss-static-")) &&
                key !== CACHE,
            )
            .map((key) => caches.delete(key)),
        ),
      )
      .then(() => self.clients.claim()),
  );
});
self.addEventListener("fetch", (event) => {
  const request = event.request,
    url = new URL(request.url);
  if (
    request.method !== "GET" ||
    url.origin !== self.location.origin ||
    url.pathname.startsWith("/auth/") ||
    url.pathname.startsWith("/api/") ||
    url.pathname.startsWith("/v1/")
  )
    return;
  if (request.mode === "navigate") {
    // Never cache a login redirect, callback, or personalized HTML response.
    event.respondWith(
      fetch(request).catch(() =>
        caches.match("/index.html").then((saved) => saved || Response.error()),
      ),
    );
  } else if (ASSETS.includes(url.pathname)) {
    event.respondWith(
      caches
        .open(CACHE)
        .then(async (cache) => (await cache.match(request)) || fetch(request)),
    );
  }
});
self.addEventListener("push", event => {
  event.waitUntil((async () => {
    let data;
    try { data = event.data?.json(); } catch { return; }
    const saved = await (await caches.open("rss-push-preferences")).match("/__push_preferences");
    const preferences = saved ? await saved.json() : null;
    if (!preferences?.enabled || preferences.account !== data?.account) return;
    await self.registration.showNotification(String(data.title || "News"), {
      body: String(data.body || ""), icon: "/icons/icon-192.png", badge: "/icons/icon-192.png",
      tag: data.tag || "rss-update", data: { url: data.url || "/", account: data.account }
    });
  })());
});
self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const url = new URL(
    event.notification.data?.url || "/",
    self.location.origin,
  );
  if (url.origin !== self.location.origin) return;
  event.waitUntil(
    self.clients
      .matchAll({ type: "window", includeUncontrolled: true })
      .then(async (clients) => {
        const existing = clients.find(
          (client) => new URL(client.url).origin === url.origin,
        );
        if (existing) {
          await existing.navigate(url.href);
          return existing.focus();
        }
        return self.clients.openWindow(url.href);
      }),
  );
});
