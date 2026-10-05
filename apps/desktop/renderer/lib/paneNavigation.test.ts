import { describe, expect, test } from "bun:test";
import { edgeSwipeDestination, paneSwipeDuration, paneSwipeProgress, shouldCompletePaneSwipe, type EdgeSwipe } from "./paneNavigation";

const gesture: EdgeSwipe = {
  pane: "list", width: 390, startX: 8, startY: 200,
  endX: 100, endY: 210, detailAvailable: true,
};

describe("mobile edge navigation", () => {
  test("moves back from detail to list and from list to sidebar", () => {
    expect(edgeSwipeDestination(gesture)).toBe("sidebar");
    expect(edgeSwipeDestination({ ...gesture, pane: "detail" })).toBe("list");
    expect(edgeSwipeDestination({ ...gesture, pane: "sidebar" })).toBeNull();
  });

  test("moves forward from sidebar to list and from list to detail", () => {
    const forward = { ...gesture, startX: 382, endX: 280 };
    expect(edgeSwipeDestination(forward)).toBe("detail");
    expect(edgeSwipeDestination({ ...forward, pane: "sidebar" })).toBe("list");
    expect(edgeSwipeDestination({ ...forward, pane: "detail" })).toBeNull();
  });

  test("allows closing the drawer from its own right edge", () => {
    expect(edgeSwipeDestination({
      ...gesture, pane: "sidebar", sidebarWidth: 300, startX: 292, endX: 180,
    })).toBe("list");
  });

  test("does not open empty detail panes", () => {
    expect(edgeSwipeDestination({ ...gesture, startX: 382, endX: 280, detailAvailable: false })).toBeNull();
  });

  test("leaves vertical scrolling, taps, and gestures inside content alone", () => {
    expect(edgeSwipeDestination({ ...gesture, endX: 25 })).toBeNull();
    expect(edgeSwipeDestination({ ...gesture, endY: 400 })).toBeNull();
    expect(edgeSwipeDestination({ ...gesture, startX: 180, endX: 280 })).toBeNull();
    expect(edgeSwipeDestination({ ...gesture, startX: 180, endX: 80 })).toBeNull();
  });
});


describe("interactive pane swipes", () => {
  test("tracks pane travel in both directions and clamps overshoot", () => {
    expect(paneSwipeProgress({ ...gesture, pane: "detail", endX: 203 }, "list")).toBe(0.5);
    expect(paneSwipeProgress({ ...gesture, startX: 382, endX: 187 }, "detail")).toBe(0.5);
    expect(paneSwipeProgress({ ...gesture, pane: "detail", endX: 500 }, "list")).toBe(1);
    expect(paneSwipeProgress({ ...gesture, pane: "detail", endX: 0 }, "list")).toBe(0);
  });

  test("measures drawer progress against the drawer width", () => {
    expect(paneSwipeProgress({ ...gesture, sidebarWidth: 300, endX: 158 }, "sidebar")).toBe(0.5);
    expect(paneSwipeProgress({ ...gesture, pane: "sidebar", sidebarWidth: 300, startX: 292, endX: 142 }, "list")).toBe(0.5);
  });

  test("cancels a slow partial drag and completes a deliberate drag", () => {
    expect(shouldCompletePaneSwipe(0.2, 390, 0.1)).toBe(false);
    expect(shouldCompletePaneSwipe(0.4, 390, 0)).toBe(true);
  });

  test("allows a quick flick but rejects accidental small movement", () => {
    expect(shouldCompletePaneSwipe(0.1, 390, 0.6)).toBe(true);
    expect(shouldCompletePaneSwipe(0.03, 390, 0.6)).toBe(false);
  });

  test("reversing direction cancels a swipe after crossing its threshold", () => {
    expect(shouldCompletePaneSwipe(0.55, 390, -0.5)).toBe(false);
  });

  test("settles for the remaining distance and respects reduced motion", () => {
    expect(paneSwipeDuration(0.8, true, false)).toBeLessThan(paneSwipeDuration(0.2, true, false));
    expect(paneSwipeDuration(0.8, false, false)).toBeGreaterThan(paneSwipeDuration(0.2, false, false));
    expect(paneSwipeDuration(0.5, true, true)).toBe(0);
    expect(paneSwipeDuration(0.5, false, true)).toBe(0);
  });
});
