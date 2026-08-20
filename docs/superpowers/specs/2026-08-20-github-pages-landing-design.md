# GitHub Pages Landing — Design

**Date:** 2026-08-20  
**Status:** Approved for implementation

## Goal

Ship a public static landing page at `https://jeethsuresh.github.io/rss/` that explains RSS Reader capabilities and offers platform download links from GitHub Releases.

## Decisions

| Topic | Choice |
|-------|--------|
| Hosting | Project Pages (`/rss/` base path) via GitHub Actions |
| Source tree | `site/` at repo root (static HTML/CSS/JS, no bundler) |
| Downloads | Fetch `releases/latest` first; fall back to pinned `v0.1.4` asset URLs |
| Content | Equal feature tour: Feeds, Read Later, Sports (MLB + F1), AI |
| Screenshots | None for v1 |
| Custom domain | Out of scope |

## Page structure

1. **Hero** — Brand “RSS Reader” as the primary signal; one supporting sentence (local-first desktop reader); CTA group linking to Downloads / GitHub. No cards, stats, or secondary marketing clutter in the first viewport.
2. **Capabilities** — Four equal sections (one job each): Feeds; Read Later; Sports (MLB + F1 only); AI assist.
3. **Downloads** — Buttons/links for macOS arm64 DMG, Windows x64 EXE, Linux `.deb` / `.rpm` / `.pacman` / AppImage. Note builds are unsigned. Secondary link to the full Releases page.
4. **Footer** — Repo link; note that data lives in local SQLite (`rss.db`).

## Downloads behavior

1. `GET https://api.github.com/repos/jeethsuresh/rss/releases/latest`
2. Map assets by filename patterns:
   - macOS: `*-mac-arm64.dmg`
   - Windows: `*-win-x64.exe`
   - Linux: `*.deb`, `*.rpm`, `*.pacman`, `*.AppImage`
3. On network/API failure or missing matches, use hardcoded `v0.1.4` browser_download_url values.
4. When dynamic load succeeds, show the release tag (e.g. `v0.1.4`) near the download section.

## Visual direction

- Light, editorial atmosphere: ink-on-paper with a soft cool wash and subtle grain — not purple gradients, not cream+terracotta serif, not dense broadsheet columns.
- Expressive display font + readable body font (CDN webfonts).
- CSS custom properties; 2–3 intentional motions (hero entrance, section reveal, CTA hover).
- Responsive single-column on small screens.

## CI / deploy

- Workflow `.github/workflows/pages.yml`: trigger on `main` pushes that touch `site/**` (and `workflow_dispatch`); upload `site/` as Pages artifact; deploy with `actions/deploy-pages`.
- Permissions: `contents: read`, `pages: write`, `id-token: write`.
- Repo must use Pages source = GitHub Actions.

## Out of scope

Real screenshots, custom domain, code signing, analytics, blog, Intel Mac builds.

## Success criteria

- Live URL loads over HTTPS with brand-first hero and four capability sections.
- Download links resolve to release assets (dynamic or fallback).
- Page works at `/rss/` base path on mobile and desktop widths.
