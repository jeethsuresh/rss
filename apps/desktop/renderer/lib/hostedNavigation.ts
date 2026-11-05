import { useEffect, useRef, type MutableRefObject } from "react";
import { DestinationHistory } from "./destinationHistory";

/** The Android shell dispatches this cancelable event before native WebView history. */
export function useHostedNavigation<T>(
  destination: T,
  restore: (value: T) => void,
  enabled: boolean,
  restoring: MutableRefObject<boolean>,
  interactionRevision: MutableRefObject<number>,
) {
  const history = useRef(new DestinationHistory(destination));
  const restoreRef = useRef(restore);
  restoreRef.current = restore;
  const interaction = useRef(false);
  const serialized = JSON.stringify(destination);
  useEffect(() => {
    if (!enabled) { history.current.reset(destination); return; }
    if (restoring.current) {
      history.current.refresh(destination);
      restoring.current = false;
    } else if (interaction.current) {
      interaction.current = false;
      history.current.visit(destination);
    } else history.current.refresh(destination);
  }, [serialized, enabled]);

  useEffect(() => {
    if (!enabled) return;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const begin = (event: Event) => {
      const target = event.target as HTMLElement | null;
      if (event instanceof KeyboardEvent && (target?.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target?.tagName ?? ""))) return;
      interaction.current = true;
      interactionRevision.current++;
      clearTimeout(timer);
      timer = setTimeout(() => { interaction.current = false; }, 250);
    };
    const navigate = (event: Event) => {
      if (event.defaultPrevented) return;
      const direction = (event as CustomEvent).detail?.direction;
      if (direction !== "back" && direction !== "forward") return;
      const next = direction === "back" ? history.current.back() : history.current.forward();
      if (next !== undefined) {
        event.preventDefault();
        interaction.current = false;
        interactionRevision.current++;
        restoring.current = true;
        restoreRef.current(next);
      } else if (direction === "forward") event.preventDefault();
    };
    const state = (event: Event) => {
      const value = (event as CustomEvent).detail;
      if (value && typeof value === "object") {
        value.back = history.current.canBack;
        value.forward = history.current.canForward;
      }
    };
    document.addEventListener("click", begin, true);
    document.addEventListener("pointerup", begin, true);
    document.addEventListener("keydown", begin, true);
    window.addEventListener("gravitas:navigate", navigate);
    window.addEventListener("gravitas:navigation-state", state);
    return () => {
      clearTimeout(timer);
      document.removeEventListener("click", begin, true);
      document.removeEventListener("pointerup", begin, true);
      document.removeEventListener("keydown", begin, true);
      window.removeEventListener("gravitas:navigate", navigate);
      window.removeEventListener("gravitas:navigation-state", state);
    };
  }, [enabled]);
}
