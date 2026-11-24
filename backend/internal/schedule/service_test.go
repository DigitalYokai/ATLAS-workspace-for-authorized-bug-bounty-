package schedule

import (
	"context"
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
