import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import type { Plugin } from "vite";

export function offlineShell(): Plugin {
  return {
    name: "rss-offline-shell",
    apply: "build",
    enforce: "post",
    generateBundle(_options, bundle) {
      const assets = [
        "/index.html",
        "/manifest.webmanifest",
        "/icons/icon-192.png",
        "/icons/icon-512.png",
        "/icons/apple-touch-icon.png",
        ...Object.keys(bundle).map((file) => `/${file}`),
      ];
      const template = readFileSync(
        new URL("./sw.template.js", import.meta.url),
        "utf8",
      );
      const hash = createHash("sha256").update(template);
      for (const asset of [
        "/manifest.webmanifest",
        "/icons/icon-192.png",
        "/icons/icon-512.png",
        "/icons/apple-touch-icon.png",
      ]) {
        hash.update(
          readFileSync(new URL("../renderer/public" + asset, import.meta.url)),
        );
      }
      for (const item of Object.values(bundle))
        hash.update(item.type === "chunk" ? item.code : item.source);
      this.emitFile({
        type: "asset",
        fileName: "sw.js",
        source: template
          .replace("__VERSION__", hash.digest("hex").slice(0, 12))
          .replace("__ASSETS__", JSON.stringify([...new Set(assets)])),
      });
    },
  };
}
