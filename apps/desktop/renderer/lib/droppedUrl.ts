/** Normalize / validate text dropped for Read Later. */

export type DroppedUrlResult =
  | { ok: true; url: string }
  | { ok: false; attempted: string };

function firstCandidate(raw: string): string {
  const text = raw.replace(/\0/g, "").trim();
  if (!text) return "";
  const uriList = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .find((line) => line && !line.startsWith("#"));
  return (uriList ?? text).trim();
}

/**
 * Accept http(s) URLs, or scheme-less hosts that become https://…
 * Reject empty, other schemes, and strings that are not URL-like.
 */
export function normalizeDroppedUrl(raw: string): DroppedUrlResult {
  const attempted = firstCandidate(raw);
  if (!attempted) {
    return { ok: false, attempted: raw.trim() };
  }

  let candidate = attempted;
  if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(candidate)) {
    candidate = `https://${candidate}`;
  }

  let parsed: URL;
  try {
    parsed = new URL(candidate);
  } catch {
    return { ok: false, attempted };
  }

  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    return { ok: false, attempted };
  }
  if (!parsed.hostname) {
    return { ok: false, attempted };
  }

  return { ok: true, url: parsed.toString() };
}

export function isEditableDropTarget(target: EventTarget | null): boolean {
  if (target == null) return false;
  if (typeof Element === "undefined" || !(target instanceof Element)) return false;
  const el = target.closest("input, textarea, select, [contenteditable=''], [contenteditable='true']");
  return el != null;
}

export function extractDropText(dataTransfer: DataTransfer | null): string {
  if (!dataTransfer) return "";
  const uri = dataTransfer.getData("text/uri-list")?.trim();
  if (uri) return uri;
  return dataTransfer.getData("text/plain") ?? "";
}
