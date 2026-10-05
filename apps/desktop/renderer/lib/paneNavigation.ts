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


export function paneSwipeProgress(swipe: EdgeSwipe, destination: MobilePane): number {
  const travel = swipe.pane === "sidebar" || destination === "sidebar"
    ? swipe.sidebarWidth ?? swipe.width
    : swipe.width;
  const direction = destination === "sidebar" || swipe.pane === "detail" ? 1 : -1;
  return Math.max(0, Math.min(1, (swipe.endX - swipe.startX) * direction / travel));
}

// A deliberate drag or a short, quick flick completes the gesture. Reversing
// toward the starting edge cancels it, even after crossing the distance threshold.
export function shouldCompletePaneSwipe(progress: number, travel: number, velocity: number): boolean {
  if (velocity < -0.25) return false;
  return progress >= 0.38 || (progress * travel >= 24 && velocity >= 0.45);
}

export function paneSwipeDuration(progress: number, complete: boolean, reducedMotion: boolean): number {
  if (reducedMotion) return 0;
  const remaining = complete ? 1 - progress : progress;
  return Math.round(140 + Math.min(1, remaining) * 120);
}
