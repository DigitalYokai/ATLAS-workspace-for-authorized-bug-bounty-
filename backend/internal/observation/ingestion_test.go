package observation

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/atlas/atlas/backend/internal/platform/database"
	"github.com/atlas/atlas/backend/internal/scope"
	"github.com/google/uuid"
)

func setupIngestionDB(t *testing.T) *sql.DB {
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
	_, err = db.Exec("INSERT INTO collection_runs(id,target_id,mode,status,configuration_json,total_jobs,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)", "run", "target", "passive", "completed", `{}`, 0, now, now)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestIngestIsIdempotentWithinARun(t *testing.T) {
	db := setupIngestionDB(t)
	ingestor := NewIngestor(db, scope.NewRepository(db))
	input := Input{EntityType: "subdomain", Value: "API.EXAMPLE.COM", Source: "fixture", Confidence: 1, Metadata: `{}`, ObservedAt: time.Now().UTC()}
	if err := ingestor.Ingest(context.Background(), "target", "run", input); err != nil {
		t.Fatal(err)
	}
	if err := ingestor.Ingest(context.Background(), "target", "run", input); err != nil {
		t.Fatal(err)
	}
	var entities, observations int
	if err := db.QueryRow("SELECT count(*) FROM entities").Scan(&entities); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT count(*) FROM observations").Scan(&observations); err != nil {
		t.Fatal(err)
	}
	if entities != 1 || observations != 1 {
		t.Fatalf("entities=%d observations=%d, want one each", entities, observations)
	}
}

func TestIngestRejectsMissingSource(t *testing.T) {
	db := setupIngestionDB(t)
	err := NewIngestor(db, scope.NewRepository(db)).Ingest(context.Background(), "target", "run", Input{EntityType: "domain", Value: "example.com", Metadata: `{}`})
	if err == nil {
		t.Fatal("expected missing provenance source to be rejected")
	}
}

func TestIngestRejectsEvidenceAboveConfiguredLimit(t *testing.T) {
	db := setupIngestionDB(t)
	if _, err := db.Exec("UPDATE app_settings SET value_json='1024' WHERE key='evidence_max_bytes'"); err != nil {
		t.Fatal(err)
	}
	err := NewIngestor(db, scope.NewRepository(db)).Ingest(context.Background(), "target", "run", Input{EntityType: "domain", Value: "example.com", Source: "fixture", Metadata: `{}`, Evidence: string(make([]byte, 1025))})
	if err == nil {
		t.Fatal("expected oversize evidence to be rejected")
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM observations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("observations=%d, expected no partial ingestion", count)
	}
}
