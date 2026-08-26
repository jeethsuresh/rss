import { describe, expect, test } from "bun:test";
import { listRowScrollTop } from "./listScroll";

describe("listRowScrollTop", () => {
  test("positions a row below the sticky list toolbar", () => {
    expect(listRowScrollTop({
      containerScrollTop: 200,
      containerTop: 80,
      rowTop: 360,
      toolbarHeight: 56,
    })).toBe(424);
  });

  test("does not scroll before the beginning of the list", () => {
    expect(listRowScrollTop({
      containerScrollTop: 0,
      containerTop: 80,
      rowTop: 100,
      toolbarHeight: 56,
    })).toBe(0);
  });
});
