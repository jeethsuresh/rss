const key = "rss:offline-session";
export function readOfflineSession<T>(): T | null {
  try {
    return JSON.parse(localStorage.getItem(key) || "null") as T | null;
  } catch {
    return null;
  }
}
export function rememberSession<T>(value: T) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {}
}
export function forgetSession() {
  try {
    localStorage.removeItem(key);
  } catch {}
}
export function networkFailure(error: unknown) {
  return (
    error instanceof TypeError ||
    (error instanceof DOMException &&
      ["AbortError", "TimeoutError"].includes(error.name))
  );
}
