/** Registry of sports sections in the Sports mode sidebar. Add new leagues here. */

export type SportId = "mlb" | "nfl" | "f1";

export type SportDefinition = {
  id: SportId;
  /** Sidebar section title, e.g. "Baseball" */
  label: string;
  /** Short label for settings / manage CTA */
  shortLabel: string;
  /** Whether this sport has a live backend integration */
  available: boolean;
  /** Optional blurb when unavailable */
  comingSoonNote?: string;
};

/**
 * Ordered list of sports shown in the Sports sidebar.
 * Wire a new Go client + IPC + view panel when flipping `available` to true.
 */
export const SPORTS_REGISTRY: readonly SportDefinition[] = [
  {
    id: "mlb",
    label: "Baseball",
    shortLabel: "MLB",
    available: true,
  },
  {
    id: "nfl",
    label: "Football",
    shortLabel: "NFL",
    available: false,
    comingSoonNote: "NFL support coming later",
  },
  {
    id: "f1",
    label: "F1",
    shortLabel: "F1",
    available: true,
  },
] as const;

export function sportById(id: SportId): SportDefinition {
  const found = SPORTS_REGISTRY.find((s) => s.id === id);
  if (!found) {
    throw new Error(`Unknown sport: ${id}`);
  }
  return found;
}
