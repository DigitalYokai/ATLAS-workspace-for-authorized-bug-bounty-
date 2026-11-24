-- +goose Up
PRAGMA foreign_keys = OFF;
ALTER TABLE collection_runs RENAME TO collection_runs_v1;
CREATE TABLE collection_runs (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  mode TEXT NOT NULL CHECK (mode IN ('passive','active')),
  status TEXT NOT NULL CHECK (status IN ('pending','running','completed','partially_completed','failed','cancelled')),
  configuration_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(configuration_json)),
  error_summary TEXT NOT NULL DEFAULT '',
  total_jobs INTEGER NOT NULL DEFAULT 0,
  completed_jobs INTEGER NOT NULL DEFAULT 0,
  failed_jobs INTEGER NOT NULL DEFAULT 0,
  started_at TEXT,
  completed_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(target_id, id)
);
INSERT INTO collection_runs (id,target_id,mode,status,configuration_json,error_summary,started_at,completed_at,created_at,updated_at)
SELECT id,target_id,mode,status,configuration_json,error_summary,started_at,completed_at,created_at,updated_at FROM collection_runs_v1;
DROP TABLE collection_runs_v1;
CREATE INDEX collection_runs_target_idx ON collection_runs(target_id, created_at DESC);

CREATE TABLE collection_jobs (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL,
  run_id TEXT NOT NULL,
  collector_name TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('pending','running','completed','failed','cancelled')),
  error_summary TEXT NOT NULL DEFAULT '',
  started_at TEXT,
  completed_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(target_id, run_id) REFERENCES collection_runs(target_id, id) ON DELETE CASCADE,
  UNIQUE(run_id, collector_name)
);
CREATE INDEX collection_jobs_claim_idx ON collection_jobs(status, created_at);

CREATE TABLE collection_events (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  run_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  message TEXT NOT NULL,
  payload_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(payload_json)),
  created_at TEXT NOT NULL,
  FOREIGN KEY(target_id, run_id) REFERENCES collection_runs(target_id, id) ON DELETE CASCADE
);
CREATE INDEX collection_events_run_idx ON collection_events(run_id, created_at);
PRAGMA foreign_keys = ON;

-- +goose Down
DROP TABLE collection_events;
DROP TABLE collection_jobs;
DROP TABLE collection_runs;
