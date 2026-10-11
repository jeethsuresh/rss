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
