import {
  app,
  BrowserWindow,
  ipcMain,
  shell,
  Notification,
  Tray,
  Menu,
  clipboard,
  nativeImage,
  safeStorage,
} from "electron";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import fs from "node:fs";
import { BackendBridge } from "./backend-bridge.js";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const isDev = !app.isPackaged;
let mainWindow: BrowserWindow | null = null;
let backend: BackendBridge | null = null;
let backendProc: ChildProcessWithoutNullStreams | null = null;
let tray: Tray | null = null;
let pendingAddLinkText: string | null = null;
let backendEventsSubscribed = false;
const GENERIC_ERROR_MESSAGE = "Something went wrong. Please try again.";

type StoredServerSession = {
  serverUrl: string;
  username: string;
  token: string;
  expiresAt: string;
};

function serverSessionPath(): string {
  return path.join(app.getPath("userData"), "server-session.bin");
}

function isStoredServerSession(value: unknown): value is StoredServerSession {
  if (!value || typeof value !== "object") return false;
  const session = value as Partial<StoredServerSession>;
  return (
    typeof session.serverUrl === "string" && /^https?:\/\//i.test(session.serverUrl) &&
    typeof session.username === "string" && session.username.length > 0 &&
    typeof session.token === "string" && session.token.length > 0 &&
    typeof session.expiresAt === "string" && Number.isFinite(Date.parse(session.expiresAt))
  );
}

async function loadServerSession(): Promise<StoredServerSession | null> {
  if (!safeStorage.isEncryptionAvailable()) return null;
  try {
    const encrypted = await fs.promises.readFile(serverSessionPath());
    const parsed: unknown = JSON.parse(safeStorage.decryptString(encrypted));
    return isStoredServerSession(parsed) ? parsed : null;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== "ENOENT") {
      console.error("Unable to read saved server session", error);
    }
    return null;
  }
}

async function saveServerSession(session: StoredServerSession): Promise<void> {
  if (!safeStorage.isEncryptionAvailable()) {
    throw new Error("OS credential encryption is unavailable");
  }
  const target = serverSessionPath();
  const temporary = `${target}.tmp`;
  const encrypted = safeStorage.encryptString(JSON.stringify(session));
  await fs.promises.writeFile(temporary, encrypted, { mode: 0o600 });
  await fs.promises.rename(temporary, target);
}

async function clearServerSession(): Promise<void> {
  try {
    await fs.promises.unlink(serverSessionPath());
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
  }
}

