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
