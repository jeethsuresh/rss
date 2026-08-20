-- Dota provider toggle + provider-scoped follows/pins
ALTER TABLE settings ADD COLUMN dota_provider TEXT NOT NULL DEFAULT 'pandascore';

CREATE TABLE IF NOT EXISTS sports_dota_followed_teams_v2 (
  provider TEXT NOT NULL,
  team_id INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (provider, team_id)
);

INSERT OR IGNORE INTO sports_dota_followed_teams_v2 (provider, team_id, created_at)
SELECT 'pandascore', team_id, created_at FROM sports_dota_followed_teams;

DROP TABLE IF EXISTS sports_dota_followed_teams;
ALTER TABLE sports_dota_followed_teams_v2 RENAME TO sports_dota_followed_teams;

CREATE TABLE IF NOT EXISTS sports_dota_pinned_events_v2 (
  provider TEXT NOT NULL,
  event_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (provider, event_id, event_type)
);

INSERT OR IGNORE INTO sports_dota_pinned_events_v2 (provider, event_id, event_type, created_at)
SELECT 'pandascore', event_id, event_type, created_at FROM sports_dota_pinned_events;

DROP TABLE IF EXISTS sports_dota_pinned_events;
ALTER TABLE sports_dota_pinned_events_v2 RENAME TO sports_dota_pinned_events;
