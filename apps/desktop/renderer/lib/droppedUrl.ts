/** Normalize and validate a URL entered for Read Later. */

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

  const authority = candidate.match(/^https?:\/\/([^/?#]*)/i)?.[1];
  if (!authority || /%(?![0-9a-fA-F]{2})/.test(candidate)) {
    return { ok: false, attempted };
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
