package correlation

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Snapshot struct {
	ID              string    `json:"id"`
	TargetID        string    `json:"targetId"`
	CollectionRunID *string   `json:"collectionRunId,omitempty"`
	Name            string    `json:"name"`
	Kind            string    `json:"kind"`
	CreatedAt       time.Time `json:"createdAt"`
}

type Comparison struct {
	FromID                 string   `json:"fromSnapshotId"`
	ToID                   string   `json:"toSnapshotId"`
	NewEntityIDs           []string `json:"newEntityIds"`
	MissingEntityIDs       []string `json:"missingEntityIds"`
	ModifiedEntityIDs      []string `json:"modifiedEntityIds"`
	NewRelationshipIDs     []string `json:"newRelationshipIds"`
	MissingRelationshipIDs []string `json:"missingRelationshipIds"`
}

func (s *Service) CreateAutomaticSnapshot(ctx context.Context, targetID, runID string) error {
	return s.createSnapshot(ctx, targetID, &runID, "", "automatic")
}

func (s *Service) CreateManualSnapshot(ctx context.Context, targetID, name string) (Snapshot, error) {
	if name == "" {
		name = "Manual baseline"
	}
	id, err := s.createSnapshotReturningID(ctx, targetID, nil, name, "manual")
	if err != nil {
		return Snapshot{}, err
	}
	return s.GetSnapshot(ctx, targetID, id)
}

func (s *Service) createSnapshot(ctx context.Context, targetID string, runID *string, name, kind string) error {
	_, err := s.createSnapshotReturningID(ctx, targetID, runID, name, kind)
	return err
}

func (s *Service) createSnapshotReturningID(ctx context.Context, targetID string, runID *string, name, kind string) (string, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, "INSERT INTO snapshots(id,target_id,collection_run_id,name,kind,created_at) VALUES(?,?,?,?,?,?)", id, targetID, runID, name, kind, now.Format(time.RFC3339Nano))
	if err != nil {
		return "", err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,metadata_json,scope_status,investigation_status,bookmarked FROM entities WHERE target_id=?", targetID)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var entityID, metadata, scopeStatus, investigationStatus string
		var bookmarked int
		if err = rows.Scan(&entityID, &metadata, &scopeStatus, &investigationStatus, &bookmarked); err != nil {
			rows.Close()
			return "", err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO snapshot_entities(snapshot_id,entity_id,metadata_hash) VALUES(?,?,?)", id, entityID, hash(metadata, scopeStatus, investigationStatus, fmt.Sprint(bookmarked))); err != nil {
			rows.Close()
			return "", err
		}
	}
	if err = rows.Close(); err != nil {
		return "", err
	}
	rows, err = tx.QueryContext(ctx, "SELECT id,relationship_type,confidence,status,assertion_kind FROM relationships WHERE target_id=?", targetID)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var relationshipID, typ, status, assertion string
		var confidence float64
		if err = rows.Scan(&relationshipID, &typ, &confidence, &status, &assertion); err != nil {
			rows.Close()
			return "", err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO snapshot_relationships(snapshot_id,relationship_id,metadata_hash) VALUES(?,?,?)", id, relationshipID, hash(typ, fmt.Sprint(confidence), status, assertion)); err != nil {
			rows.Close()
			return "", err
		}
	}
	if err = rows.Close(); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	if kind == "automatic" {
		_ = s.markHistorical(ctx, targetID)
	}
	return id, nil
}

func (s *Service) markHistorical(ctx context.Context, targetID string) error {
	// Only two latest automatic snapshots participate in lifecycle changes.
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM snapshots WHERE target_id=? AND kind='automatic' ORDER BY created_at DESC LIMIT 2", targetID)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Close(); err != nil || len(ids) < 2 {
		return err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE relationships SET status='historical',updated_at=? WHERE target_id=? AND status='current' AND id NOT IN (SELECT relationship_id FROM snapshot_relationships WHERE snapshot_id=?) AND id NOT IN (SELECT relationship_id FROM snapshot_relationships WHERE snapshot_id=?)", time.Now().UTC().Format(time.RFC3339Nano), targetID, ids[0], ids[1])
	return err
}

func (s *Service) ListSnapshots(ctx context.Context, targetID string) ([]Snapshot, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,target_id,collection_run_id,name,kind,created_at FROM snapshots WHERE target_id=? ORDER BY created_at DESC", targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Snapshot{}
	for rows.Next() {
		var x Snapshot
		var run sql.NullString
		var created string
		if err = rows.Scan(&x.ID, &x.TargetID, &run, &x.Name, &x.Kind, &created); err != nil {
			return nil, err
		}
		if run.Valid {
			x.CollectionRunID = &run.String
		}
		x.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) GetSnapshot(ctx context.Context, targetID, id string) (Snapshot, error) {
	var x Snapshot
	var run sql.NullString
	var created string
	err := s.db.QueryRowContext(ctx, "SELECT id,target_id,collection_run_id,name,kind,created_at FROM snapshots WHERE target_id=? AND id=?", targetID, id).Scan(&x.ID, &x.TargetID, &run, &x.Name, &x.Kind, &created)
	if run.Valid {
		x.CollectionRunID = &run.String
	}
	x.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return x, err
}

func (s *Service) CompareSnapshots(ctx context.Context, targetID, fromID, toID string) (Comparison, error) {
	comparison := Comparison{FromID: fromID, ToID: toID, NewEntityIDs: []string{}, MissingEntityIDs: []string{}, ModifiedEntityIDs: []string{}, NewRelationshipIDs: []string{}, MissingRelationshipIDs: []string{}}
	for _, id := range []string{fromID, toID} {
		if _, err := s.GetSnapshot(ctx, targetID, id); err != nil {
			return comparison, err
		}
	}
	newIDs, err := s.diffIDs(ctx, "snapshot_entities", "entity_id", fromID, toID, true)
	if err != nil {
		return comparison, err
	}
	comparison.NewEntityIDs = newIDs
	missingIDs, err := s.diffIDs(ctx, "snapshot_entities", "entity_id", fromID, toID, false)
	if err != nil {
		return comparison, err
	}
	comparison.MissingEntityIDs = missingIDs
	rows, err := s.db.QueryContext(ctx, "SELECT t.entity_id FROM snapshot_entities t JOIN snapshot_entities f ON f.entity_id=t.entity_id WHERE t.snapshot_id=? AND f.snapshot_id=? AND t.metadata_hash<>f.metadata_hash", toID, fromID)
	if err != nil {
		return comparison, err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return comparison, err
		}
		comparison.ModifiedEntityIDs = append(comparison.ModifiedEntityIDs, id)
	}
	_ = rows.Close()
	comparison.NewRelationshipIDs, err = s.diffIDs(ctx, "snapshot_relationships", "relationship_id", fromID, toID, true)
	if err != nil {
		return comparison, err
	}
	comparison.MissingRelationshipIDs, err = s.diffIDs(ctx, "snapshot_relationships", "relationship_id", fromID, toID, false)
	return comparison, err
}

func (s *Service) diffIDs(ctx context.Context, table, column, fromID, toID string, added bool) ([]string, error) {
	left, right := fromID, toID
	if !added {
		left, right = toID, fromID
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE snapshot_id=? AND %s NOT IN (SELECT %s FROM %s WHERE snapshot_id=?)", column, table, column, column, table)
	rows, err := s.db.QueryContext(ctx, query, right, left)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
