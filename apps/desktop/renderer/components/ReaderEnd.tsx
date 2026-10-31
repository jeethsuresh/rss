import { useEffect, useRef, useState } from "react";
import { ReaderEndGesture, readerPullProgress } from "../lib/readerEndGesture";

type Props = { articleId: string; onFinish: () => Promise<boolean> };

export function ReaderEnd({ articleId, onFinish }: Props) {
  const footer = useRef<HTMLDivElement>(null);
  const finishRef = useRef(onFinish);
  finishRef.current = onFinish;
  const finishAction = useRef<() => void>(() => undefined);
  const wheelBlockedUntil = useRef(0);
  const [progress, setProgress] = useState(0);
  const [status, setStatus] = useState<"idle" | "busy" | "finished" | "error">("idle");

  useEffect(() => {
    const reader = footer.current?.closest<HTMLElement>(".reader");
    if (!reader) return;
    reader.scrollTop = 0;
    setProgress(0);
    setStatus("idle");
    let disposed = false;
    let busy = false;
    let completed = false;
    let gesture = new ReaderEndGesture();
    let resetTimer: ReturnType<typeof setTimeout> | undefined;
    let touch: { id: number; x: number; y: number; ready: boolean; progress: number } | null = null;
    const atEnd = () => reader.scrollHeight - reader.clientHeight - Math.max(0, reader.scrollTop) <= 3;
    const reset = () => { setProgress(0); touch = null; };
    const finish = async () => {
      if (busy || completed) return;
      busy = true;
      clearTimeout(resetTimer);
      wheelBlockedUntil.current = performance.now() + 700;
      setProgress(1);
      setStatus("busy");
      try {
        const endOfList = await finishRef.current();
        completed = endOfList;
        if (!disposed) { setProgress(0); setStatus(endOfList ? "finished" : "idle"); }
      } catch {
        if (!disposed) { setProgress(0); setStatus("error"); }
        gesture = new ReaderEndGesture();
      } finally { busy = false; }
    };
    finishAction.current = () => { void finish(); };
    const wheel = (event: WheelEvent) => {
      if (busy || completed || event.defaultPrevented || event.ctrlKey || event.metaKey
        || Math.abs(event.deltaX) >= Math.abs(event.deltaY)) return;
      const now = performance.now();
      if (now < wheelBlockedUntil.current) {
        wheelBlockedUntil.current = Math.max(wheelBlockedUntil.current, now + 220);
        return;
      }
      const delta = event.deltaY * (event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? reader.clientHeight : 1);
      const end = atEnd();
      const value = gesture.wheel(delta, end, now);
      clearTimeout(resetTimer);
      setProgress(value);
      if (end && delta > 0 && event.cancelable) event.preventDefault();
      if (value >= 1) void finish();
      else resetTimer = setTimeout(() => { setProgress(0); }, 600);
    };
    const start = (event: TouchEvent) => {
      clearTimeout(resetTimer);
      if (busy || completed || event.touches.length !== 1
        || (event.target as Element).closest("button, a, input, select, textarea, [contenteditable='true']")) {
        reset(); return;
      }
      const point = event.touches[0];
      touch = { id: point.identifier, x: point.clientX, y: point.clientY, ready: atEnd(), progress: 0 };
    };
    const move = (event: TouchEvent) => {
      if (!touch || !touch.ready || event.touches.length !== 1 || event.defaultPrevented) return;
      const point = Array.from(event.touches).find((item) => item.identifier === touch!.id);
      if (!point) { reset(); return; }
      const dx = Math.abs(point.clientX - touch.x);
      const dy = touch.y - point.clientY;
      if (dx > 10 && dx > Math.abs(dy)) { reset(); return; }
      if (!atEnd() || dy < -8) { reset(); return; }
      touch.progress = readerPullProgress(touch.x, touch.y, point.clientX, point.clientY);
      if (dy > 8 && event.cancelable) event.preventDefault();
      setProgress(touch.progress);
    };
    const end = () => {
      const value = touch?.progress ?? 0;
      reset();
      if (value >= 1) void finish();
    };
    const scroll = () => { if (!atEnd() && !busy) reset(); };
    reader.addEventListener("wheel", wheel, { passive: false });
    reader.addEventListener("touchstart", start, { passive: true });
    reader.addEventListener("touchmove", move, { passive: false });
    reader.addEventListener("touchend", end);
    reader.addEventListener("touchcancel", reset);
    reader.addEventListener("scroll", scroll, { passive: true });
    return () => {
      disposed = true;
      clearTimeout(resetTimer);
      finishAction.current = () => undefined;
      reader.removeEventListener("wheel", wheel);
      reader.removeEventListener("touchstart", start);
      reader.removeEventListener("touchmove", move);
      reader.removeEventListener("touchend", end);
      reader.removeEventListener("touchcancel", reset);
      reader.removeEventListener("scroll", scroll);
    };
  }, [articleId]);

  const message = status === "busy" ? "Marking read…"
    : status === "finished" ? "End of list"
    : status === "error" ? "Couldn’t continue. Try again."
    : progress >= 1 ? "Release to continue"
    : progress > 0 ? "Keep going to continue"
    : "End of article";
  return (
    <div ref={footer} className={`reader-end ${progress > 0 ? "reader-end-pulling" : ""}`}
      style={{ transform: `translateY(${-18 * Math.sqrt(progress)}px)` }}>
      <span className="reader-end-arrow" aria-hidden="true">↑</span>
      <strong role="status">{message}</strong>
      <span className="muted">{status === "idle" ? "Scroll firmly or pull up to mark read and continue" : status === "finished" ? "Article marked read" : ""}</span>
      <div className="reader-end-track" aria-hidden="true"><span style={{ transform: `scaleX(${progress})` }} /></div>
      <button type="button" className="reader-end-next" disabled={status === "busy" || status === "finished"}
        onClick={() => finishAction.current()}>Mark read and continue</button>
    </div>
  );
}
