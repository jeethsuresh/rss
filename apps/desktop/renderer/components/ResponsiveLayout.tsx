import { createContext, useContext, useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { edgeSwipeDestination, type EdgeSwipe } from "../lib/paneNavigation";

export type SidebarChrome = {
  navigation: ReactNode;
  footer: ReactNode;
  tools?: ReactNode;
};

const PaneContext = createContext<{
  sidebarId: string;
  chrome?: SidebarChrome;
  back: () => void;
  detail: boolean;
} | null>(null);

export function ReaderSidebar({ children, className = "" }: { children: ReactNode; className?: string }) {
  const pane = useContext(PaneContext);
  return (
    <aside id={pane?.sidebarId} className={`pane sidebar reader-sidebar ${className}`} aria-label="Reader navigation">
      {pane?.chrome ? (
        <div className="mobile-sidebar-top">
          {pane.chrome.navigation}
          {pane.chrome.tools}
        </div>
      ) : null}
      <div className="sidebar-content">{children}</div>
      {pane?.chrome ? <div className="mobile-sidebar-bottom">{pane.chrome.footer}</div> : null}
    </aside>
  );
}

export function DetailBackButton() {
  const pane = useContext(PaneContext);
  if (!pane?.detail) return null;
  return (
    <div className="detail-back-row">
      <button type="button" className="btn detail-back-button" onClick={pane.back}>
        <span aria-hidden="true">←</span> Back
      </button>
    </div>
  );
}

// Shared by the hosted reader and Electron. The desktop grid and chrome stay intact.
export function ResponsiveLayout({ children, className = "", detail, onDetailChange, label, detailAvailable = true, toolbar, chrome }: {
  children: ReactNode;
  className?: string;
  detail: boolean;
  onDetailChange: (detail: boolean) => void;
  label: string;
  detailAvailable?: boolean;
  toolbar?: ReactNode;
  chrome?: SidebarChrome;
}) {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const sidebarId = useId();
  const root = useRef<HTMLDivElement>(null);
  const menu = useRef<HTMLButtonElement>(null);
  const lastTrigger = useRef<HTMLElement | null>(null);
  const listScroll = useRef<{ element: HTMLElement; top: number; left: number }[]>([]);
  const rememberListScroll = () => {
    listScroll.current = Array.from(root.current?.querySelectorAll<HTMLElement>(
      ".article-list, .mlb-dashboard, .f1-dashboard, .mlb-schedule-scroller, .f1-timeline-scroller",
    ) ?? []).map((element) => ({ element, top: element.scrollTop, left: element.scrollLeft }));
  };
  const detailChange = useRef(onDetailChange);
  detailChange.current = onDetailChange;

  const back = () => {
    setSidebarOpen(false);
    onDetailChange(false);
  };

  useLayoutEffect(() => {
    if (detail || !window.matchMedia("(max-width: 980px)").matches) return;
    for (const saved of listScroll.current) {
      if (saved.element.isConnected) saved.element.scrollTo({ top: saved.top, left: saved.left, behavior: "auto" });
    }
  }, [detail]);

  useEffect(() => {
    if (!window.matchMedia("(max-width: 980px)").matches) return;
    if (detail) root.current?.querySelector<HTMLButtonElement>(".detail-back-button")?.focus({ preventScroll: true });
    else if (lastTrigger.current?.isConnected) lastTrigger.current.focus({ preventScroll: true });
    else menu.current?.focus({ preventScroll: true });
  }, [detail]);

  useEffect(() => {
    const media = window.matchMedia("(max-width: 640px)");
    const resized = () => { if (!media.matches) setSidebarOpen(false); };
    media.addEventListener("change", resized);
    return () => media.removeEventListener("change", resized);
  }, []);

  useEffect(() => {
    if (!sidebarOpen || !window.matchMedia("(max-width: 640px)").matches) return;
    const sidebar = root.current?.querySelector<HTMLElement>(".sidebar");
    const controls = () => Array.from(sidebar?.querySelectorAll<HTMLElement>("button:not(:disabled), a[href], input, select") ?? [])
      .filter((element) => element.getClientRects().length > 0);
    controls()[0]?.focus({ preventScroll: true });
    const keydown = (event: KeyboardEvent) => {
      if (document.querySelector(".modal-backdrop")) return;
      if (event.key === "Escape") {
        event.preventDefault();
        setSidebarOpen(false);
        menu.current?.focus();
      }
      if (event.key === "Tab") {
        const items = controls();
        const first = items[0], last = items[items.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      }
    };
    window.addEventListener("keydown", keydown);
    return () => window.removeEventListener("keydown", keydown);
  }, [sidebarOpen]);

  useEffect(() => {
    const element = root.current;
    if (!element) return;
    let gesture: (EdgeSwipe & { touchId: number }) | null = null;
    const start = (event: TouchEvent) => {
      gesture = null;
      if (!window.matchMedia("(max-width: 640px)").matches || event.touches.length !== 1
        || document.querySelector(".modal-backdrop")) return;
      if ((event.target as Element).closest("input, textarea, select, [contenteditable='true']")) return;
      const touch = event.touches[0];
      gesture = {
        pane: sidebarOpen ? "sidebar" : detail ? "detail" : "list",
        width: window.innerWidth,
        sidebarWidth: element.querySelector(".sidebar")?.getBoundingClientRect().width,
        startX: touch.clientX, startY: touch.clientY,
        endX: touch.clientX, endY: touch.clientY,
        touchId: touch.identifier, detailAvailable,
      };
    };
    const move = (event: TouchEvent) => {
      if (!gesture) return;
      if (event.touches.length !== 1) { gesture = null; return; }
      const touch = event.touches[0];
      gesture.endX = touch.clientX;
      gesture.endY = touch.clientY;
      if (edgeSwipeDestination(gesture, 12) && event.cancelable) event.preventDefault();
    };
    const end = (event: TouchEvent) => {
      if (!gesture) return;
      const touch = Array.from(event.changedTouches).find((item) => item.identifier === gesture?.touchId);
      if (!touch) { gesture = null; return; }
      const destination = edgeSwipeDestination({ ...gesture, endX: touch.clientX, endY: touch.clientY });
      gesture = null;
      if (destination === "sidebar") setSidebarOpen(true);
      else if (destination) {
        if (destination === "detail") rememberListScroll();
        setSidebarOpen(false);
        detailChange.current(destination === "detail");
      }
    };
    const cancel = () => { gesture = null; };
    element.addEventListener("touchstart", start, { passive: true });
    element.addEventListener("touchmove", move, { passive: false });
    element.addEventListener("touchend", end);
    element.addEventListener("touchcancel", cancel);
    return () => {
      element.removeEventListener("touchstart", start);
      element.removeEventListener("touchmove", move);
      element.removeEventListener("touchend", end);
      element.removeEventListener("touchcancel", cancel);
    };
  }, [sidebarOpen, detail, detailAvailable]);

  return (
    <PaneContext.Provider value={{ sidebarId, chrome, back, detail }}>
      <div ref={root} className={`responsive-shell ${detail ? "mobile-detail" : "mobile-list"} ${sidebarOpen ? "sidebar-open" : ""}`}>
        <header className="mobile-pane-nav" aria-label={`${label} controls`}>
          <button ref={menu} type="button" className="btn sidebar-menu-button" aria-label="Open sidebar"
            aria-expanded={sidebarOpen} aria-controls={sidebarId} onClick={() => setSidebarOpen((open) => !open)}>
            <span aria-hidden="true">☰</span>
          </button>
          {toolbar ?? <strong>{label}</strong>}
        </header>
        <div className={`layout ${className}`} onClickCapture={(event) => {
          if (!window.matchMedia("(max-width: 980px)").matches) return;
          const target = event.target as Element;
          const selection = target.closest<HTMLElement>("button.article-row, .story-member-main, .mlb-game-card-hitarea, .f1-race-card-hitarea");
          if (selection) {
            lastTrigger.current = selection;
            rememberListScroll();
          }
          if (target.closest(".sidebar button.nav-item, .sidebar button.feed-item, .sidebar .folder-item-name, .sidebar [role='tab']")) {
            back();
            menu.current?.focus();
          } else if (target.closest(".sidebar .section-add, .sidebar [data-dismiss-sidebar]")) {
            setSidebarOpen(false);
          }
        }}>
          {children}
        </div>
        {sidebarOpen ? <button type="button" className="sidebar-scrim" aria-label="Close sidebar" onClick={() => {
          setSidebarOpen(false); menu.current?.focus();
        }} /> : null}
      </div>
    </PaneContext.Provider>
  );
}
