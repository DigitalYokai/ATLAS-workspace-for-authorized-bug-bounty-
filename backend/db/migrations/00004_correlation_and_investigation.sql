-- +goose Up
PRAGMA foreign_keys = OFF;

ALTER TABLE entities ADD COLUMN investigation_status TEXT NOT NULL DEFAULT 'unreviewed' CHECK (investigation_status IN ('unreviewed','reviewing','reviewed','needs_follow_up','archived'));
ALTER TABLE entities ADD COLUMN bookmarked INTEGER NOT NULL DEFAULT 0 CHECK (bookmarked IN (0,1));
ALTER TABLE observations ADD COLUMN origin TEXT NOT NULL DEFAULT 'automated' CHECK (origin IN ('automated','manual'));

ALTER TABLE relationships RENAME TO relationships_v1;
CREATE TABLE relationships (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  source_entity_id TEXT NOT NULL,
  destination_entity_id TEXT NOT NULL,
  relationship_type TEXT NOT NULL CHECK (relationship_type IN ('HAS_SUBDOMAIN','RESOLVES_TO','HAS_DNS_RECORD','USES_TECHNOLOGY','PRESENTS_CERTIFICATE','HAS_ENDPOINT','HOSTS_SERVICE','REDIRECTS_TO','OBSERVED_IN')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'current' CHECK (status IN ('current','historical')),
  assertion_kind TEXT NOT NULL DEFAULT 'observed' CHECK (assertion_kind IN ('observed','inferred','manual','hypothesis')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(target_id, source_entity_id) REFERENCES entities(target_id, id) ON DELETE CASCADE,
  FOREIGN KEY(target_id, destination_entity_id) REFERENCES entities(target_id, id) ON DELETE CASCADE,
  UNIQUE(target_id, source_entity_id, destination_entity_id, relationship_type)
);
INSERT INTO relationships(id,target_id,source_entity_id,destination_entity_id,relationship_type,confidence,first_seen_at,last_seen_at,status,assertion_kind,created_at,updated_at)
SELECT id,target_id,source_entity_id,destination_entity_id,relationship_type,confidence,first_seen_at,last_seen_at,
  CASE WHEN status='historical' THEN 'historical' ELSE 'current' END,
  CASE WHEN status='inferred' THEN 'inferred' ELSE 'observed' END,
  first_seen_at,last_seen_at
FROM relationships_v1;
DROP TABLE relationships_v1;
CREATE INDEX relationships_graph_idx ON relationships(target_id, source_entity_id, destination_entity_id);
CREATE INDEX relationships_target_type_idx ON relationships(target_id, relationship_type, status);

CREATE TABLE relationship_evidence (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  relationship_id TEXT NOT NULL REFERENCES relationships(id) ON DELETE CASCADE,
  observation_id TEXT REFERENCES observations(id) ON DELETE SET NULL,
  evidence_id TEXT REFERENCES evidence(id) ON DELETE SET NULL,
  rationale TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  CHECK (observation_id IS NOT NULL OR evidence_id IS NOT NULL OR rationale <> ''),
  UNIQUE(relationship_id, observation_id)
);
CREATE INDEX relationship_evidence_relationship_idx ON relationship_evidence(relationship_id);

CREATE TABLE tags (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  name TEXT NOT NULL COLLATE NOCASE,
  color TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  UNIQUE(target_id, name)
);
CREATE TABLE entity_tags (
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  entity_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
  tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  PRIMARY KEY(entity_id, tag_id)
);
CREATE INDEX entity_tags_target_idx ON entity_tags(target_id, entity_id);

CREATE TABLE note_references (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  note_id TEXT NOT NULL REFERENCES investigation_notes(id) ON DELETE CASCADE,
  entity_id TEXT REFERENCES entities(id) ON DELETE CASCADE,
  observation_id TEXT REFERENCES observations(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  CHECK (entity_id IS NOT NULL OR observation_id IS NOT NULL)
);

CREATE TABLE correlation_jobs (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  collection_run_id TEXT REFERENCES collection_runs(id) ON DELETE CASCADE,
  status TEXT NOT NULL CHECK (status IN ('pending','running','completed','failed')),
  error_summary TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  started_at TEXT,
  completed_at TEXT,
  updated_at TEXT NOT NULL,
  UNIQUE(collection_run_id)
);
CREATE INDEX correlation_jobs_claim_idx ON correlation_jobs(status, created_at);

CREATE TABLE snapshots (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
  collection_run_id TEXT REFERENCES collection_runs(id) ON DELETE SET NULL,
  name TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL CHECK (kind IN ('automatic','manual')),
  created_at TEXT NOT NULL,
  UNIQUE(target_id, collection_run_id)
);
CREATE INDEX snapshots_target_idx ON snapshots(target_id, created_at DESC);
CREATE TABLE snapshot_entities (
  snapshot_id TEXT NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
  entity_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
  metadata_hash TEXT NOT NULL,
  PRIMARY KEY(snapshot_id, entity_id)
);
CREATE TABLE snapshot_relationships (
  snapshot_id TEXT NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
  relationship_id TEXT NOT NULL REFERENCES relationships(id) ON DELETE CASCADE,
  metadata_hash TEXT NOT NULL,
  PRIMARY KEY(snapshot_id, relationship_id)
);

CREATE VIRTUAL TABLE entity_search USING fts5(entity_id UNINDEXED, target_id UNINDEXED, canonical_value, display_value, type);
INSERT INTO entity_search(entity_id,target_id,canonical_value,display_value,type) SELECT id,target_id,canonical_value,display_value,type FROM entities;
-- +goose StatementBegin
CREATE TRIGGER entity_search_insert AFTER INSERT ON entities BEGIN
  INSERT INTO entity_search(entity_id,target_id,canonical_value,display_value,type) VALUES(new.id,new.target_id,new.canonical_value,new.display_value,new.type);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER entity_search_update AFTER UPDATE OF canonical_value,display_value,type ON entities BEGIN
  DELETE FROM entity_search WHERE entity_id=old.id;
  INSERT INTO entity_search(entity_id,target_id,canonical_value,display_value,type) VALUES(new.id,new.target_id,new.canonical_value,new.display_value,new.type);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER entity_search_delete AFTER DELETE ON entities BEGIN
  DELETE FROM entity_search WHERE entity_id=old.id;
END;
-- +goose StatementEnd

PRAGMA foreign_keys = ON;

-- +goose Down
DROP TRIGGER entity_search_delete;
DROP TRIGGER entity_search_update;
DROP TRIGGER entity_search_insert;
DROP TABLE entity_search;
DROP TABLE snapshot_relationships;
DROP TABLE snapshot_entities;
DROP TABLE snapshots;
DROP TABLE correlation_jobs;
DROP TABLE note_references;
DROP TABLE entity_tags;
DROP TABLE tags;
DROP TABLE relationship_evidence;
DROP TABLE relationships;
