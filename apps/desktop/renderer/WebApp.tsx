import { disconnectPush } from "./lib/push";
import { DeviceFeatures } from "./lib/pwa";
import { clearOffline } from "./lib/offline";
import { readOfflineSession, rememberSession, forgetSession, networkFailure } from "./lib/offlineSession";
import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { App } from "./App";
import { createWebBackend, installWebDesktopShim } from "./lib/webBackend";
import { appReturnPath, automaticSignInURL } from "./lib/automaticSignIn";
import { createSessionMonitor } from "./lib/sessionMonitor";

type User = { id: string; username: string; displayName?: string };
type Config = { registrationEnabled: boolean; federated?: boolean; defaultDomain?: string; autoSignIn?: boolean; version: string };

if (!window.rss) installWebDesktopShim();

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, { credentials: "same-origin", ...init, signal: AbortSignal.timeout(10_000) });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body?.error?.message || "Request failed");
  return body as T;
}

async function loadSession(): Promise<{ user: User } | null> {
  const response = await fetch("/v1/web/session", { credentials: "same-origin", cache: "no-store", signal: AbortSignal.timeout(10_000) });
  if (response.status === 401) return null;
  if (!response.ok) throw new Error("Unable to check your session");
  return response.json();
}

export function WebApp() {
  const [user, setUser] = useState<User | null>(null);
  const backend = useMemo(() => createWebBackend(user?.id), [user?.id]);
  const [config, setConfig] = useState<Config | null>(null);
  const [checking, setChecking] = useState(true);
  const [startupError, setStartupError] = useState(false);
  const automaticAttempted = useRef(false);
  const userRef = useRef<User | null>(null);
  const monitorRef = useRef<ReturnType<typeof createSessionMonitor<User>> | null>(null);
  const [automaticAllowed, setAutomaticAllowed] = useState(false);
  const [temporarilyUnavailable, setTemporarilyUnavailable] = useState(false);

  useEffect(() => {
    let cancelled = false, initialized = false;
    void (async () => {
      const saved = readOfflineSession<{ user: User; config: Config }>();
      let session: { user: User } | null;
      try { session = await loadSession(); }
      catch (error) {
        if (!networkFailure(error) || !saved) throw error;
        if (cancelled) return;
        userRef.current = saved.user; setUser(saved.user); setConfig(saved.config);
        setTemporarilyUnavailable(true); return;
      }
      if (cancelled) return;
      if (!session) { void disconnectPush(); forgetSession(); if (saved) void clearOffline(saved.user.id).catch(() => {}); }
      const nextConfig = await api<Config>("/v1/web/config").catch(error => { if (session && saved) return saved.config; throw error; });
      if (cancelled) return;
      setConfig(nextConfig); setAutomaticAllowed(nextConfig.autoSignIn === true);
      if (session) rememberSession({ user: session.user, config: nextConfig });
      userRef.current = session?.user ?? null; setUser(session?.user ?? null);
    })().catch(() => { if (!cancelled) setStartupError(true); }).finally(() => { if (!cancelled) { initialized = true; setChecking(false); } });
    const monitor = createSessionMonitor<User>({
      sessionURL: "/v1/web/session", configURL: "/v1/web/config", logoutURL: "/v1/web/logout", csrfHeader: "X-RSS-CSRF",
      isAuthenticated: () => userRef.current !== null,
      onSession: value => { userRef.current = value; setUser(value); setStartupError(false); const saved = readOfflineSession<{config: Config}>(); if (saved) rememberSession({ user: value, config: saved.config }); },
      onRevoked: () => { void disconnectPush(); const id = userRef.current?.id; forgetSession(); if (id) void clearOffline(id).catch(() => {}); userRef.current = null; setUser(null); },
      onAutoHint: setAutomaticAllowed, onUnavailable: setTemporarilyUnavailable,
    });
    monitorRef.current = monitor;
    const unauthorized = () => { void monitor.unauthorized(); };
    const resume = () => { if (initialized) void monitor.check(); };
    const visibility = () => { if (document.visibilityState === "visible") resume(); };
    const unavailable = () => setTemporarilyUnavailable(true);
    const interval = setInterval(resume, 15_000);
    window.addEventListener("rss:unauthorized", unauthorized);
    window.addEventListener("rss:auth-unavailable", unavailable);
    window.addEventListener("focus", resume);
    window.addEventListener("online", resume);
    window.addEventListener("pageshow", resume);
    document.addEventListener("visibilitychange", visibility);
    return () => { cancelled = true; monitor.dispose(); clearInterval(interval); window.removeEventListener("rss:unauthorized", unauthorized); window.removeEventListener("rss:auth-unavailable", unavailable); window.removeEventListener("focus", resume); window.removeEventListener("online", resume); window.removeEventListener("pageshow", resume); document.removeEventListener("visibilitychange", visibility); };
  }, []);

  const automaticURL = automaticSignInURL({
    enabled: !!config?.federated && automaticAllowed && !startupError,
    hasSession: !!user, attempted: automaticAttempted.current, location,
  });
  useEffect(() => {
    if (checking || !automaticURL || automaticAttempted.current) return;
    automaticAttempted.current = true;
    window.location.replace(automaticURL);
  }, [checking, automaticURL]);

  if (checking || automaticURL) return <div className="web-auth-shell"><p>Connecting to Gravitas…</p></div>;
  if (startupError) return <main className="web-auth-shell"><section className="web-auth-card"><p>Unable to connect. Please try again.</p><button className="btn" onClick={() => window.location.reload()}>Try again</button></section></main>;
  if (!user) return <Login config={config} onAuthenticated={value => { userRef.current = value; setUser(value); if (config) rememberSession({user: value, config}); }} />;

  const logout = async () => {
    await disconnectPush();
    await api("/v1/web/logout", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-RSS-CSRF": "1" },
      body: "{}",
    });
    history.replaceState(null, "", config?.federated ? "/login?auto=off" : "/login");
    monitorRef.current?.localSignedOut();
    userRef.current = null; setUser(null);
  };

  return (
    <div className="web-app-shell">
      <div className="web-session-bar">
        <span>{user.displayName || user.username}</span>
        {temporarilyUnavailable && <span role="status">Connection unavailable. Saved content is available on this device.</span>}
        <DeviceFeatures />
        <button className="btn" onClick={() => void logout().catch(() => setTemporarilyUnavailable(true))}>{config?.federated ? "Sign out of News" : "Log out"}</button>
      </div>
      <App key={user.id} backend={backend} serverAuthoritative />
    </div>
  );
}

