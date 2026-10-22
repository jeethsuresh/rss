type Options<User> = {
  sessionURL: string; configURL: string; logoutURL: string; csrfHeader: string;
  isAuthenticated: () => boolean;
  onSession: (user: User) => void;
  onRevoked: () => void;
  onAutoHint: (available: boolean) => void;
  onUnavailable: (unavailable: boolean) => void;
  fetcher?: typeof fetch;
};

// A confirmed401 invalidates in-flight older results. A temporary failure never
// clears the user, app cookie or local drafts, and cannot start a new login.
export function createSessionMonitor<User>(options: Options<User>) {
  const fetcher = options.fetcher ?? fetch;
  let generation = 0, disposed = false;
  let checking: Promise<void> | null = null, invalidating: Promise<void> | null = null;
  const request = async (url: string, init?: RequestInit) => {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 10_000);
    try { return await fetcher(url, { credentials: "same-origin", cache: "no-store", ...init, signal: controller.signal }); }
    finally { clearTimeout(timeout); }
  };
  const localSignedOut = () => {
    generation++;
    options.onAutoHint(false);
    options.onRevoked();
  };
  const refreshHint = async (epoch: number) => {
    const response = await request(options.configURL);
    if (!response.ok) throw new Error("Sign-in unavailable");
    const config = await response.json() as { autoSignIn?: boolean };
    if (!disposed && generation === epoch) options.onAutoHint(config.autoSignIn === true);
  };
  const unauthorized = (): Promise<void> => {
    if (disposed) return Promise.resolve();
    if (invalidating) return invalidating;
    localSignedOut();
    const epoch = generation;
    invalidating = (async () => {
      // Local logout clears only this app's rejected cookie. Complete it before
      // enabling a fresh handshake, so its response cannot erase the new cookie.
      try { await request(options.logoutURL, { method: "POST", headers: { [options.csrfHeader]: "1" } }); } catch { /* already confirmed signed out */ }
      try { await refreshHint(epoch); if (!disposed && generation === epoch) options.onUnavailable(false); }
      catch { if (!disposed && generation === epoch) options.onUnavailable(true); }
    })().finally(() => { invalidating = null; });
    return invalidating;
  };
  const check = (): Promise<void> => {
    if (disposed) return Promise.resolve();
    if (invalidating) return invalidating;
    if (checking) return checking;
    const epoch = generation;
    checking = (async () => {
      try {
        const response = await request(options.sessionURL);
        if (disposed || generation !== epoch) return;
        if (response.status === 401) {
          if (options.isAuthenticated()) await unauthorized();
          else await refreshHint(epoch);
          return;
        }
        if (!response.ok) throw new Error("Sign-in unavailable");
        const body = await response.json() as { user?: User };
        if (!body.user) throw new Error("Invalid session response");
        if (!disposed && generation === epoch) { options.onSession(body.user); options.onUnavailable(false); }
      } catch { if (!disposed && generation === epoch) options.onUnavailable(true); }
    })().finally(() => { checking = null; });
    return checking;
  };
  return { check, unauthorized, localSignedOut, dispose: () => { disposed = true; generation++; } };
}
