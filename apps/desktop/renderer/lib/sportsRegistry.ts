/** Registry of sports sections in the Sports mode sidebar. Add new leagues here. */

export type SportId = "mlb" | "nba" | "nfl" | "nhl";

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
    id: "nba",
    label: "Basketball",
    shortLabel: "NBA",
    available: false,
    comingSoonNote: "NBA support coming later",
  },
  {
    id: "nfl",
    label: "Football",
    shortLabel: "NFL",
    available: false,
    comingSoonNote: "NFL support coming later",
  },
  {
    id: "nhl",
    label: "Hockey",
    shortLabel: "NHL",
    available: false,
    comingSoonNote: "NHL support coming later",
  },
] as const;

export function sportById(id: SportId): SportDefinition {
  const found = SPORTS_REGISTRY.find((s) => s.id === id);
  if (!found) {
    const _exhaustive: never = id;
    return _exhaustive;
  }
  return found;
}
