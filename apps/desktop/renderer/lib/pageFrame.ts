/** Sandbox flags for the full-page article iframe. No scripts: blob/srcdoc frames
 *  have an opaque origin (`null`), so page `fetch`/XHR (newsletter widgets, etc.)
 *  would CORS-fail and `history.replaceState` can break the parent shell. */
export const PAGE_FRAME_SANDBOX =
  "allow-popups allow-popups-to-escape-sandbox allow-downloads";

const PREVIEW_CSP =
  "default-src * data: blob:; img-src * data: blob:; media-src *; style-src * 'unsafe-inline'; font-src * data:; script-src 'none'; connect-src 'none'; worker-src 'none'; object-src 'none'; frame-src 'none'";

export function preparePageFrameHtml(html: string, pageUrl: string): string {
  return ensureHeadLead(html, pageUrl);
}

function ensureHeadLead(html: string, pageUrl: string): string {
  const href = baseHrefFor(pageUrl);
  const withoutBase = html.replace(/<base\b[^>]*>/gi, "");
  const lead = headLead(href);
  const headMatch = withoutBase.match(/<head[^>]*>/i);
  if (headMatch && headMatch.index != null) {
    const insertAt = headMatch.index + headMatch[0].length;
    return withoutBase.slice(0, insertAt) + lead + withoutBase.slice(insertAt);
  }
  const htmlMatch = withoutBase.match(/<html[^>]*>/i);
  if (htmlMatch && htmlMatch.index != null) {
    const insertAt = htmlMatch.index + htmlMatch[0].length;
    return withoutBase.slice(0, insertAt) + `<head>${lead}</head>` + withoutBase.slice(insertAt);
  }
  return `<head>${lead}</head>${withoutBase}`;
}

function headLead(baseHref: string): string {
  const csp = `<meta http-equiv="Content-Security-Policy" content="${escapeAttr(PREVIEW_CSP)}">`;
  if (!baseHref) {
    return csp;
  }
  return `${csp}<base href="${escapeAttr(baseHref)}">`;
}

function baseHrefFor(pageUrl: string): string {
  try {
    const u = new URL(pageUrl);
    return `${u.origin}/`;
  } catch {
    return pageUrl;
  }
}

function escapeAttr(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}
