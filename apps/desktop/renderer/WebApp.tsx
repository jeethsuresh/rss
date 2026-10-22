import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { App } from "./App";
import { createWebBackend, installWebDesktopShim } from "./lib/webBackend";
import { appReturnPath, automaticSignInURL } from "./lib/automaticSignIn";

type User = { id: string; username: string; displayName?: string };
type Config = { registrationEnabled: boolean; federated?: boolean; autoSignIn?: boolean; version: string };

if (!window.rss) installWebDesktopShim();

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, { credentials: "same-origin", ...init });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body?.error?.message || "Request failed");
  return body as T;
}

async function loadSession(): Promise<{ user: User } | null> {
  const response = await fetch("/v1/web/session", { credentials: "same-origin", cache: "no-store" });
  if (response.status === 401) return null;
  if (!response.ok) throw new Error("Unable to check your session");
  return response.json();
}

export function WebApp() {
  const backend = useMemo(() => createWebBackend(), []);
  const [user, setUser] = useState<User | null>(null);
  const [config, setConfig] = useState<Config | null>(null);
  const [checking, setChecking] = useState(true);
  const [startupError, setStartupError] = useState(false);
  const automaticAttempted = useRef(false);

  useEffect(() => {
    void Promise.all([
      api<Config>("/v1/web/config"),
      loadSession(),
    ]).then(([nextConfig, session]) => {
      setConfig(nextConfig);
      setUser(session?.user ?? null);
    }).catch(() => setStartupError(true)).finally(() => setChecking(false));
    const unauthorized = () => setUser(null);
    window.addEventListener("rss:unauthorized", unauthorized);
    return () => window.removeEventListener("rss:unauthorized", unauthorized);
  }, []);

  const automaticURL = automaticSignInURL({
    enabled: !!config?.federated && !!config.autoSignIn && !startupError,
    hasSession: !!user, attempted: automaticAttempted.current, location,
  });
  useEffect(() => {
    if (checking || !automaticURL || automaticAttempted.current) return;
    automaticAttempted.current = true;
    window.location.replace(automaticURL);
  }, [checking, automaticURL]);

  if (checking || automaticURL) return <div className="web-auth-shell"><p>Connecting to Gravitas…</p></div>;
  if (startupError) return <main className="web-auth-shell"><section className="web-auth-card"><p>Unable to connect. Please try again.</p><button className="btn" onClick={() => window.location.reload()}>Try again</button></section></main>;
  if (!user) return <Login config={config} onAuthenticated={setUser} />;

  const logout = async () => {
    await api("/v1/web/logout", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-RSS-CSRF": "1" },
      body: "{}",
    });
    history.replaceState(null, "", config?.federated ? "/login?auto=off" : "/login");
    setUser(null);
  };

  return (
    <div className="web-app-shell">
      <div className="web-session-bar">
        <span>{user.displayName || user.username}</span>
        <button className="btn" onClick={() => void logout()}>{config?.federated ? "Sign out of News" : "Log out"}</button>
      </div>
      <App backend={backend} serverAuthoritative />
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
      <a className="btn primary" href={"/auth/gravitas/start?" + query}>Sign in with Gravitas</a>
      <a className="web-auth-switch" href={"/auth/gravitas/register?" + query}>Register at gravitas.cool</a>
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
