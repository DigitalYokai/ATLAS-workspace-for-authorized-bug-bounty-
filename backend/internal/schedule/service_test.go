package schedule

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/atlas/atlas/backend/internal/collection"
	"github.com/atlas/atlas/backend/internal/platform/database"
)

func TestScheduleRejectsActiveCollectorsAndPersistsPassiveSchedule(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "atlas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = db.Exec("INSERT INTO targets(id,name,primary_domain,status,created_at,updated_at) VALUES('target','Target','example.com','active',?,?)", now, now); err != nil {
		t.Fatal(err)
	}
	runs := collection.NewService(db, nil)
	s := NewService(db, runs)
	t.Cleanup(s.Close)
	if _, err = s.Create(context.Background(), "target", CreateInput{Collectors: []string{"http_metadata"}, IntervalHours: 1}); err == nil {
		t.Fatal("active collector schedule accepted")
	}
	item, err := s.Create(context.Background(), "target", CreateInput{Collectors: []string{"dns", "certificate_transparency"}, IntervalHours: 24})
	if err != nil {
		t.Fatal(err)
	}
	if !item.Enabled || item.IntervalHours != 24 {
		t.Fatalf("unexpected schedule %#v", item)
	}
}

func TestPurgeExpiredEvidencePreservesObservation(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "atlas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Now().UTC()
	if _, err = db.Exec("INSERT INTO targets(id,name,primary_domain,status,created_at,updated_at) VALUES('target','Target','example.com','active',?,?)", now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO evidence(id,target_id,evidence_type,source,content_hash,content,content_size,collected_at,created_at) VALUES('evidence','target','test','test','hash','raw',3,?,?)", now.AddDate(0, 0, -2).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO observations(id,target_id,collector,observation_type,subject,value,source,evidence_id,observed_at,confidence,raw_metadata_json,created_at) VALUES('observation','target','test','test','x','x','test','evidence',?,1,'{}',?)", now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE app_settings SET value_json='1' WHERE key='evidence_retention_days'"); err != nil {
		t.Fatal(err)
	}
	s := &Service{db: db}
	if err = s.PurgeExpiredEvidence(context.Background()); err != nil {
		t.Fatal(err)
	}
	var content sql.NullString
	if err = db.QueryRow("SELECT content FROM evidence WHERE id='evidence'").Scan(&content); err != nil {
		t.Fatal(err)
	}
	if content.Valid {
		t.Fatalf("retained evidence content=%q", content.String)
	}
	var targetID, evidenceID string
	if err = db.QueryRow("SELECT target_id,evidence_id FROM observations WHERE id='observation'").Scan(&targetID, &evidenceID); err != nil {
		t.Fatal(err)
	}
	if targetID != "target" || evidenceID != "evidence" {
		t.Fatalf("observation provenance changed target=%q evidence=%q", targetID, evidenceID)
	}
}
