package correlation

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/atlas/atlas/backend/internal/observation"
	"github.com/atlas/atlas/backend/internal/platform/database"
	"github.com/atlas/atlas/backend/internal/scope"
	"github.com/google/uuid"
)

func setupCorrelationDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "atlas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.Exec("INSERT INTO targets(id,name,primary_domain,status,created_at,updated_at) VALUES(?,?,?,?,?,?)", "target", uuid.NewString(), "example.com", "active", now, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO collection_runs(id,target_id,mode,status,configuration_json,total_jobs,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)", "run", "target", "passive", "completed", `{}`, 1, now, now)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCorrelateRunCreatesEvidenceBackedRelationshipIdempotently(t *testing.T) {
	db := setupCorrelationDB(t)
	ingestor := observation.NewIngestor(db, scope.NewRepository(db))
	err := ingestor.Ingest(context.Background(), "target", "run", observation.Input{EntityType: "ip_address", ObservationType: "dns_resolution", Subject: "api.example.com", Value: "192.0.2.10", Source: "dns", Confidence: 1, Metadata: `{}`, ObservedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{db: db}
	if err = svc.CorrelateRun(context.Background(), "target", "run"); err != nil {
		t.Fatal(err)
	}
	if err = svc.CorrelateRun(context.Background(), "target", "run"); err != nil {
		t.Fatal(err)
	}
	var relationships, provenance int
	if err = db.QueryRow("SELECT count(*) FROM relationships WHERE relationship_type='RESOLVES_TO'").Scan(&relationships); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT count(*) FROM relationship_evidence").Scan(&provenance); err != nil {
		t.Fatal(err)
	}
	if relationships != 1 || provenance != 1 {
		t.Fatalf("relationships=%d provenance=%d, want one each", relationships, provenance)
	}
}

func TestSnapshotComparisonReportsNewEntities(t *testing.T) {
	db := setupCorrelationDB(t)
	svc := &Service{db: db}
	first, err := svc.CreateManualSnapshot(context.Background(), "target", "baseline")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.Exec("INSERT INTO entities(id,target_id,type,canonical_value,display_value,metadata_json,scope_status,first_seen_at,last_seen_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", "entity", "target", "domain", "api.example.com", "api.example.com", `{}`, "unknown", now, now, now, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateManualSnapshot(context.Background(), "target", "after")
	if err != nil {
		t.Fatal(err)
	}
	comparison, err := svc.CompareSnapshots(context.Background(), "target", first.ID, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comparison.NewEntityIDs) != 1 || comparison.NewEntityIDs[0] != "entity" {
		t.Fatalf("new entities=%#v", comparison.NewEntityIDs)
	}
}
