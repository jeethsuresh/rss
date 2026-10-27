const base = "/v1/push";
const csrf = "X-RSS-CSRF";
export const pushPreference = "rss:device-notifications";
const cleanupKey = "rss:push-unsubscribe";
const activeKey = "rss:push-active";
async function request(path: string, method = "GET", body?: unknown) {
  const response = await fetch(base + path, {
    method,
    credentials: "same-origin",
    cache: "no-store",
    signal: AbortSignal.timeout(10000),
    headers: { [csrf]: "1", "Content-Type": "application/json" },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok)
    throw new Error(
      "Notifications could not connect to the server. Please try again online.",
    );
  return response.json();
}
async function setAccount(
  worker: ServiceWorkerRegistration,
  account: string,
  enabled: boolean,
) {
  const active = worker.active;
  if (!active)
    throw new Error("Offline support is still installing. Try again shortly.");
  await new Promise<void>((resolve, reject) => {
    const channel = new MessageChannel();
    const timeout = setTimeout(() => {
      channel.port1.close();
      reject(
        new Error(
          "Update the app from this menu before enabling notifications.",
        ),
      );
    }, 4000);
    channel.port1.onmessage = () => {
      clearTimeout(timeout);
      channel.port1.close();
      resolve();
    };
    active.postMessage({ type: "SET_PUSH_ACCOUNT", account, enabled }, [
      channel.port2,
    ]);
  });
}
export async function connectPush(create = false) {
  if (!("serviceWorker" in navigator) || !("PushManager" in window))
    throw new Error(
      "Web Push is unavailable. On iPhone and iPad, add the app to your Home Screen first.",
    );
  const worker = await navigator.serviceWorker.getRegistration();
  if (!worker?.active)
    throw new Error(
      "Offline support is still installing. Reopen the app online and try again.",
    );
  let subscription = await worker.pushManager.getSubscription();
  if (!create && !subscription) return false;
  const config = (await request("/key")) as {
    publicKey: string;
    owner: string;
  };
  if (!subscription) {
    const text = atob(config.publicKey.replace(/-/g, "+").replace(/_/g, "/"));
    subscription = await worker.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: Uint8Array.from(text, (char) => char.charCodeAt(0)),
    });
  }
  await setAccount(worker, config.owner, true);
  try {
    await request("/subscription", "POST", subscription.toJSON());
  } catch (error) {
    await setAccount(worker, "", false).catch(() => {});
    throw error;
  }
  localStorage.setItem(activeKey, "on");
  return true;
}
export async function disconnectPush() {
  try {
    localStorage.removeItem(pushPreference);
    localStorage.removeItem(activeKey);
    if (!("serviceWorker" in navigator)) return;
    const worker = await navigator.serviceWorker.getRegistration();
    if (!worker) return;
    await setAccount(worker, "", false).catch(() => {});
    const subscription = await worker.pushManager.getSubscription();
    if (subscription) {
      localStorage.setItem(cleanupKey, subscription.endpoint);
      await subscription.unsubscribe().catch(() => false);
    }
    await cleanupPush();
  } catch {
    /* The worker's disabled account gate also blocks queued notices. */
  }
}
export async function cleanupPush() {
  const endpoint = localStorage.getItem(cleanupKey);
  if (!endpoint) return;
  await request("/subscription", "DELETE", { endpoint });
  localStorage.removeItem(cleanupKey);
}
