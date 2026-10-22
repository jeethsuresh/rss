type AppLocation = { pathname: string; search: string; hash: string };

export function appReturnPath(current: AppLocation): string {
  const requested = new URLSearchParams(current.search).get("next");
  if (requested) return requested + (requested.includes("#") ? "" : current.hash);
  return current.pathname === "/login" ? "/" + current.hash : current.pathname + current.search + current.hash;
}

export function automaticSignInURL(options: {
  enabled: boolean;
  hasSession: boolean;
  attempted: boolean;
  location: AppLocation;
}): string | null {
  if (!options.enabled || options.hasSession || options.attempted ||
      new URLSearchParams(options.location.search).get("auto") === "off") return null;
  return "/auth/gravitas/start?" + new URLSearchParams({ next: appReturnPath(options.location), prompt: "none" });
}
