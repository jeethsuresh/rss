import { describe, expect, test } from "bun:test";
import { DestinationHistory } from "./destinationHistory";

describe("hosted Android destination history", () => {
  test("restores multiple destinations and discards an abandoned branch", () => {
    const h = new DestinationHistory("list");
    h.visit("article A"); h.visit("settings");
    expect(h.back()).toBe("article A");
    expect(h.back()).toBe("list");
    expect(h.forward()).toBe("article A");
    h.visit("article B");
    expect(h.forward()).toBeUndefined();
  });
  test("background data refreshes do not create navigation or destroy Forward", () => {
    const h = new DestinationHistory({ pane: "list", title: "old" });
    h.visit({ pane: "reader", title: "article" }); h.back();
    h.refresh({ pane: "list", title: "new" });
    expect(h.canBack).toBe(false);
    expect(h.forward()?.pane).toBe("reader");
  });
  test("bounds history and clears account destinations", () => {
    const h = new DestinationHistory(0, 3);
    for (let i = 1; i < 10; i++) h.visit(i);
    expect([h.back(), h.back(), h.back(), h.back()]).toEqual([8, 7, 6, undefined]);
    h.reset(100);
    expect(h.back()).toBeUndefined(); expect(h.forward()).toBeUndefined();
  });
});
