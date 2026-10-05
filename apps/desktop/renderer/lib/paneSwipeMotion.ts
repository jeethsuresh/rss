import { paneSwipeDuration, type MobilePane } from "./paneNavigation";

type ScrollPosition = { element: HTMLElement; top: number; left: number };
export type PaneScene = { element: HTMLElement; scroll: ScrollPosition[] };

// The incoming pane is an inert visual copy. Previewing a gesture never opens an
// article, changes its read state, starts a sports watch, or mounts another React tree.
// The outgoing pane remains live, including embedded pages and media.
export function capturePaneScene(root: HTMLElement, pane: "list" | "detail"): PaneScene {
  const element = root.cloneNode(true) as HTMLElement;
  const originals = [root, ...root.querySelectorAll<HTMLElement>("*")];
  const copies = [element, ...element.querySelectorAll<HTMLElement>("*")];
  const scroll: ScrollPosition[] = [];
  originals.forEach((original, index) => {
    const copy = copies[index];
    copy.removeAttribute("id");
    copy.removeAttribute("autofocus");
    if (original.scrollTop || original.scrollLeft) {
      scroll.push({ element: copy, top: original.scrollTop, left: original.scrollLeft });
    }
    if (original instanceof HTMLInputElement && copy instanceof HTMLInputElement) {
      copy.value = original.value;
      copy.checked = original.checked;
    } else if (original instanceof HTMLTextAreaElement && copy instanceof HTMLTextAreaElement) {
      copy.value = original.value;
    } else if (original instanceof HTMLSelectElement && copy instanceof HTMLSelectElement) {
      copy.selectedIndex = original.selectedIndex;
    }
    if (copy instanceof HTMLMediaElement) copy.autoplay = false;
  });
  element.querySelectorAll(".sidebar, .sidebar-scrim").forEach((node) => node.remove());
  element.classList.remove("sidebar-open", "pane-swipe-live", "pane-swipe-drawer");
  element.classList.toggle("mobile-detail", pane === "detail");
  element.classList.toggle("mobile-list", pane === "list");
  element.classList.add("pane-swipe-scene");
  element.setAttribute("aria-hidden", "true");
  element.inert = true;
  return { element, scroll };
}

export class PaneSwipeMotion {
  readonly travel: number;
  private readonly drawer: HTMLElement | null;
  private readonly layer: HTMLElement | null;
  private readonly rootStyle: string | null;
  private readonly reducedMotion: boolean;
  private readonly parallax: number;
  private progress = 0;
  private timer: number | undefined;
  private frame: number | undefined;
  private disposed = false;

  constructor(private readonly root: HTMLElement, private readonly from: MobilePane,
    private readonly to: MobilePane, incoming?: PaneScene) {
    const bounds = root.getBoundingClientRect();
    this.rootStyle = root.getAttribute("style");
    this.reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    this.parallax = this.reducedMotion ? 0 : 0.26;
    this.drawer = from === "sidebar" || to === "sidebar" ? root.querySelector<HTMLElement>(".sidebar") : null;
    this.travel = this.drawer?.getBoundingClientRect().width ?? bounds.width;
    root.style.setProperty("--pane-swipe-duration", "0ms");
    if (this.drawer) {
      this.layer = null;
      root.classList.add("pane-swipe-drawer");
    } else {
      if (!incoming) throw new Error("A pane preview is required");
      const layer = document.createElement("div");
      layer.className = "pane-swipe-layer";
      layer.setAttribute("aria-hidden", "true");
      layer.inert = true;
      Object.assign(layer.style, {
        top: `${bounds.top}px`, left: `${bounds.left}px`,
        width: `${bounds.width}px`, height: `${bounds.height}px`,
      });
      layer.append(incoming.element);
      root.parentElement?.append(layer);
      for (const saved of incoming.scroll) {
        saved.element.scrollTo({ top: saved.top, left: saved.left, behavior: "auto" });
      }
      this.layer = layer;
      root.classList.add("pane-swipe-live");
      root.style.zIndex = from === "detail" ? "25" : "23";
    }
    this.update(0);
  }

  update(progress: number) {
    if (this.disposed) return;
    this.progress = progress;
    if (this.drawer) {
      const reveal = this.to === "sidebar" ? progress : 1 - progress;
      this.root.style.setProperty("--pane-sidebar-x", `${(reveal - 1) * this.travel}px`);
      this.root.style.setProperty("--pane-sidebar-reveal", `${reveal}`);
      return;
    }
    const back = this.from === "detail";
    const currentX = back ? this.travel * progress : -this.travel * this.parallax * progress;
    const incomingX = back ? -this.travel * this.parallax * (1 - progress) : this.travel * (1 - progress);
    this.root.style.transform = `translate3d(${currentX}px, 0, 0)`;
    this.layer!.style.transform = `translate3d(${incomingX}px, 0, 0)`;
    this.root.style.setProperty("--pane-swipe-dim", back ? "0" : `${0.14 * progress}`);
    this.layer!.style.setProperty("--pane-swipe-dim", back ? `${0.14 * (1 - progress)}` : "0");
    const foreground = back ? this.root : this.layer!;
    foreground.style.boxShadow = "-10px 0 28px rgb(0 0 0 / 20%)";
  }

  settle(complete: boolean, finished: () => void) {
    if (this.disposed) return;
    const duration = paneSwipeDuration(this.progress, complete, this.reducedMotion);
    const transition = `transform ${duration}ms cubic-bezier(0.22, 0.8, 0.25, 1)`;
    // Flush the last finger position before starting the settling transition.
    this.root.getBoundingClientRect();
    this.layer?.getBoundingClientRect();
    this.root.style.setProperty("--pane-swipe-duration", `${duration}ms`);
    if (this.layer) {
      this.root.style.transition = transition;
      this.layer.style.transition = transition;
      this.layer.style.setProperty("--pane-swipe-duration", `${duration}ms`);
    }
    this.frame = window.requestAnimationFrame(() => {
      this.update(complete ? 1 : 0);
      this.timer = window.setTimeout(finished, duration ? duration + 24 : 0);
    });
  }

  finishEntryAnimations() {
    // Revealing a previously hidden reader restarts its CSS entrance fade.
    // Finish that fade before handing off the already visible gesture preview.
    this.root.getBoundingClientRect();
    for (const animation of this.root.getAnimations({ subtree: true })) {
      if (animation instanceof CSSAnimation && animation.effect?.getTiming().iterations === 1) {
        animation.finish();
      }
    }
  }

  dispose() {
    if (this.disposed) return;
    this.disposed = true;
    window.clearTimeout(this.timer);
    if (this.frame != null) window.cancelAnimationFrame(this.frame);
    this.layer?.remove();
    this.root.classList.remove("pane-swipe-live", "pane-swipe-drawer");
    if (this.rootStyle == null) this.root.removeAttribute("style");
    else this.root.setAttribute("style", this.rootStyle);
  }
}
