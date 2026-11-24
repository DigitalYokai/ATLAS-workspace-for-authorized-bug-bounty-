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
