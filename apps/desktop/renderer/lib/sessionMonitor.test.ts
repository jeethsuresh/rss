import { expect, test } from "bun:test";
import { createSessionMonitor } from "./sessionMonitor";
function fixture(fetcher: typeof fetch) {
  const state = { user: { id: "a" } as { id: string } | null, hint: true, unavailable: false };
  const monitor = createSessionMonitor<{ id: string }>({ sessionURL: "/session", configURL: "/config", logoutURL: "/logout", csrfHeader: "X-Test-CSRF", isAuthenticated: () => !!state.user, onSession: user => { state.user = user; }, onRevoked: () => { state.user = null; }, onAutoHint: hint => { state.hint = hint; }, onUnavailable: unavailable => { state.unavailable = unavailable; }, fetcher });
  return { state, monitor };
}
test("temporary failure denies refresh without clearing the current user or starting login", async () => {
  const paths: string[] = [];
  let failed = true;
  const { state, monitor } = fixture((async (url: RequestInfo | URL, init?: RequestInit) => {
    paths.push(String(url)); expect(init?.credentials).toBe("same-origin"); expect(init?.cache).toBe("no-store");
    return failed ? new Response("", { status: 503 }) : Response.json({ user: { id: "a" } });
  }) as unknown as typeof fetch);
  await monitor.check(); expect(state.user?.id).toBe("a"); expect(state.unavailable).toBe(true); expect(paths).toEqual(["/session"]);
  failed = false; await monitor.check(); expect(state.unavailable).toBe(false); expect(state.user?.id).toBe("a"); monitor.dispose();
});
test("revocation immediately clears user and stale hint before completing local logout and fresh hint", async () => {
  let release!: (r: Response) => void;
  const logout = new Promise<Response>(resolve => { release = resolve; });
  const paths: string[] = [];
  const { state, monitor } = fixture((async (url: RequestInfo | URL, init?: RequestInit) => {
    paths.push(String(url));
    if (url === "/session") return new Response("", { status: 401 });
    if (url === "/logout") { expect(init?.method).toBe("POST"); expect(new Headers(init?.headers).get("X-Test-CSRF")).toBe("1"); return logout; }
    return Response.json({ autoSignIn: false });
  }) as unknown as typeof fetch);
  const pending = monitor.check(); await new Promise(resolve => setTimeout(resolve, 0));
  expect(state.user).toBeNull(); expect(state.hint).toBe(false); expect(paths).toEqual(["/session", "/logout"]);
  release(Response.json({ ok: true })); await pending;
  expect(paths).toEqual(["/session", "/logout", "/config"]); expect(state.hint).toBe(false); monitor.dispose();
});
test("late successful request cannot resurrect a locally signed out or revoked UI", async () => {
  let release!: (r: Response) => void;
  const { state, monitor } = fixture((() => new Promise<Response>(resolve => { release = resolve; })) as unknown as typeof fetch);
  const pending = monitor.check(); monitor.localSignedOut(); release(Response.json({ user: { id: "a" } })); await pending;
  expect(state.user).toBeNull(); expect(state.hint).toBe(false); monitor.dispose();
});
test("an anonymous legacy session can refresh the shared hint without authenticating the marker", async () => {
  const { state, monitor } = fixture((async (url: RequestInfo | URL) => url === "/session" ? new Response("", { status: 401 }) : Response.json({ autoSignIn: true })) as unknown as typeof fetch);
  state.user = null; state.hint = false; await monitor.check(); expect(state.user).toBeNull(); expect(state.hint).toBe(true); monitor.dispose();
});
