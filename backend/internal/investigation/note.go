package investigation

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"time"
)

type Note struct {
	ID           string    `json:"id"`
	TargetID     string    `json:"targetId"`
	Title        string    `json:"title"`
	BodyMarkdown string    `json:"bodyMarkdown"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db} }
func (r *Repository) List(ctx context.Context, targetID string) ([]Note, error) {
	rows, e := r.db.QueryContext(ctx, "SELECT id,target_id,title,body_markdown,status,created_at,updated_at FROM investigation_notes WHERE target_id=? ORDER BY updated_at DESC", targetID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Note{}
	for rows.Next() {
		var n Note
		var c, u string
		if e = rows.Scan(&n.ID, &n.TargetID, &n.Title, &n.BodyMarkdown, &n.Status, &c, &u); e != nil {
			return nil, e
		}
		n.CreatedAt, _ = time.Parse(time.RFC3339Nano, c)
		n.UpdatedAt, _ = time.Parse(time.RFC3339Nano, u)
		out = append(out, n)
	}
	return out, rows.Err()
}
func (r *Repository) Create(ctx context.Context, targetID, title, body string) (Note, error) {
	n := Note{ID: uuid.NewString(), TargetID: targetID, Title: title, BodyMarkdown: body, Status: "unreviewed", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	_, e := r.db.ExecContext(ctx, "INSERT INTO investigation_notes(id,target_id,title,body_markdown,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?)", n.ID, n.TargetID, n.Title, n.BodyMarkdown, n.Status, n.CreatedAt.Format(time.RFC3339Nano), n.UpdatedAt.Format(time.RFC3339Nano))
	return n, e
}
