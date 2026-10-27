import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";
import { offlineShell } from "./pwa/plugin";

export default defineConfig({
  plugins: [react(), offlineShell()],
  root: "renderer",
  base: "./",
  resolve: {
    alias: {
      "@shared": path.resolve(__dirname, "../../packages/shared/src"),
    },
  },
  server: {
    port: 5173,
    strictPort: true,
  },
  build: {
    outDir: path.resolve(__dirname, "dist-renderer"),
    emptyOutDir: true,
  },
});
