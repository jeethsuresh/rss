export type MobilePane = "sidebar" | "list" | "detail";

export type EdgeSwipe = {
  pane: MobilePane;
  width: number;
  sidebarWidth?: number;
  startX: number;
  startY: number;
  endX: number;
  endY: number;
  detailAvailable: boolean;
};

// Only edge gestures advance panes. Ordinary horizontal scrolling stays local
// to schedules, tables, and article content.
export function edgeSwipeDestination(swipe: EdgeSwipe, threshold = 56): MobilePane | null {
  const dx = swipe.endX - swipe.startX;
  const dy = swipe.endY - swipe.startY;
  if (Math.abs(dx) < threshold || Math.abs(dx) < Math.abs(dy) * 1.5) return null;

  const leftEdge = swipe.startX <= 28;
  const rightEdge = swipe.startX >= swipe.width - 28;
  const drawerEdge = swipe.pane === "sidebar" && swipe.sidebarWidth != null
    && swipe.startX >= swipe.sidebarWidth - 28 && swipe.startX <= swipe.sidebarWidth;

  if (dx > 0 && leftEdge) {
    if (swipe.pane === "detail") return "list";
    if (swipe.pane === "list") return "sidebar";
  }
  if (dx < 0 && (rightEdge || drawerEdge)) {
    if (swipe.pane === "sidebar") return "list";
    if (swipe.pane === "list" && swipe.detailAvailable) return "detail";
  }
  return null;
}
