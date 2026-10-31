// A separate, deliberate push after reaching the bottom prevents scroll momentum
// from finishing an article. Cap each wheel event so a single fling cannot do it.
export const READER_WHEEL_PAUSE_MS = 220;
export const READER_WHEEL_DISTANCE = 480;
export const READER_PULL_DISTANCE = 180;

export class ReaderEndGesture {
  private lastWheelAt: number | null = null;
  private wheelReady = false;
  private distance = 0;

  wheel(delta: number, atEnd: boolean, now: number): number {
    const fresh = this.lastWheelAt === null || now - this.lastWheelAt > READER_WHEEL_PAUSE_MS;
    this.lastWheelAt = now;
    if (!atEnd || delta <= 0) {
      this.wheelReady = false;
      this.distance = 0;
      return 0;
    }
    if (fresh) {
      this.wheelReady = true;
      this.distance = 0;
    }
    if (!this.wheelReady) return 0;
    this.distance += Math.min(80, delta);
    return Math.min(1, this.distance / READER_WHEEL_DISTANCE);
  }
}

export function readerPullProgress(startX: number, startY: number, x: number, y: number): number {
  const upward = startY - y;
  if (upward <= 0 || Math.abs(x - startX) > upward) return 0;
  return Math.min(1, upward / READER_PULL_DISTANCE);
}
