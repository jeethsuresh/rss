import { describe, expect, test } from "bun:test";
import { edgeSwipeDestination, type EdgeSwipe } from "./paneNavigation";

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
