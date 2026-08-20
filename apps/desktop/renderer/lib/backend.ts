import type { BackendEvent, ReaderBackend } from "@rss-reader/shared";

declare global {
  interface Window {
    rss: ReaderBackend;
    desktop: {
      openExternal: (url: string) => Promise<void>;
      notify: (title: string, body: string) => Promise<boolean>;
    };
  }
}

export function getBackend(): ReaderBackend {
  if (!window.rss) {
    throw new Error("Backend bridge unavailable");
  }
  return window.rss;
}

export type { BackendEvent };
