# GitHub Pages Landing Implementation Plan

> **For agentic workers:** Execute inline in this session (user approved continuous run).

**Goal:** Static marketing site at `https://jeethsuresh.github.io/rss/` with capability copy and release download links.

**Architecture:** Hand-authored HTML/CSS/JS in `site/`; GitHub Actions deploys the folder as a project Pages site. `app.js` fetches the latest GitHub Release and rewrites download `href`s; pinned `v0.1.4` URLs are the fallback.

**Tech Stack:** Static HTML5, CSS3, vanilla JS, GitHub Pages (`actions/upload-pages-artifact` + `actions/deploy-pages`), GitHub Releases API.

## Global Constraints

- Base path `/rss/` for all relative assets and in-page anchors.
- No screenshots; no bundler; no Dota mentions.
- Downloads: dynamic latest first, then hardcoded `v0.1.4`.
- Visual: avoid purple gradients, cream+terracotta serif, broadsheet density.
- Product name “RSS Reader” is the hero-level brand signal.

## File map

| Path | Responsibility |
|------|----------------|
| `site/index.html` | Markup: hero, capabilities, downloads, footer |
| `site/styles.css` | Layout, type, motion, responsive rules |
| `site/app.js` | Release fetch, asset mapping, fallback URLs, DOM wiring |
| `.github/workflows/pages.yml` | Build-less deploy of `site/` to Pages |
| `README.md` | Link to live site |
| `docs/superpowers/specs/2026-08-20-github-pages-landing-design.md` | Approved design |
| `docs/superpowers/plans/2026-08-20-github-pages-landing.md` | This plan |

---

### Task 1: Static site markup + styles + download script

**Files:**
- Create: `site/index.html`, `site/styles.css`, `site/app.js`

- [ ] **Step 1:** Create `site/index.html` with `<base href="/rss/">`, hero, four capability sections, downloads list with `data-asset` keys (`mac-dmg`, `win-exe`, `linux-deb`, `linux-rpm`, `linux-pacman`, `linux-appimage`), footer, and script tag `app.js`.
- [ ] **Step 2:** Create `site/styles.css` with CSS variables, grain wash background, Syne + Figtree fonts, hero/composition layout, section spacing, download button row, reduced-motion support, mobile stack.
- [ ] **Step 3:** Create `site/app.js` with `FALLBACK_TAG = "v0.1.4"`, `FALLBACK_ASSETS` map of browser URLs for each key, `fetchLatestAssets()`, pattern matchers, and `applyDownloads(assets, tag)`.
- [ ] **Step 4:** Open `site/index.html` locally (or `python3 -m http.server` from `site` with temporary base `/`) and confirm structure; API fetch may work from file restrictions — prefer quick static check that fallback constants are valid URLs.

- [ ] **Step 5:** Commit site files.

---

### Task 2: Pages workflow + README

**Files:**
- Create: `.github/workflows/pages.yml`
- Modify: `README.md`

- [ ] **Step 1:** Add workflow: `permissions` for pages; concurrency group; job `deploy` on `ubuntu-latest`; checkout; `actions/configure-pages@v5`; `actions/upload-pages-artifact@v3` with `path: site`; `actions/deploy-pages@v4`. Triggers: `push` to `main` paths `site/**`, `.github/workflows/pages.yml`; plus `workflow_dispatch`.
- [ ] **Step 2:** Add README section linking `https://jeethsuresh.github.io/rss/`.
- [ ] **Step 3:** Commit workflow + README.

---

### Task 3: Enable Pages + verify deploy

- [ ] **Step 1:** `gh api` / `gh api repos/jeethsuresh/rss/pages` to set build_type `workflow` if needed.
- [ ] **Step 2:** Push `main`; watch Pages workflow; confirm site returns 200.
- [ ] **Step 3:** Update `ai-rss-reader` um note with Pages URL and download fallback behavior.

---

## Spec coverage

- Hosting `/rss/` + Actions → Tasks 2–3
- Hero + four capabilities + downloads + footer → Task 1
- Dynamic + fallback downloads → Task 1 `app.js`
- Visual constraints → Task 1 CSS
- README link → Task 2
