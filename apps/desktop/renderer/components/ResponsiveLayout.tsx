import { useEffect, useId, useRef, useState, type ReactNode } from "react";

// Both the hosted reader and Electron use this layout. Desktop grids stay in CSS.
export function ResponsiveLayout({ children, className = "", detail, onDetailChange, label, detailAvailable = true }: {
  children: ReactNode;
  className?: string;
  detail: boolean;
  onDetailChange: (detail: boolean) => void;
  label: string;
  detailAvailable?: boolean;
}) {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const id = useId();
  const root = useRef<HTMLDivElement>(null);
  const menu = useRef<HTMLButtonElement>(null);
  const back = useRef<HTMLButtonElement>(null);
  const lastTrigger = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!window.matchMedia("(max-width: 980px)").matches) return;
    if (detail) back.current?.focus();
    else lastTrigger.current?.focus({ preventScroll: true });
  }, [detail]);

  useEffect(() => {
    if (!sidebarOpen || !window.matchMedia("(max-width: 640px)").matches) return;
    const sidebar = root.current?.querySelector<HTMLElement>(".sidebar");
    const controls = () => Array.from(sidebar?.querySelectorAll<HTMLElement>("button:not(:disabled), a[href], input, select") ?? []);
    controls()[0]?.focus();
    const keydown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        setSidebarOpen(false);
        menu.current?.focus();
      }
      if (event.key === "Tab" && window.matchMedia("(max-width: 640px)").matches) {
        const items = controls();
        const first = items[0], last = items[items.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      }
    };
    window.addEventListener("keydown", keydown);
    return () => window.removeEventListener("keydown", keydown);
  }, [sidebarOpen]);

  return (
    <div ref={root} className={`responsive-shell ${detail ? "mobile-detail" : "mobile-list"} ${sidebarOpen ? "sidebar-open" : ""}`}>
      <nav className="mobile-pane-nav" aria-label={`${label} navigation`}>
        <button ref={menu} className="btn" aria-expanded={sidebarOpen} aria-controls={id}
          onClick={() => setSidebarOpen((open) => !open)}><span aria-hidden="true">☰</span> Browse</button>
        <strong>{label}</strong>
        {detail ? <button ref={back} className="btn" onClick={() => onDetailChange(false)}>← Back to list</button>
          : detailAvailable ? <button className="btn" onClick={() => onDetailChange(true)}>View detail →</button> : null}
      </nav>
      <div id={id} className={`layout ${className}`} onClickCapture={(event) => {
        if (!window.matchMedia("(max-width: 980px)").matches) return;
        const target = event.target as Element;
        const selection = target.closest<HTMLElement>("button.article-row, .story-member-main, .mlb-game-card-hitarea, .f1-race-card-hitarea");
        if (selection) lastTrigger.current = selection;
        if (target.closest(".sidebar button.nav-item, .sidebar button.feed-item, .sidebar .folder-item-name")) {
          setSidebarOpen(false);
          onDetailChange(false);
          menu.current?.focus();
        }
      }}>
        {children}
      </div>
      {sidebarOpen ? <button className="sidebar-scrim" aria-label="Close sidebar" onClick={() => {
        setSidebarOpen(false); menu.current?.focus();
      }} /> : null}
    </div>
  );
}
