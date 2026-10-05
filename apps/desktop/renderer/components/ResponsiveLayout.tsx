import { createContext, useContext, useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { flushSync } from "react-dom";
import { edgeSwipeDestination, paneSwipeProgress, shouldCompletePaneSwipe, type EdgeSwipe, type MobilePane } from "../lib/paneNavigation";
import { capturePaneScene, PaneSwipeMotion, type PaneScene } from "../lib/paneSwipeMotion";

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
  if (!pane) return null;
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
  const scenes = useRef<Partial<Record<"list" | "detail", PaneScene>>>({});
  const motion = useRef<PaneSwipeMotion | null>(null);
  const paneState = useRef({ sidebarOpen, detail, detailAvailable });
  paneState.current = { sidebarOpen, detail, detailAvailable };
  const saveScene = (pane: "list" | "detail") => {
    if (root.current && !motion.current && window.matchMedia("(max-width: 640px)").matches) {
      scenes.current[pane] = capturePaneScene(root.current, pane);
    }
  };
  const listScroll = useRef<{ element: HTMLElement; top: number; left: number }[]>([]);
  const rememberListScroll = () => {
    saveScene("list");
    listScroll.current = Array.from(root.current?.querySelectorAll<HTMLElement>(
      ".article-list, .mlb-dashboard, .f1-dashboard, .mlb-schedule-scroller, .f1-timeline-scroller",
    ) ?? []).map((element) => ({ element, top: element.scrollTop, left: element.scrollLeft }));
  };
  const detailChange = useRef(onDetailChange);
  detailChange.current = onDetailChange;

  const back = () => {
    if (detail) saveScene("detail");
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

  useLayoutEffect(() => {
    motion.current?.dispose();
    motion.current = null;
  }, [detail, sidebarOpen]);

  useEffect(() => {
    const element = root.current;
    if (!element) return;
    type Gesture = EdgeSwipe & {
      touchId: number;
      destination?: MobilePane;
      motion?: PaneSwipeMotion;
      samples: { x: number; time: number }[];
    };
    let gesture: Gesture | null = null;
    const settle = (current: Gesture, complete: boolean) => {
      const animation = current.motion;
      if (!animation || motion.current !== animation) return;
      animation.settle(complete, () => {
        motion.current = null;
        if (complete) {
          flushSync(() => {
            if (current.destination === "sidebar") setSidebarOpen(true);
            else {
              setSidebarOpen(false);
              detailChange.current(current.destination === "detail");
            }
          });
          animation.finishEntryAnimations();
        }
        animation.dispose();
        const state = paneState.current;
        if (!state.sidebarOpen) saveScene(state.detail ? "detail" : "list");
      });
    };
    const cancel = () => {
      if (gesture) settle(gesture, false);
      gesture = null;
    };
    const start = (event: TouchEvent) => {
      if (motion.current) return;
      gesture = null;
      if (!window.matchMedia("(max-width: 640px)").matches || event.touches.length !== 1
        || document.querySelector(".modal-backdrop")) return;
      if ((event.target as Element).closest("input, textarea, select, [contenteditable='true']")) return;
      const touch = event.touches[0];
      const state = paneState.current;
      const candidate: Gesture = {
        pane: state.sidebarOpen ? "sidebar" : state.detail ? "detail" : "list",
        width: element.getBoundingClientRect().width,
        sidebarWidth: element.querySelector(".sidebar")?.getBoundingClientRect().width,
        startX: touch.clientX, startY: touch.clientY,
        endX: touch.clientX, endY: touch.clientY,
        touchId: touch.identifier, detailAvailable: state.detailAvailable,
        samples: [{ x: touch.clientX, time: performance.now() }],
      };
      if (!edgeSwipeDestination({ ...candidate, endX: touch.clientX + 10 }, 8)
        && !edgeSwipeDestination({ ...candidate, endX: touch.clientX - 10 }, 8)) return;
      if (candidate.pane === "list") rememberListScroll();
      else if (candidate.pane === "detail") saveScene("detail");
      gesture = candidate;
    };
    const record = (current: Gesture, touch: Touch) => {
      current.endX = touch.clientX;
      current.endY = touch.clientY;
      if (touch.clientX !== current.samples.at(-1)?.x) {
        current.samples.push({ x: touch.clientX, time: performance.now() });
        current.samples = current.samples.slice(-3);
      }
    };
    const move = (event: TouchEvent) => {
      if (!gesture) return;
      if (event.touches.length !== 1 || document.querySelector(".modal-backdrop")) { cancel(); return; }
      const touch = event.touches[0];
      record(gesture, touch);
      if (!gesture.destination) {
        const dx = Math.abs(gesture.endX - gesture.startX);
        const dy = Math.abs(gesture.endY - gesture.startY);
        if (dy > 10 && dy > dx) { gesture = null; return; }
        const destination = edgeSwipeDestination(gesture, 8);
        if (!destination) return;
        let incoming: PaneScene | undefined;
        if (destination !== "sidebar" && gesture.pane !== "sidebar") {
          const liveDetail = element.querySelector(".reader-pane, .f1-detail-container, .mlb-dashboard");
          incoming = destination === "detail" && liveDetail
            ? capturePaneScene(element, "detail")
            : scenes.current[destination] ?? capturePaneScene(element, destination);
        }
        gesture.destination = destination;
        gesture.motion = new PaneSwipeMotion(element, gesture.pane, destination, incoming);
        motion.current = gesture.motion;
      }
      if (motion.current !== gesture.motion) { gesture = null; return; }
      if (event.cancelable) event.preventDefault();
      gesture.motion!.update(paneSwipeProgress(gesture, gesture.destination));
    };
    const end = (event: TouchEvent) => {
      if (!gesture) return;
      const touch = Array.from(event.changedTouches).find((item) => item.identifier === gesture?.touchId);
      if (!touch) { cancel(); return; }
      const current = gesture;
      gesture = null;
      if (!current.motion || !current.destination) return;
      if (event.cancelable) event.preventDefault();
      record(current, touch);
      const progress = paneSwipeProgress(current, current.destination);
      current.motion.update(progress);
      const last = current.samples.at(-1)!;
      const previous = current.samples.at(-2) ?? last;
      const direction = current.destination === "sidebar" || current.pane === "detail" ? 1 : -1;
      const velocity = performance.now() - last.time > 80 ? 0
        : direction * (last.x - previous.x) / Math.max(8, last.time - previous.time);
      settle(current, shouldCompletePaneSwipe(progress, current.motion.travel, velocity));
    };
    const resized = () => {
      gesture = null;
      motion.current?.dispose();
      motion.current = null;
    };
    element.addEventListener("touchstart", start, { passive: true });
    element.addEventListener("touchmove", move, { passive: false });
    element.addEventListener("touchend", end, { passive: false });
    element.addEventListener("touchcancel", cancel);
    window.addEventListener("resize", resized);
    return () => {
      resized();
      element.removeEventListener("touchstart", start);
      element.removeEventListener("touchmove", move);
      element.removeEventListener("touchend", end);
      element.removeEventListener("touchcancel", cancel);
      window.removeEventListener("resize", resized);
    };
  }, []);

  return (
    <PaneContext.Provider value={{ sidebarId, chrome, back, detail }}>
      <div ref={root} className={`responsive-shell ${detail ? "mobile-detail" : "mobile-list"} ${sidebarOpen ? "sidebar-open" : ""}`}
        onClickCapture={(event) => { if (motion.current) { event.preventDefault(); event.stopPropagation(); } }}>
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
          const selection = target.closest<HTMLElement>("button.article-row, .story-member-main, .mlb-today-game, .mlb-game-card-hitarea, .f1-race-card-hitarea");
          if (selection && !selection.classList.contains("meta-story-row")) {
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
        <button type="button" className="sidebar-scrim" aria-label="Close sidebar" aria-hidden={!sidebarOpen}
          disabled={!sidebarOpen} tabIndex={sidebarOpen ? 0 : -1} onClick={() => {
            setSidebarOpen(false); menu.current?.focus();
          }} />
      </div>
    </PaneContext.Provider>
  );
}
