#!/usr/bin/env bun
import { spawn, type Subprocess } from "bun";
import { mkdirSync, existsSync } from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dir, "..");
const desktop = path.join(root, "apps", "desktop");
const binDir = path.join(desktop, "resources", "bin");
const backendOut = path.join(binDir, process.platform === "win32" ? "rss-backend.exe" : "rss-backend");

mkdirSync(binDir, { recursive: true });

function run(cmd: string[], cwd = root, env?: Record<string, string>): Subprocess {
  return spawn({
    cmd,
    cwd,
    stdout: "inherit",
    stderr: "inherit",
    env: { ...process.env, ...env },
  });
}

async function buildBackend() {
  console.log("→ building Go backend…");
  const proc = run(["go", "build", "-o", backendOut, "./cmd/desktop"], path.join(root, "backend"));
  const code = await proc.exited;
  if (code !== 0) {
    throw new Error(`go build failed with ${code}`);
  }
  if (!existsSync(backendOut)) {
    throw new Error(`backend binary missing: ${backendOut}`);
  }
  console.log(`✓ backend → ${backendOut}`);
}

async function buildElectron() {
  console.log("→ bundling Electron main/preload…");
  // Main can be ESM; sandboxed preload must be CJS (Electron rejects ESM import syntax).
  const main = run(
    [
      "bun",
      "build",
      "./electron/main.ts",
      "--outdir",
      "./dist-electron",
      "--target",
      "node",
      "--format",
      "esm",
      "--external",
      "electron",
    ],
    desktop,
  );
  if ((await main.exited) !== 0) throw new Error("electron main bundle failed");
  const preload = run(
    [
      "bun",
      "build",
      "./electron/preload.ts",
      "--outdir",
      "./dist-electron",
      "--target",
      "node",
      "--format",
      "cjs",
      "--external",
      "electron",
    ],
    desktop,
  );
  if ((await preload.exited) !== 0) throw new Error("electron preload bundle failed");
}

const children: Subprocess[] = [];

function cleanup() {
  for (const child of children) {
    try {
      child.kill();
    } catch {
      // ignore
    }
  }
  process.exit(0);
}

process.on("SIGINT", cleanup);
process.on("SIGTERM", cleanup);

await buildBackend();
await buildElectron();

console.log("→ starting Vite…");
const vite = run(["bun", "x", "vite", "--host", "127.0.0.1", "--port", "5173"], desktop);
children.push(vite);

// wait for vite
await Bun.sleep(1200);

console.log("→ starting Electron…");
const electronBin = path.join(desktop, "node_modules", "electron", "cli.js");
const electron = run(["bun", electronBin, "."], desktop, {
  ELECTRON_RUN_AS_NODE: "",
  RSS_SEED: process.env.RSS_SEED ?? "0",
});
children.push(electron);

const code = await Promise.race([vite.exited, electron.exited]);
cleanup();
process.exit(code ?? 0);
