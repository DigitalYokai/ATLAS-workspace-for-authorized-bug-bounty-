package schedule

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/atlas/atlas/backend/internal/collection"
	"github.com/google/uuid"
)

type Schedule struct {
	ID              string     `json:"id"`
	TargetID        string     `json:"targetId"`
	Collectors      []string   `json:"collectors"`
	IntervalHours   int        `json:"intervalHours"`
	Enabled         bool       `json:"enabled"`
	LastExecutedAt  *time.Time `json:"lastExecutedAt,omitempty"`
	NextExecutionAt time.Time  `json:"nextExecutionAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}
type CreateInput struct {
	Collectors    []string `json:"collectors"`
	IntervalHours int      `json:"intervalHours"`
	Enabled       *bool    `json:"enabled"`
}
type Service struct {
	db   *sql.DB
	runs *collection.Service
	stop chan struct{}
}

func NewService(db *sql.DB, runs *collection.Service) *Service {
	s := &Service{db: db, runs: runs, stop: make(chan struct{})}
	go s.loop()
	return s
}
func (s *Service) Close() { close(s.stop) }
func (s *Service) loop() {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-tick.C:
			_ = s.RunDue(context.Background())
		}
	}
}
func (s *Service) Create(ctx context.Context, targetID string, in CreateInput) (Schedule, error) {
	if len(in.Collectors) == 0 || in.IntervalHours < 1 {
		return Schedule{}, fmt.Errorf("select collectors and an interval of at least one hour")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	for _, name := range in.Collectors {
		if name != "dns" && name != "certificate_transparency" {
			return Schedule{}, fmt.Errorf("scheduled collection supports passive collectors only")
		}
	}
	now := time.Now().UTC()
	raw, _ := json.Marshal(in.Collectors)
	x := Schedule{ID: uuid.NewString(), TargetID: targetID, Collectors: in.Collectors, IntervalHours: in.IntervalHours, Enabled: enabled, NextExecutionAt: now.Add(time.Duration(in.IntervalHours) * time.Hour), CreatedAt: now, UpdatedAt: now}
	_, err := s.db.ExecContext(ctx, "INSERT INTO collection_schedules(id,target_id,collectors_json,interval_hours,enabled,next_execution_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)", x.ID, x.TargetID, string(raw), x.IntervalHours, boolInt(x.Enabled), x.NextExecutionAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	return x, err
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func (s *Service) List(ctx context.Context, targetID string) ([]Schedule, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,target_id,collectors_json,interval_hours,enabled,last_executed_at,next_execution_at,created_at,updated_at FROM collection_schedules WHERE target_id=? ORDER BY next_execution_at", targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Schedule{}
	for rows.Next() {
		var x Schedule
		var raw, next, created, updated string
		var last sql.NullString
		var enabled int
		if err = rows.Scan(&x.ID, &x.TargetID, &raw, &x.IntervalHours, &enabled, &last, &next, &created, &updated); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(raw), &x.Collectors)
		x.Enabled = enabled == 1
		x.NextExecutionAt, _ = time.Parse(time.RFC3339Nano, next)
		x.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		x.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		if last.Valid {
			t, _ := time.Parse(time.RFC3339Nano, last.String)
			x.LastExecutedAt = &t
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) SetEnabled(ctx context.Context, targetID, id string, enabled bool) error {
	_, err := s.db.ExecContext(ctx, "UPDATE collection_schedules SET enabled=?,updated_at=? WHERE id=? AND target_id=?", boolInt(enabled), time.Now().UTC().Format(time.RFC3339Nano), id, targetID)
	return err
}
func (s *Service) RunDue(ctx context.Context) error {
	var paused bool
	if err := s.db.QueryRowContext(ctx, "SELECT value_json='true' FROM app_settings WHERE key='scheduler_paused'").Scan(&paused); err != nil || paused {
		return err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,target_id,collectors_json,interval_hours FROM collection_schedules WHERE enabled=1 AND next_execution_at<=?", time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, targetID, raw string
		var hours int
		if err = rows.Scan(&id, &targetID, &raw, &hours); err != nil {
			return err
		}
		var running int
		if err = s.db.QueryRowContext(ctx, "SELECT count(*) FROM collection_runs WHERE schedule_id=? AND status IN ('pending','running')", id).Scan(&running); err != nil {
			return err
		}
		if running > 0 {
			continue
		}
		var collectors []string
		_ = json.Unmarshal([]byte(raw), &collectors)
		run, e := s.runs.Start(ctx, targetID, collection.CreateInput{Collectors: collectors, Mode: collection.Passive})
		if e != nil {
			continue
		}
		_, _ = s.db.ExecContext(ctx, "UPDATE collection_runs SET schedule_id=? WHERE id=?", id, run.ID)
		now := time.Now().UTC()
		_, _ = s.db.ExecContext(ctx, "UPDATE collection_schedules SET last_executed_at=?,next_execution_at=?,updated_at=? WHERE id=?", now.Format(time.RFC3339Nano), now.Add(time.Duration(hours)*time.Hour).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), id)
	}
	return rows.Err()
}
