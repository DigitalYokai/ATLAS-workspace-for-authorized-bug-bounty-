package activity

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID          string    `json:"id"`
	TargetID    *string   `json:"targetId,omitempty"`
	EventType   string    `json:"eventType"`
	SubjectType string    `json:"subjectType"`
	SubjectID   string    `json:"subjectId"`
	Message     string    `json:"message"`
	CreatedAt   time.Time `json:"createdAt"`
}
type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }
func (s *Service) Record(ctx context.Context, targetID *string, kind, subjectType, subjectID, message string) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO activity_events(id,target_id,event_type,subject_type,subject_id,message,created_at) VALUES(?,?,?,?,?,?,?)", uuid.NewString(), targetID, kind, subjectType, subjectID, message, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func (s *Service) List(ctx context.Context, targetID, kind, before string, limit int) ([]Event, error) {
	q := "SELECT id,target_id,event_type,subject_type,subject_id,message,created_at FROM activity_events WHERE 1=1"
	args := []any{}
	if targetID != "" {
		q += " AND target_id=?"
		args = append(args, targetID)
	}
	if kind != "" {
		q += " AND event_type=?"
		args = append(args, kind)
	}
	if before != "" {
		q += " AND created_at<?"
		args = append(args, before)
	}
	q += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		var target sql.NullString
		var created string
		if err = rows.Scan(&e.ID, &target, &e.EventType, &e.SubjectType, &e.SubjectID, &e.Message, &created); err != nil {
			return nil, err
		}
		if target.Valid {
			e.TargetID = &target.String
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, e)
	}
	return out, rows.Err()
}
