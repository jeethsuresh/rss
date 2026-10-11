import { describe, expect, test } from "bun:test";
import { ReaderEndGesture, readerPullProgress } from "./readerEndGesture";

describe("deliberate scrolling past an article", () => {
  test("arriving at the bottom and the rest of a fling do not finish it", () => {
    const gesture = new ReaderEndGesture();
    expect(gesture.wheel(500, false, 0)).toBe(0);
    for (let time = 16; time < 500; time += 16) {
      expect(gesture.wheel(100, true, time)).toBe(0);
    }
    expect(gesture.wheel(100, true, 800)).toBeGreaterThan(0);
  });

  test("a single large wheel event cannot finish; a sustained fresh push can", () => {
    const gesture = new ReaderEndGesture();
    expect(gesture.wheel(5000, true, 0)).toBeLessThan(1);
    for (let i = 1; i < 5; i++) expect(gesture.wheel(100, true, i * 30)).toBeLessThan(1);
    expect(gesture.wheel(100, true, 150)).toBe(1);
  });

  test("scrolling away and pausing discard partial progress", () => {
    const gesture = new ReaderEndGesture();
    const first = gesture.wheel(80, true, 0);
    expect(gesture.wheel(80, true, 30)).toBeGreaterThan(first);
    expect(gesture.wheel(80, true, 400)).toBe(first);
    expect(gesture.wheel(-80, true, 430)).toBe(0);
    expect(gesture.wheel(80, true, 450)).toBe(0);
    expect(gesture.wheel(80, false, 800)).toBe(0);
  });

  test("an upward pull needs substantial travel and horizontal/back gestures do not qualify", () => {
    expect(readerPullProgress(200, 700, 200, 670)).toBeLessThan(1);
    expect(readerPullProgress(200, 700, 200, 520)).toBe(1);
    expect(readerPullProgress(200, 700, 200, 720)).toBe(0);
    expect(readerPullProgress(8, 700, 180, 680)).toBe(0);
    expect(readerPullProgress(200, 700, 200, 690)).toBeLessThan(readerPullProgress(200, 700, 200, 600));
  });
});
