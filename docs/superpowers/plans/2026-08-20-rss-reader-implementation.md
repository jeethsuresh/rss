# RSS Reader Implementation Plan

> **For agentic workers:** Execute phases 0–8 in order. Keep the app runnable after each phase. Commit after each phase. REQUIRED: verification before claiming done.

**Goal:** Production-quality local-first desktop RSS reader (Electron + React + Go + SQLite) with architecture ready for a future standalone Go server.

**Architecture:** Electron shell ↔ stdin/stdout JSON-RPC ↔ Go application services ↔ SQLite. React uses a `ReaderBackend` abstraction only.

**Tech Stack:** Bun, Electron, React, TypeScript, Vite, Go, SQLite (modernc.org/sqlite or mattn/go-sqlite3 — prefer pure Go `modernc.org/sqlite` for easier cross-compile), gofeed for RSS.

## Global Constraints

- No Node/Bun backend; no localhost HTTP for IPC unless forced
- No auth, accounts, cloud sync, or `cmd/server` implementation in this effort
- No business logic or SQLite in React/Electron TS
- Article HTML is untrusted; sanitize; open external in system browser
- `contextIsolation: true`, `nodeIntegration: false`
- Prefer mature libraries; YAGNI

## Phase execution order

0. Contracts + skeleton  
1. Scaffold (`bun dev`)  
2. SQLite + repos  
3. Vertical slice (add feed → read)  
4. Scheduler + folders  
5. Search + UX hydration  
6. Hardening + tests  
7. Packaging + README  
8. Future-server docs only  

## Verification gates

After Phase 1: `bun install && bun dev` launches; `system.ping` works; clean quit.  
After Phase 3: full add-feed lifecycle persists.  
After Phase 7: `bun test`, `go test ./...`, production build scripts exist.  
Final: Definition of Done checklist in original build spec §44.