async function restoreServerSession(bridge: BackendBridge): Promise<void> {
  const session = await loadServerSession();
  if (!session) return;
  if (Date.parse(session.expiresAt) <= Date.now() + 60_000) {
    await clearServerSession();
    return;
  }
  try {
    await bridge.request("sync.connect", {
      serverUrl: session.serverUrl,
      username: session.username,
      password: "",
      register: false,
      sessionToken: session.token,
      sessionExpiresAt: session.expiresAt,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    console.error("Unable to restore saved server session", message);
    if (/expired|invalid session|unauthorized/i.test(message)) {
      await clearServerSession();
    }
  }
}

function backendBinaryPath(): string {
  const resources = isDev
    ? path.join(__dirname, "..", "resources", "bin")
    : path.join(process.resourcesPath, "bin");
  const name = process.platform === "win32" ? "rss-backend.exe" : "rss-backend";
  return path.join(resources, name);
}

async function startBackend(): Promise<BackendBridge> {
  const bin = backendBinaryPath();
  if (!fs.existsSync(bin)) {
    throw new Error(`Backend binary missing at ${bin}. Run backend build first.`);
  }
  const dbPath = path.join(app.getPath("userData"), "rss.db");
  const args = ["-db", dbPath];
  if (process.env.RSS_SEED === "1") {
    args.push("-seed");
  }
  backendProc = spawn(bin, args, {
    stdio: ["pipe", "pipe", "pipe"],
    env: { ...process.env },
  });
  backendProc.stderr.on("data", (buf: Buffer) => {
    console.error("[backend]", buf.toString());
  });
  backendProc.on("exit", (code, signal) => {
    console.error("[backend] exited", { code, signal });
  });
  const bridge = new BackendBridge(backendProc.stdin, backendProc.stdout);
  await bridge.request("system.handshake", {});
  void restoreServerSession(bridge);
  return bridge;
}

function focusMainWindow() {
  if (!mainWindow) return;
  if (mainWindow.isMinimized()) mainWindow.restore();
  mainWindow.show();
  mainWindow.focus();
  if (process.platform === "darwin") {
    app.dock?.show();
    app.focus({ steal: true });
  }
}

function requestAddLink(text = "") {
  if (!mainWindow || mainWindow.isDestroyed()) {
    pendingAddLinkText = text;
    void createWindow().catch((error) => {
      console.error(error);
      void recordError("desktop", "window.create", error);
    });
    return;
  }
  focusMainWindow();
  const send = () => {
    if (!mainWindow || mainWindow.isDestroyed()) {
      pendingAddLinkText = text;
      return;
    }
    mainWindow.webContents.send("desktop:add-link-requested", text);
  };
  if (mainWindow.webContents.isLoading()) {
    mainWindow.webContents.once("did-finish-load", send);
  } else {
    send();
  }
}

function flushPendingAddLink() {
  if (!mainWindow || mainWindow.isDestroyed()) return;
  if (pendingAddLinkText !== null) {
    const text = pendingAddLinkText;
    pendingAddLinkText = null;
    requestAddLink(text);
  }
}

function trayIcon() {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 18 18"><g fill="none" stroke="black" stroke-width="1.8" stroke-linecap="round"><path d="M3 4.5a10.5 10.5 0 0 1 10.5 10.5"/><path d="M3 8.5A6.5 6.5 0 0 1 9.5 15"/></g><circle cx="3.2" cy="14.8" r="1.7" fill="black"/></svg>`;
  const icon = nativeImage.createFromDataURL(`data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`);
  icon.setTemplateImage(true);
  return icon;
}

function setupTray() {
  tray = new Tray(trayIcon());
  tray.setToolTip("RSS Reader");
  tray.setContextMenu(Menu.buildFromTemplate([
    {
      label: "Add Link…",
      click: () => {
        const text = clipboard.readText().trim();
        requestAddLink(/^https?:\/\//i.test(text) ? text : "");
      },
    },
    { type: "separator" },
    { label: "Show RSS Reader", click: () => focusMainWindow() },
    { label: "Quit", click: () => app.quit() },
  ]));
}

async function createWindow() {
  if (!backend) {
    backend = await startBackend();
  }

  mainWindow = new BrowserWindow({
    width: 1280,
    height: 840,
    minWidth: 900,
    minHeight: 600,
    title: "RSS Reader",
    backgroundColor: "#0f1419",
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
      webviewTag: true,
    },
  });
  mainWindow.once("closed", () => {
    mainWindow = null;
  });

  mainWindow.webContents.on("will-attach-webview", (_event, webPreferences) => {
    webPreferences.nodeIntegration = false;
    webPreferences.contextIsolation = true;
    webPreferences.sandbox = true;
  });

  mainWindow.webContents.on("did-attach-webview", (_event, guestContents) => {
    guestContents.setWindowOpenHandler(({ url }) => {
      if (/^https?:\/\//i.test(url)) {
        mainWindow?.webContents.send("desktop:open-in-pane", url);
      }
      return { action: "deny" };
    });
  });

  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    if (/^https?:\/\//i.test(url)) {
      mainWindow?.webContents.send("desktop:open-in-pane", url);
    }
    return { action: "deny" };
  });

  mainWindow.webContents.on("will-navigate", (event, url) => {
    if (!/^https?:\/\//i.test(url)) return;
    const appOrigin = isDev ? "http://localhost:5173" : null;
    if (appOrigin && new URL(url).origin === appOrigin) return;
    event.preventDefault();
    mainWindow?.webContents.send("desktop:open-in-pane", url);
  });

  mainWindow.webContents.on("will-frame-navigate", (event) => {
    const url = event.url;
    if (!/^https?:\/\//i.test(url)) return;
    const appOrigin = isDev ? "http://localhost:5173" : null;
    if (event.isMainFrame && appOrigin && new URL(url).origin === appOrigin) return;
    event.preventDefault();
    mainWindow?.webContents.send("desktop:open-in-pane", url);
  });

  if (!backendEventsSubscribed) {
    backendEventsSubscribed = true;
    backend.onEvent((event) => {
      mainWindow?.webContents.send("backend:event", event);
      if (event.event === "feed.error") {
        const payload = event.payload as { error?: string };
        void recordError("backend", "feed.refresh", payload.error ?? "Feed refresh failed");
      } else if (event.event === "sports.refresh") {
        const payload = event.payload as { phase?: string; key?: string; error?: string };
        if (payload.phase === "error") {
          void recordError("backend", `sports.refresh:${payload.key ?? "unknown"}`, payload.error ?? "Sports refresh failed");
        }
      } else if (event.event === "ai.log") {
        const payload = event.payload as { level?: string; message?: string; detail?: string };
        if (payload.level === "error") {
          void recordError(
            "ai",
            payload.message ?? "ai.operation",
            payload.detail ?? payload.message ?? "AI operation failed",
          );
        }
      }
    });
  }

  mainWindow.webContents.on("did-finish-load", () => {
    flushPendingAddLink();
  });

  if (isDev) {
    await mainWindow.loadURL("http://localhost:5173");
    mainWindow.webContents.openDevTools({ mode: "detach" });
  } else {
    await mainWindow.loadFile(path.join(__dirname, "..", "dist-renderer", "index.html"));
  }
  flushPendingAddLink();
}

async function recordError(source: string, operation: string, error: unknown) {
  if (!backend) return;
  const detail = error instanceof Error ? error.message : String(error);
  try {
    await backend.request("errors.record", {
      source,
      operation,
      message: `${operation} failed`,
      detail,
    });
  } catch (logError) {
    console.error("Failed to persist error log", logError);
  }
}

function setupIpc() {
  ipcMain.handle("backend:request", async (_evt, method: string, params: unknown) => {
    if (!backend) {
      throw new Error(GENERIC_ERROR_MESSAGE);
    }
    try {
      const result = await backend.request(method, params ?? {});
      if (method === "sync.connect") {
        try {
          const session = (await backend.request("sync.session.get", {})) as StoredServerSession;
          if (isStoredServerSession(session)) await saveServerSession(session);
        } catch (sessionError) {
          console.error("Unable to save server session", sessionError);
        }
      } else if (method === "sync.disconnect") {
        try {
          await clearServerSession();
        } catch (sessionError) {
          console.error("Unable to clear saved server session", sessionError);
        }
      }
      return result;
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error);
      // Story groups are rebuilt in the background, so a selected story can
      // legitimately disappear between stories.list and stories.get. Treat
      // that stale selection as an empty optional result instead of surfacing
      // an Electron handler exception.
      if (method === "stories.get" && /(?:NOT_FOUND|remote status 404)/i.test(detail)) {
        return null;
      }
      if (method !== "errors.record") {
        await recordError("renderer", method, error);
      }
      throw new Error(GENERIC_ERROR_MESSAGE);
    }
  });

  ipcMain.handle("shell:openExternal", async (_evt, url: string) => {
    if (typeof url !== "string" || !/^https?:\/\//i.test(url)) {
      throw new Error(GENERIC_ERROR_MESSAGE);
    }
    await shell.openExternal(url);
  });

  ipcMain.handle("app:notify", async (_evt, title: string, body: string) => {
    if (!Notification.isSupported()) return false;
    new Notification({ title, body }).show();
    return true;
  });

  ipcMain.handle("app:focusMainWindow", async () => {
    focusMainWindow();
  });
}

app.whenReady().then(async () => {
  setupIpc();
  try {
    await createWindow();
    setupTray();
  } catch (err) {
    console.error(err);
    app.quit();
  }
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") {
    app.quit();
  }
});

app.on("activate", () => {
  if (BrowserWindow.getAllWindows().length === 0) {
    void createWindow();
  }
});

app.on("before-quit", () => {
  try {
    backend?.request("system.shutdown", {}).catch(() => undefined);
  } catch {
    // ignore
  }
  if (backendProc && !backendProc.killed) {
    backendProc.kill("SIGTERM");
    setTimeout(() => {
      if (backendProc && !backendProc.killed) {
        backendProc.kill("SIGKILL");
      }
    }, 2000);
  }
});
