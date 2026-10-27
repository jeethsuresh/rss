// IndexedDB stores account data separately from the public service-worker shell.
type RecordValue = {
  key: string;
  account: string;
  method: string;
  value: any;
  saved: number;
};
type Pending = {
  key: string;
  account: string;
  method: string;
  params: { id: string };
  saved: number;
  revision: string;
};
let database: Promise<IDBDatabase> | undefined;
function open() {
  return (database ??= new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open("rss-offline", 1);
    request.onupgradeneeded = () => {
      request.result.createObjectStore("responses", { keyPath: "key" });
      request.result.createObjectStore("pending", { keyPath: "key" });
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => {
      database = undefined;
      reject(request.error);
    };
  }));
}
async function transaction<T>(
  store: string,
  mode: IDBTransactionMode,
  action: (store: IDBObjectStore) => IDBRequest<T>,
): Promise<T> {
  const db = await open();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(store, mode),
      request = action(tx.objectStore(store));
    tx.oncomplete = () => resolve(request.result);
    tx.onerror = tx.onabort = () => reject(tx.error);
  });
}
export function stableKey(value: unknown): string {
  if (value === undefined) return "null";
  if (!value || typeof value !== "object") return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(stableKey).join(",")}]`;
  return `{${Object.entries(value)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, val]) => `${JSON.stringify(key)}:${stableKey(val)}`)
    .join(",")}}`;
}
export function cacheable(method: string) {
  return /^(feeds\.(list|get|exportUrls)|articles\.(list|get|fetchLive)|readLater\.list|stories\.(list|get)|folders\.list|settings\.get|sports\..+\.(list|get)|sports\.(followed\.get|schedule\.daily)|system\.(info|ping))$/.test(
    method,
  );
}
function key(account: string, method: string, params: unknown) {
  return stableKey([account, method, params]);
}
export async function saveResponse(
  account: string,
  method: string,
  params: unknown,
  value: unknown,
) {
  await transaction("responses", "readwrite", (store) =>
    store.put({
      key: key(account, method, params),
      account,
      method,
      value,
      saved: Date.now(),
    }),
  );
}
export async function cachedResponse<T>(
  account: string,
  method: string,
  params: unknown,
): Promise<T | undefined> {
  const result = await transaction<RecordValue | undefined>(
    "responses",
    "readonly",
    (store) => store.get(key(account, method, params)),
  );
  return result?.value as T | undefined;
}
export async function pendingChanges(account: string) {
  return (
    await transaction<Pending[]>("pending", "readonly", (store) =>
      store.getAll(),
    )
  )
    .filter((item) => item.account === account)
    .sort((a, b) => a.saved - b.saved);
}
export async function removePending(item: Pending) {
  const db = await open();
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction("pending", "readwrite"),
      store = tx.objectStore("pending");
    const request = store.get(item.key);
    request.onsuccess = () => {
      if (request.result?.revision === item.revision) store.delete(item.key);
    };
    tx.oncomplete = () => resolve();
    tx.onerror = tx.onabort = () => reject(tx.error);
  });
}
export async function queueReadState(
  account: string,
  method: string,
  params: { id: string },
) {
  const db = await open();
  const updated = await new Promise<any>((resolve, reject) => {
    // Read and patch within the same transaction so simultaneous offline edits
    // cannot overwrite each other's changes to a cached article list.
    const tx = db.transaction(["responses", "pending"], "readwrite");
    const store = tx.objectStore("responses"),
      request = store.getAll();
    let updated: any;
    request.onsuccess = () => {
      const records = (request.result as RecordValue[]).filter(
        (item) => item.account === account,
      );
      const canonical =
        records.find(
          (item) => item.key === key(account, "articles.get", params),
        ) ??
        records.find(
          (item) => item.key === key(account, "articles.fetchLive", params),
        );
      if (canonical)
        updated = {
          ...canonical.value,
          isRead: method === "articles.markRead",
        };
      function patch(value: any): any {
        if (Array.isArray(value)) return value.map(patch);
        if (!value || typeof value !== "object") return value;
        const result = Object.fromEntries(
          Object.entries(value).map(([k, v]) => [k, patch(v)]),
        );
        if (value.id === params.id && "isRead" in value) {
          result.isRead = method === "articles.markRead";
          updated ??= result;
        }
        return result;
      }
      records.forEach((item) =>
        store.put({ ...item, value: patch(item.value) }),
      );
      if (!updated) {
        tx.abort();
        return;
      }
      tx.objectStore("pending").put({
        key: key(account, "article-read-state", params),
        account,
        method,
        params,
        saved: Date.now(),
        revision: crypto.randomUUID(),
      });
    };
    tx.oncomplete = () => resolve(updated);
    tx.onerror = tx.onabort = () =>
      reject(
        tx.error ||
          new Error("Open this article online before changing it offline."),
      );
  });
  window.dispatchEvent(new Event("rss:offline-change"));
  return updated;
}

export async function clearOffline(account: string) {
  for (const name of ["responses", "pending"]) {
    const records = await transaction<(RecordValue | Pending)[]>(
      name,
      "readonly",
      (store) => store.getAll(),
    );
    for (const item of records.filter((item) => item.account === account))
      await transaction(name, "readwrite", (store) => store.delete(item.key));
  }
}