function Login({ config, onAuthenticated }: { config: Config | null; onAuthenticated: (user: User) => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [inviteKey, setInviteKey] = useState("");
  const [registering, setRegistering] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const result = await api<{ user: User }>(registering ? "/v1/web/register" : "/v1/web/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password, ...(registering ? { inviteKey } : {}) }),
      });
      history.replaceState(null, "", "/");
      onAuthenticated(result.user);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Unable to sign in");
    } finally {
      setBusy(false);
    }
  };

  if (config?.federated) {
    const next = appReturnPath(location);
    const query = new URLSearchParams({ next }).toString();
    return <main className="web-auth-shell"><section className="web-auth-card">
      <p className="web-auth-kicker">RSS Reader</p><h1>Welcome to News</h1>
      <p>Use your Gravitas account to read and save your feeds.</p>
      <form className="gravitas-login" action="/auth/gravitas/start" method="get">
        <input type="hidden" name="next" value={next} />
        <label htmlFor="gravitas-username">Username</label>
        <div className="gravitas-login-row"><input id="gravitas-username" name="username" autoComplete="username" autoCapitalize="none" spellCheck={false} placeholder={`username@${config.defaultDomain}`} maxLength={256} required />
        <button className="btn primary" type="submit" aria-label="Sign in" title="Sign in">🔥</button></div>
        <small>Without a domain, sign in at {config.defaultDomain}.</small>
      </form>
      <a className="web-auth-switch" href={"/auth/gravitas/register?" + query}>Register at {config.defaultDomain}</a>
    </section></main>;
  }

  return (
    <main className="web-auth-shell">
      <form className="web-auth-card" onSubmit={submit}>
        <div>
          <p className="web-auth-kicker">RSS Reader</p>
          <h1>{registering ? "Create your account" : "Sign in"}</h1>
          <p>Your feeds, saved links, stories, and sports data are managed by this server.</p>
        </div>
        <label>Username<input autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} required minLength={3} /></label>
        <label>Password<input autoComplete={registering ? "new-password" : "current-password"} type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={10} /></label>
        {registering ? <label>Invite key<input type="password" autoComplete="off" value={inviteKey} onChange={(e) => setInviteKey(e.target.value)} required /></label> : null}
        {error ? <p className="error" role="alert">{error}</p> : null}
        <button className="btn primary" disabled={busy}>{busy ? "Please wait…" : registering ? "Create account" : "Sign in"}</button>
        {config?.registrationEnabled ? (
          <button className="web-auth-switch" type="button" onClick={() => { setRegistering((value) => !value); setError(null); }}>
            {registering ? "Already have an account? Sign in" : "Need an account? Register"}
          </button>
        ) : null}
        <small>Server {config?.version ?? ""}</small>
      </form>
    </main>
  );
}
