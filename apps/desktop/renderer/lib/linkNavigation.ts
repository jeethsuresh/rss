export function browserPaneUrl(rawHref: string | null, baseUrl: string): string | null {
  const href = rawHref?.trim() ?? "";
  if (!href || href.startsWith("#")) return null;

  try {
    const url = new URL(href, baseUrl);
    return /^https?:$/i.test(url.protocol) ? url.href : null;
  } catch {
    return null;
  }
}

/** Absolute http(s) image URL. Rejects `file:` and other local schemes. */
export function safeHttpUrl(value: string | null | undefined): string | null {
  const raw = value?.trim() ?? "";
  if (!raw) return null;
  try {
    const url = new URL(raw);
    return url.protocol === "http:" || url.protocol === "https:" ? url.href : null;
  } catch {
    return null;
  }
}

/** True when the browser would navigate this anchor to a local file. */
export function isFileLink(resolvedHref: string): boolean {
  return resolvedHref.trim().toLowerCase().startsWith("file:");
}

/** Keep explicit browser actions native; ordinary article clicks use the pane. */
export function shouldOpenLinkInPane(
  event: Pick<MouseEvent, "defaultPrevented" | "button" | "ctrlKey" | "metaKey" | "shiftKey" | "altKey">,
  anchor: Pick<HTMLAnchorElement, "target" | "hasAttribute">,
): boolean {
  const target = anchor.target.trim().toLowerCase();
  return !event.defaultPrevented && event.button === 0
    && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey
    && (!target || target === "_self") && !anchor.hasAttribute("download");
}
