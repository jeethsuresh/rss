import { useEffect, useMemo, useState, type FormEvent } from "react";
import { App } from "./App";
import { createWebBackend, installWebDesktopShim } from "./lib/webBackend";

type User = { id: string; username: string };
type Config = { registrationEnabled: boolean; version: string };

if (!window.rss) installWebDesktopShim();

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, { credentials: "same-origin", ...init });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body?.error?.message || "Request failed");
  return body as T;
}

export function WebApp() {
  const backend = useMemo(() => createWebBackend(), []);
  const [user, setUser] = useState<User | null>(null);
  const [config, setConfig] = useState<Config | null>(null);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    void Promise.all([
      api<Config>("/v1/web/config"),
      api<{ user: User }>("/v1/web/session").catch(() => null),
    ]).then(([nextConfig, session]) => {
      setConfig(nextConfig);
      setUser(session?.user ?? null);
      setChecking(false);
    });
    const unauthorized = () => setUser(null);
    window.addEventListener("rss:unauthorized", unauthorized);
    return () => window.removeEventListener("rss:unauthorized", unauthorized);
  }, []);

  if (checking) return <div className="web-auth-shell"><p>Connecting to the server…</p></div>;
  if (!user) return <Login config={config} onAuthenticated={setUser} />;

  const logout = async () => {
    await api("/v1/web/logout", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-RSS-CSRF": "1" },
      body: "{}",
    });
    setUser(null);
    history.replaceState(null, "", "/login");
  };

  return (
    <div className="web-app-shell">
      <div className="web-session-bar">
        <span>{user.username}</span>
        <button className="btn" onClick={() => void logout()}>Log out</button>
      </div>
      <App backend={backend} serverAuthoritative />
    </div>
  );
}

function Login({ config, onAuthenticated }: { config: Config | null; onAuthenticated: (user: User) => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
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
        body: JSON.stringify({ username, password }),
      });
      history.replaceState(null, "", "/");
      onAuthenticated(result.user);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Unable to sign in");
    } finally {
      setBusy(false);
    }
  };

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
