package notification

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"time"
)

type Item struct {
	ID        string     `json:"id"`
	TargetID  *string    `json:"targetId,omitempty"`
	EventType string     `json:"eventType"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	Link      string     `json:"link"`
	ReadAt    *time.Time `json:"readAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}
type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db} }
func (s *Service) Create(ctx context.Context, targetID *string, typ, key, title, message, link string) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO notifications(id,target_id,event_type,dedupe_key,title,message,link,created_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(dedupe_key) DO NOTHING", uuid.NewString(), targetID, typ, key, title, message, link, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func (s *Service) List(ctx context.Context, unread bool, limit int) ([]Item, error) {
	q := "SELECT id,target_id,event_type,title,message,link,read_at,created_at FROM notifications"
	if unread {
		q += " WHERE read_at IS NULL"
	}
	q += " ORDER BY created_at DESC LIMIT ?"
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var x Item
		var target, read sql.NullString
		var created string
		if err = rows.Scan(&x.ID, &target, &x.EventType, &x.Title, &x.Message, &x.Link, &read, &created); err != nil {
			return nil, err
		}
		if target.Valid {
			x.TargetID = &target.String
		}
		if read.Valid {
			t, _ := time.Parse(time.RFC3339Nano, read.String)
			x.ReadAt = &t
		}
		x.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) MarkRead(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE notifications SET read_at=COALESCE(read_at,?) WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}
