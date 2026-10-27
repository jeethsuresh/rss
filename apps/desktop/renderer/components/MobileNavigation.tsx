export type AppDestination = "rss" | "readLater" | "sports" | "settings";

const destinations: { id: AppDestination; label: string; path: string }[] = [
  { id: "rss", label: "RSS", path: "M5 11a8 8 0 0 1 8 8M5 5a14 14 0 0 1 14 14M6 19h.01" },
  { id: "readLater", label: "Read Later", path: "M6 4h12v17l-6-4-6 4V4Z" },
  { id: "sports", label: "Sports", path: "M8 3h8v6a4 4 0 0 1-8 0V3ZM8 5H4v2a4 4 0 0 0 4 4m8-6h4v2a4 4 0 0 1-4 4m-4 2v5m-4 3h8m-4-3-3 3m3-3 3 3" },
  { id: "settings", label: "Settings", path: "M9 4h6l1 3 3 1v6l-3 1-1 3H9l-1-3-3-1V8l3-1 1-3Zm6 7a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" },
];

export function MobileNavigation({ active, onNavigate }: {
  active: AppDestination;
  onNavigate: (destination: AppDestination) => void;
}) {
  return (
    <nav className="mobile-bottom-navigation" aria-label="Main navigation">
      {destinations.map(({ id, label, path }) => (
        <button key={id} type="button" aria-current={active === id ? "page" : undefined} onClick={() => onNavigate(id)}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d={path} />
          </svg>
          <span>{label}</span>
        </button>
      ))}
    </nav>
  );
}
