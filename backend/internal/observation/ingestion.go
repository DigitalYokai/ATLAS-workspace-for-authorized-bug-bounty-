package observation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/atlas/atlas/backend/internal/scope"
	"github.com/google/uuid"
)

// Ingestor is the only path from collector output into persistent intelligence.
type Ingestor struct {
	db     *sql.DB
	scopes *scope.Repository
}
type Input struct {
	EntityType, Value, Source, Metadata, Evidence string
	Confidence                                    float64
	ObservedAt                                    time.Time
}

func NewIngestor(db *sql.DB, scopes *scope.Repository) *Ingestor { return &Ingestor{db, scopes} }
func canonical(kind, value string) (string, error) {
	v := strings.TrimSpace(value)
	if kind == "domain" || kind == "subdomain" {
		return scope.NormalizeHost(v)
	}
	if v == "" {
		return "", fmt.Errorf("empty entity value")
	}
	return strings.ToLower(v), nil
}
func (i *Ingestor) Ingest(ctx context.Context, targetID, runID string, o Input) error {
	if strings.TrimSpace(o.Source) == "" {
		return fmt.Errorf("observation source is required")
	}
	value, err := canonical(o.EntityType, o.Value)
	if err != nil {
		return err
	}
	rules, err := i.scopes.List(ctx, targetID)
	if err != nil {
		return err
	}
	status := "unknown"
	d := scope.Evaluate(rules, value, "passive_discovery")
	if d.Status == "included" {
		status = "included"
	}
	if d.Status == "excluded" {
		status = "excluded"
	}
	now := time.Now().UTC()
	if o.ObservedAt.IsZero() {
		o.ObservedAt = now
	}
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	entityID := uuid.NewString()
	_, err = tx.ExecContext(ctx, "INSERT INTO entities(id,target_id,type,canonical_value,display_value,metadata_json,scope_status,first_seen_at,last_seen_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(target_id,type,canonical_value) DO UPDATE SET last_seen_at=excluded.last_seen_at,updated_at=excluded.updated_at", entityID, targetID, o.EntityType, value, o.Value, o.Metadata, status, o.ObservedAt.Format(time.RFC3339Nano), o.ObservedAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("upsert entity: %w", err)
	}
	var evidenceID any = nil
	if o.Evidence != "" {
		h := sha256.Sum256([]byte(o.Evidence))
		hash := hex.EncodeToString(h[:])
		id := uuid.NewString()
		_, err = tx.ExecContext(ctx, "INSERT INTO evidence(id,target_id,evidence_type,source,content_hash,content,content_size,collected_at,created_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(target_id,content_hash) DO NOTHING", id, targetID, "collector_output", o.Source, hash, o.Evidence, len(o.Evidence), o.ObservedAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		var found string
		if err = tx.QueryRowContext(ctx, "SELECT id FROM evidence WHERE target_id=? AND content_hash=?", targetID, hash).Scan(&found); err != nil {
			return err
		}
		evidenceID = found
	}
	key := fmt.Sprintf("%s|%s|%s|%s", runID, o.Source, o.EntityType, value)
	_, err = tx.ExecContext(ctx, "INSERT INTO observations(id,target_id,collection_run_id,collector,observation_type,subject,value,source,evidence_id,observed_at,confidence,raw_metadata_json,idempotency_key,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(target_id,idempotency_key) DO NOTHING", uuid.NewString(), targetID, runID, o.Source, o.EntityType, value, o.Value, o.Source, evidenceID, o.ObservedAt.Format(time.RFC3339Nano), o.Confidence, o.Metadata, key, now.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}
