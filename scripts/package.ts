#!/usr/bin/env bun
/**
 * Production package helper for the current host platform.
 * Stages the Go backend, then runs electron-builder.
 */
import { spawn } from "bun";
import path from "node:path";
import { existsSync } from "node:fs";

const root = path.resolve(import.meta.dir, "..");
const desktop = path.join(root, "apps", "desktop");

async function run(cmd: string[], cwd = root) {
  const proc = spawn({ cmd, cwd, stdout: "inherit", stderr: "inherit" });
  const code = await proc.exited;
  if (code !== 0) process.exit(code ?? 1);
}

const goos =
  process.platform === "win32" ? "windows" : process.platform === "darwin" ? "darwin" : "linux";
const goarch = process.arch === "arm64" ? "arm64" : "amd64";

await run(["bash", "scripts/stage-backend.sh", goos, goarch]);
await run(["bun", "run", "--filter", "@rss-reader/desktop", "build"]);

console.log("→ electron-builder");
await run(
  ["bun", "x", "electron-builder", "--config", "electron-builder.yml", "--publish", "never"],
  desktop,
);
console.log("✓ package complete — see apps/desktop/release/");
