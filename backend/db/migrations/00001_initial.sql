-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE targets (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL COLLATE NOCASE,
  primary_domain TEXT NOT NULL COLLATE NOCASE,
  platform TEXT NOT NULL DEFAULT '',
  program_url TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  program_rules TEXT NOT NULL DEFAULT '',
  program_restrictions TEXT NOT NULL DEFAULT '',
  rate_limit_per_minute INTEGER,
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  archived_at TEXT,
  UNIQUE(name)
);

CREATE TABLE scope_rules (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  rule_kind TEXT NOT NULL CHECK (rule_kind IN ('include', 'exclude')),
  pattern TEXT NOT NULL COLLATE NOCASE,
  pattern_kind TEXT NOT NULL CHECK (pattern_kind IN ('exact', 'wildcard')),
  permitted_methods_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(permitted_methods_json)),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(target_id, rule_kind, pattern)
);
CREATE INDEX scope_rules_target_idx ON scope_rules(target_id);

CREATE TABLE entities (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  type TEXT NOT NULL CHECK (type IN ('domain','subdomain','ip_address','dns_record','technology','url','service','certificate')),
  canonical_value TEXT NOT NULL,
  display_value TEXT NOT NULL,
  metadata_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(metadata_json)),
  scope_status TEXT NOT NULL DEFAULT 'unknown' CHECK (scope_status IN ('included','excluded','unknown')),
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(target_id, type, canonical_value),
  UNIQUE(target_id, id)
);
CREATE INDEX entities_target_seen_idx ON entities(target_id, last_seen_at DESC);

CREATE TABLE relationships (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  source_entity_id TEXT NOT NULL,
  destination_entity_id TEXT NOT NULL,
  relationship_type TEXT NOT NULL,
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'observed' CHECK (status IN ('observed','inferred','historical')),
  FOREIGN KEY(target_id, source_entity_id) REFERENCES entities(target_id, id) ON DELETE CASCADE,
  FOREIGN KEY(target_id, destination_entity_id) REFERENCES entities(target_id, id) ON DELETE CASCADE,
  UNIQUE(target_id, source_entity_id, destination_entity_id, relationship_type)
);
CREATE INDEX relationships_graph_idx ON relationships(target_id, source_entity_id, destination_entity_id);

CREATE TABLE evidence (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  evidence_type TEXT NOT NULL,
  source TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  content TEXT,
  storage_reference TEXT,
  content_size INTEGER NOT NULL CHECK (content_size >= 0),
  collected_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  CHECK (content IS NOT NULL OR storage_reference IS NOT NULL),
  UNIQUE(target_id, content_hash),
  UNIQUE(target_id, id)
);

CREATE TABLE collection_runs (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  mode TEXT NOT NULL CHECK (mode IN ('passive','active')),
  status TEXT NOT NULL CHECK (status IN ('pending','running','completed','failed','cancelled')),
  configuration_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(configuration_json)),
  error_summary TEXT NOT NULL DEFAULT '',
  started_at TEXT,
  completed_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(target_id, id)
);
CREATE INDEX collection_runs_target_idx ON collection_runs(target_id, created_at DESC);

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
CREATE INDEX observations_target_time_idx ON observations(target_id, observed_at DESC);

CREATE TABLE investigation_notes (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  body_markdown TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'unreviewed' CHECK (status IN ('unreviewed','reviewing','reviewed','needs_follow_up','archived')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX investigation_notes_target_idx ON investigation_notes(target_id, updated_at DESC);

-- +goose Down
DROP TABLE investigation_notes;
DROP TABLE observations;
DROP TABLE collection_runs;
DROP TABLE evidence;
DROP TABLE relationships;
DROP TABLE entities;
DROP TABLE scope_rules;
DROP TABLE targets;
