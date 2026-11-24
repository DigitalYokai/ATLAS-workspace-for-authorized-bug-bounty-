package collection

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db} }
func (r *Repository) Create(ctx context.Context, targetID string, in CreateInput) (Run, error) {
	now := time.Now().UTC()
	run := Run{ID: uuid.NewString(), TargetID: targetID, Mode: in.Mode, Status: Pending, Configuration: in.Configuration, TotalJobs: len(in.Collectors), CreatedAt: now, UpdatedAt: now}
	cfg, _ := json.Marshal(in.Configuration)
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return run, e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, "INSERT INTO collection_runs(id,target_id,mode,status,configuration_json,total_jobs,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)", run.ID, targetID, run.Mode, run.Status, string(cfg), run.TotalJobs, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if e != nil {
		return run, e
	}
	for _, name := range in.Collectors {
		_, e = tx.ExecContext(ctx, "INSERT INTO collection_jobs(id,target_id,run_id,collector_name,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?)", uuid.NewString(), targetID, run.ID, name, Pending, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
		if e != nil {
			return run, e
		}
	}
	return run, tx.Commit()
}
func (r *Repository) Jobs(ctx context.Context, runID string) ([]Job, error) {
	rows, e := r.db.QueryContext(ctx, "SELECT id,collector_name,status,error_summary,started_at,completed_at FROM collection_jobs WHERE run_id=? ORDER BY created_at", runID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		var j Job
		var s, c sql.NullString
		e = rows.Scan(&j.ID, &j.CollectorName, &j.Status, &j.ErrorSummary, &s, &c)
		if e != nil {
			return nil, e
		}
		if s.Valid {
			v, _ := time.Parse(time.RFC3339Nano, s.String)
			j.StartedAt = &v
		}
		if c.Valid {
			v, _ := time.Parse(time.RFC3339Nano, c.String)
			j.CompletedAt = &v
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *Repository) SetRun(ctx context.Context, id string, status Status, completed, failed int, message string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, e := r.db.ExecContext(ctx, "UPDATE collection_runs SET status=?,completed_jobs=?,failed_jobs=?,error_summary=?,updated_at=?,started_at=CASE WHEN ?='running' AND started_at IS NULL THEN ? ELSE started_at END,completed_at=CASE WHEN ? IN ('completed','partially_completed','failed','cancelled') THEN ? ELSE completed_at END WHERE id=?", status, completed, failed, message, now, status, now, status, now, id)
	return e
}
func (r *Repository) SetJob(ctx context.Context, id string, status Status, message string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, e := r.db.ExecContext(ctx, "UPDATE collection_jobs SET status=?,error_summary=?,updated_at=?,started_at=CASE WHEN ?='running' AND started_at IS NULL THEN ? ELSE started_at END,completed_at=CASE WHEN ? IN ('completed','failed','cancelled') THEN ? ELSE completed_at END WHERE id=?", status, message, now, status, now, status, now, id)
	return e
}
func (r *Repository) AddEvent(ctx context.Context, targetID, runID, kind, message string) error {
	_, e := r.db.ExecContext(ctx, "INSERT INTO collection_events(id,target_id,run_id,event_type,message,created_at) VALUES(?,?,?,?,?,?)", uuid.NewString(), targetID, runID, kind, message, time.Now().UTC().Format(time.RFC3339Nano))
	return e
}
func (r *Repository) Get(ctx context.Context, id string) (Run, error) {
	var x Run
	var cfg, created, updated string
	var started, completed sql.NullString
	err := r.db.QueryRowContext(ctx, "SELECT id,target_id,mode,status,configuration_json,error_summary,total_jobs,completed_jobs,failed_jobs,started_at,completed_at,created_at,updated_at FROM collection_runs WHERE id=?", id).Scan(&x.ID, &x.TargetID, &x.Mode, &x.Status, &cfg, &x.ErrorSummary, &x.TotalJobs, &x.CompletedJobs, &x.FailedJobs, &started, &completed, &created, &updated)
	if err != nil {
		return x, err
	}
	_ = json.Unmarshal([]byte(cfg), &x.Configuration)
	x.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	x.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	if started.Valid {
		v, _ := time.Parse(time.RFC3339Nano, started.String)
		x.StartedAt = &v
	}
	if completed.Valid {
		v, _ := time.Parse(time.RFC3339Nano, completed.String)
		x.CompletedAt = &v
	}
	return x, nil
}
func (r *Repository) List(ctx context.Context, targetID string) ([]Run, error) {
	rows, e := r.db.QueryContext(ctx, "SELECT id FROM collection_runs WHERE target_id=? ORDER BY created_at DESC", targetID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		x, e := r.Get(ctx, id)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
