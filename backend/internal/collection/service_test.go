package collection

import (
	"context"
	"database/sql"
	"github.com/atlas/atlas/backend/internal/platform/database"
	"github.com/google/uuid"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type blockingCollector struct{ current, max *int32 }

func (c blockingCollector) Name() string { return "blocking" }
func (c blockingCollector) Active() bool { return false }
func (c blockingCollector) Collect(ctx context.Context, _ Request, emit func(Observation) error) error {
	n := atomic.AddInt32(c.current, 1)
	for {
		m := atomic.LoadInt32(c.max)
		if n <= m || atomic.CompareAndSwapInt32(c.max, m, n) {
			break
		}
	}
	defer atomic.AddInt32(c.current, -1)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(40 * time.Millisecond):
		return emit(Observation{EntityType: "technology", Value: "fixture", Source: "blocking", Confidence: 1, Metadata: "{}", ObservedAt: time.Now().UTC()})
	}
}
func setupCollectionDB(t *testing.T) *sql.DB {
	t.Helper()
	db, e := database.Open(context.Background(), filepath.Join(t.TempDir(), "atlas.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, e = db.Exec("INSERT INTO targets(id,name,primary_domain,status,created_at,updated_at) VALUES(?,?,?,?,?,?)", "target", uuid.NewString(), "example.com", "active", now, now)
	if e != nil {
		t.Fatal(e)
	}
	return db
}
func TestRunPersistsCollectedObservation(t *testing.T) {
	db := setupCollectionDB(t)
	var current, max int32
	s := NewService(db, []Collector{blockingCollector{&current, &max}})
	run, e := s.Start(context.Background(), "target", CreateInput{Collectors: []string{"blocking"}, Mode: Passive, Configuration: Configuration{Concurrency: 1, TimeoutSeconds: 2}})
	if e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, e := s.Get(context.Background(), run.ID)
		if e == nil && got.Status == Completed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	var count int
	if e := db.QueryRow("SELECT count(*) FROM observations WHERE collection_run_id=?", run.ID).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 1 {
		got, _ := s.Get(context.Background(), run.ID)
		jobs, _ := s.repo.Jobs(context.Background(), run.ID)
		t.Fatalf("observation count=%d run=%#v jobs=%#v", count, got, jobs)
	}
	if atomic.LoadInt32(&max) > 1 {
		t.Fatalf("worker concurrency exceeded: %d", max)
	}
}
