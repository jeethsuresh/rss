import {
  app,
  BrowserWindow,
  ipcMain,
  shell,
  Notification,
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
const pendingDroppedTexts: string[] = [];

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

function deliverDroppedText(text: string) {
  const trimmed = text.trim();
  if (!trimmed) return;
  if (!mainWindow || mainWindow.isDestroyed()) {
    pendingDroppedTexts.push(trimmed);
    return;
  }
  focusMainWindow();
  const send = () => {
    if (!mainWindow || mainWindow.isDestroyed()) {
      pendingDroppedTexts.push(trimmed);
      return;
    }
    mainWindow.webContents.send("desktop:dropped-text", trimmed);
  };
  if (mainWindow.webContents.isLoading()) {
    mainWindow.webContents.once("did-finish-load", send);
  } else {
    send();
  }
}

function flushPendingDrops() {
  if (!mainWindow || mainWindow.isDestroyed()) return;
  while (pendingDroppedTexts.length > 0) {
    const text = pendingDroppedTexts.shift();
    if (text) deliverDroppedText(text);
  }
}

function readWeblocURL(filePath: string): string | null {
  try {
    const raw = fs.readFileSync(filePath, "utf8");
    const xmlMatch = raw.match(/<string>(https?:\/\/[^<]+)<\/string>/i);
    if (xmlMatch?.[1]) return xmlMatch[1].trim();
    const plistMatch = raw.match(/URL\s*=\s*"([^"]+)"/);
    if (plistMatch?.[1]) return plistMatch[1].trim();
  } catch {
    // ignore
  }
  return null;
}

async function createWindow() {
  backend = await startBackend();

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
    },
  });

  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    void shell.openExternal(url);
    return { action: "deny" };
  });

  backend.onEvent((event) => {
    mainWindow?.webContents.send("backend:event", event);
    if (
      event.event === "articles.added" &&
      Notification.isSupported()
    ) {
      // Notifications gated by settings in renderer; main only relays unless settings say so later.
    }
  });

  mainWindow.webContents.on("did-finish-load", () => {
    flushPendingDrops();
  });

  if (isDev) {
    await mainWindow.loadURL("http://localhost:5173");
    mainWindow.webContents.openDevTools({ mode: "detach" });
  } else {
    await mainWindow.loadFile(path.join(__dirname, "..", "dist-renderer", "index.html"));
  }
  flushPendingDrops();
}

function setupIpc() {
  ipcMain.handle("backend:request", async (_evt, method: string, params: unknown) => {
    if (!backend) {
      throw new Error("Backend not ready");
    }
    return backend.request(method, params ?? {});
  });

  ipcMain.handle("shell:openExternal", async (_evt, url: string) => {
    if (typeof url !== "string" || !/^https?:\/\//i.test(url)) {
      throw new Error("Invalid URL");
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

app.on("will-finish-launching", () => {
  app.on("open-url", (event, url) => {
    event.preventDefault();
    deliverDroppedText(url);
  });
  app.on("open-file", (event, filePath) => {
    event.preventDefault();
    const lower = filePath.toLowerCase();
    if (lower.endsWith(".webloc") || lower.endsWith(".url")) {
      const url = readWeblocURL(filePath);
      if (url) deliverDroppedText(url);
      else deliverDroppedText(filePath);
      return;
    }
    deliverDroppedText(filePath);
  });
});

app.whenReady().then(async () => {
  setupIpc();
  try {
    await createWindow();
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
