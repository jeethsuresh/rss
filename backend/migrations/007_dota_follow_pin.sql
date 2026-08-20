CREATE TABLE IF NOT EXISTS sports_dota_followed_teams (
  team_id INTEGER PRIMARY KEY NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sports_dota_pinned_events (
  event_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (event_id, event_type)
);
