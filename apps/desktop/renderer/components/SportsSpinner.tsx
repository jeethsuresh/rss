/** Tiny inline spinner for Sports refresh indicators. */
export function SportsSpinner({ label = "Refreshing" }: { label?: string }) {
  return (
    <span className="sports-spinner" role="status" aria-live="polite" title={label}>
      <span className="sports-spinner-dot" aria-hidden />
      <span className="sports-spinner-label">{label}</span>
    </span>
  );
}

/** Full pane placeholder while a new selection is loading (never reuse prior item UI). */
export function SportsLoadingPane({ label = "Loading…" }: { label?: string }) {
  return (
    <div className="empty">
      <SportsSpinner label={label} />
    </div>
  );
}
