/** Tiny inline spinner for Sports refresh indicators. */
export function SportsSpinner({ label = "Refreshing" }: { label?: string }) {
  return (
    <span className="sports-spinner" role="status" aria-live="polite" title={label}>
      <span className="sports-spinner-dot" aria-hidden />
      <span className="sports-spinner-label">{label}</span>
    </span>
  );
}
