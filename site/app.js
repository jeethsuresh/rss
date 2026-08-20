const REPO = "jeethsuresh/rss";
const FALLBACK_TAG = "v0.1.4";

const FALLBACK_ASSETS = {
  "mac-dmg":
    "https://github.com/jeethsuresh/rss/releases/download/v0.1.4/RSS-Reader-0.1.4-mac-arm64.dmg",
  "win-exe":
    "https://github.com/jeethsuresh/rss/releases/download/v0.1.4/RSS-Reader-0.1.4-win-x64.exe",
  "linux-deb":
    "https://github.com/jeethsuresh/rss/releases/download/v0.1.4/RSS-Reader-0.1.4-linux-amd64.deb",
  "linux-rpm":
    "https://github.com/jeethsuresh/rss/releases/download/v0.1.4/RSS-Reader-0.1.4-linux-x86_64.rpm",
  "linux-pacman":
    "https://github.com/jeethsuresh/rss/releases/download/v0.1.4/RSS-Reader-0.1.4-linux-x64.pacman",
  "linux-appimage":
    "https://github.com/jeethsuresh/rss/releases/download/v0.1.4/RSS-Reader-0.1.4-linux-x86_64.AppImage",
};

/** @type {Record<string, (name: string) => boolean>} */
const MATCHERS = {
  "mac-dmg": (name) => /mac-arm64\.dmg$/i.test(name),
  "win-exe": (name) => /win-x64\.exe$/i.test(name),
  "linux-deb": (name) => /\.deb$/i.test(name),
  "linux-rpm": (name) => /\.rpm$/i.test(name),
  "linux-pacman": (name) => /\.pacman$/i.test(name),
  "linux-appimage": (name) => /\.AppImage$/i.test(name),
};

/**
 * @param {{ name: string, browser_download_url: string }[]} assets
 * @returns {Record<string, string>}
 */
function mapAssets(assets) {
  /** @type {Record<string, string>} */
  const out = {};
  for (const [key, match] of Object.entries(MATCHERS)) {
    const hit = assets.find((a) => match(a.name));
    if (hit) out[key] = hit.browser_download_url;
  }
  return out;
}

/**
 * @param {Record<string, string>} urls
 * @param {string} tag
 */
function applyDownloads(urls, tag) {
  const tagEl = document.getElementById("release-tag");
  if (tagEl) tagEl.textContent = tag;

  for (const link of document.querySelectorAll("a[data-asset]")) {
    const key = link.getAttribute("data-asset");
    if (!key) continue;
    const href = urls[key];
    if (href) link.setAttribute("href", href);
  }
}

async function loadDownloads() {
  applyDownloads(FALLBACK_ASSETS, FALLBACK_TAG);

  try {
    const res = await fetch(
      `https://api.github.com/repos/${REPO}/releases/latest`,
      {
        headers: { Accept: "application/vnd.github+json" },
      },
    );
    if (!res.ok) return;

    const data = await res.json();
    const mapped = mapAssets(data.assets ?? []);
    const complete = Object.keys(MATCHERS).every((k) => mapped[k]);
    if (!complete) return;

    applyDownloads(mapped, data.tag_name || FALLBACK_TAG);
  } catch {
    // Keep fallback URLs already applied.
  }
}

loadDownloads();
