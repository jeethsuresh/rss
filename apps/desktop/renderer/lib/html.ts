import DOMPurify from "isomorphic-dompurify";

DOMPurify.addHook("uponSanitizeAttribute", (_node, data) => {
  if (data.attrName === "href" || data.attrName === "src") {
    const v = String(data.attrValue || "").trim().toLowerCase();
    if (v.startsWith("javascript:") || v.startsWith("data:text/html")) {
      data.keepAttr = false;
    }
  }
});

export function sanitizeArticleHtml(html: string): string {
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    FORBID_TAGS: ["script", "iframe", "object", "embed", "form", "input", "button", "textarea", "link"],
    FORBID_ATTR: ["onerror", "onload", "onclick", "style"],
    ALLOW_DATA_ATTR: false,
  });
}

export function formatRelativeTime(iso: string | null | undefined): string {
  if (!iso) return "";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "";
  const diff = Date.now() - t;
  const mins = Math.round(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m`;
  const hours = Math.round(mins / 60);
  if (hours < 48) return `${hours}h`;
  const days = Math.round(hours / 24);
  if (days < 14) return `${days}d`;
  return new Date(iso).toLocaleDateString();
}

export function stripHtml(html: string): string {
  const cleaned = sanitizeArticleHtml(html);
  return cleaned
    .replace(/<[^>]+>/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}
