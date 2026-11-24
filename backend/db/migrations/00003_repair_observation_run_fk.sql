-- +goose Up
PRAGMA foreign_keys = OFF;
ALTER TABLE observations RENAME TO observations_v2;
CREATE TABLE observations (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  collection_run_id TEXT,
  collector TEXT NOT NULL,
  observation_type TEXT NOT NULL,
  subject TEXT NOT NULL,
  value TEXT NOT NULL,
  source TEXT NOT NULL,
  evidence_id TEXT,
  observed_at TEXT NOT NULL,
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  raw_metadata_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(raw_metadata_json)),
  idempotency_key TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY(target_id, collection_run_id) REFERENCES collection_runs(target_id, id) ON DELETE SET NULL,
  FOREIGN KEY(target_id, evidence_id) REFERENCES evidence(target_id, id) ON DELETE SET NULL,
  UNIQUE(target_id, idempotency_key)
);
INSERT INTO observations SELECT * FROM observations_v2;
DROP TABLE observations_v2;
CREATE INDEX observations_target_time_idx ON observations(target_id, observed_at DESC);
PRAGMA foreign_keys = ON;

-- +goose Down
DROP TABLE observations;
