-- +goose Up
ALTER TABLE collection_runs ADD COLUMN schedule_id TEXT REFERENCES collection_schedules(id) ON DELETE SET NULL;

CREATE TABLE users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL COLLATE NOCASE UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  csrf_token TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL
);
CREATE INDEX sessions_expiry_idx ON sessions(expires_at);

CREATE TABLE app_settings (
  key TEXT PRIMARY KEY,
  value_json TEXT NOT NULL CHECK (json_valid(value_json)),
  updated_at TEXT NOT NULL
);
INSERT INTO app_settings(key,value_json,updated_at) VALUES
  ('scheduler_paused','false',strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  ('default_concurrency','2',strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  ('evidence_retention_days','0',strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  ('evidence_max_bytes','1048576',strftime('%Y-%m-%dT%H:%M:%fZ','now'));

CREATE TABLE collection_schedules (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  collectors_json TEXT NOT NULL CHECK (json_valid(collectors_json)),
  interval_hours INTEGER NOT NULL CHECK (interval_hours >= 1),
  enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)),
  last_executed_at TEXT,
  next_execution_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX collection_schedules_due_idx ON collection_schedules(enabled,next_execution_at);

CREATE TABLE activity_events (
  id TEXT PRIMARY KEY,
  target_id TEXT REFERENCES targets(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  subject_type TEXT NOT NULL DEFAULT '',
  subject_id TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL,
  metadata_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(metadata_json)),
  created_at TEXT NOT NULL
);
CREATE INDEX activity_events_target_time_idx ON activity_events(target_id,created_at DESC);
CREATE INDEX activity_events_time_idx ON activity_events(created_at DESC);

CREATE TABLE notifications (
  id TEXT PRIMARY KEY,
  target_id TEXT REFERENCES targets(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  dedupe_key TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  message TEXT NOT NULL,
  link TEXT NOT NULL DEFAULT '',
  read_at TEXT,
  created_at TEXT NOT NULL
);
CREATE INDEX notifications_read_time_idx ON notifications(read_at,created_at DESC);

-- +goose Down
DROP TABLE notifications;
DROP TABLE activity_events;
DROP TABLE collection_schedules;
DROP TABLE app_settings;
DROP TABLE sessions;
DROP TABLE users;
