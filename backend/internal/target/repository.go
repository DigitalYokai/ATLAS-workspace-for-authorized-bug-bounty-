package target

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }
func scan(row interface{ Scan(...any) error }) (Target, error) {
	var t Target
	var created, updated string
	var archived sql.NullString
	var rate sql.NullInt64
	err := row.Scan(&t.ID, &t.Name, &t.PrimaryDomain, &t.Platform, &t.ProgramURL, &t.Description, &t.ProgramRules, &t.ProgramRestrictions, &rate, &t.Status, &created, &updated, &archived)
	if err != nil {
		return t, err
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	t.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	if rate.Valid {
		v := int(rate.Int64)
		t.RateLimitPerMinute = &v
	}
	if archived.Valid {
		v, _ := time.Parse(time.RFC3339Nano, archived.String)
		t.ArchivedAt = &v
	}
	return t, nil
}

const targetColumns = "id,name,primary_domain,platform,program_url,description,program_rules,program_restrictions,rate_limit_per_minute,status,created_at,updated_at,archived_at"

func (r *Repository) Create(ctx context.Context, tx *sql.Tx, t Target) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO targets ("+targetColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)", t.ID, t.Name, t.PrimaryDomain, t.Platform, t.ProgramURL, t.Description, t.ProgramRules, t.ProgramRestrictions, t.RateLimitPerMinute, t.Status, t.CreatedAt.Format(time.RFC3339Nano), t.UpdatedAt.Format(time.RFC3339Nano), nil)
	return err
}
func (r *Repository) Get(ctx context.Context, id string) (Target, error) {
	return scan(r.db.QueryRowContext(ctx, "SELECT "+targetColumns+" FROM targets WHERE id=?", id))
}
func (r *Repository) List(ctx context.Context, status Status, limit int) ([]Target, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+targetColumns+" FROM targets WHERE status=? ORDER BY updated_at DESC LIMIT ?", status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Target{}
	for rows.Next() {
		t, e := scan(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (r *Repository) Update(ctx context.Context, t Target) error {
	_, err := r.db.ExecContext(ctx, "UPDATE targets SET name=?,platform=?,program_url=?,description=?,program_rules=?,program_restrictions=?,rate_limit_per_minute=?,status=?,updated_at=?,archived_at=? WHERE id=?", t.Name, t.Platform, t.ProgramURL, t.Description, t.ProgramRules, t.ProgramRestrictions, t.RateLimitPerMinute, t.Status, t.UpdatedAt.Format(time.RFC3339Nano), nullableTime(t.ArchivedAt), t.ID)
	return err
}
func nullableTime(v *time.Time) any {
	if v == nil {
		return nil
	}
	return v.Format(time.RFC3339Nano)
}
func (r *Repository) Counts(ctx context.Context, id string) (int, int, int, error) {
	var e, rel, o int
	err := r.db.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM entities WHERE target_id=?),(SELECT count(*) FROM relationships WHERE target_id=?),(SELECT count(*) FROM observations WHERE target_id=?)", id, id, id).Scan(&e, &rel, &o)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("counts: %w", err)
	}
	return e, rel, o, nil
}
