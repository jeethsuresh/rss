#!/usr/bin/env bun
/**
 * Production package helper.
 * Builds renderer + electron, ensures backend binary exists, then runs electron-builder if available.
 */
import { spawn } from "bun";
import path from "node:path";
import { existsSync } from "node:fs";

const root = path.resolve(import.meta.dir, "..");
const desktop = path.join(root, "apps", "desktop");
const bin = path.join(
  desktop,
  "resources",
  "bin",
  process.platform === "win32" ? "rss-backend.exe" : "rss-backend",
);

if (!existsSync(bin)) {
  console.error(`Missing backend binary at ${bin}. Run: bun run backend:build`);
  process.exit(1);
}

async function run(cmd: string[], cwd: string) {
  const proc = spawn({ cmd, cwd, stdout: "inherit", stderr: "inherit" });
  const code = await proc.exited;
  if (code !== 0) process.exit(code ?? 1);
}

const builder = path.join(desktop, "node_modules", "electron-builder", "cli.js");
if (!existsSync(builder)) {
  console.log("electron-builder not installed — installing…");
  await run(["bun", "add", "-d", "electron-builder"], desktop);
}

console.log("→ electron-builder");
await run(["bun", "x", "electron-builder", "--config", "electron-builder.yml"], desktop);
console.log("✓ package complete — see apps/desktop/release/");
