package scope

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db} }
func (r *Repository) List(ctx context.Context, targetID string) ([]Rule, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id,target_id,rule_kind,pattern,pattern_kind,permitted_methods_json,created_at,updated_at FROM scope_rules WHERE target_id=? ORDER BY rule_kind, pattern", targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := []Rule{}
	for rows.Next() {
		var x Rule
		var methods, created, updated string
		if err = rows.Scan(&x.ID, &x.TargetID, &x.Kind, &x.Pattern, &x.PatternKind, &methods, &created, &updated); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(methods), &x.PermittedMethods)
		x.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		x.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		rules = append(rules, x)
	}
	return rules, rows.Err()
}
func (r *Repository) Replace(ctx context.Context, targetID string, inputs []Input) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM scope_rules WHERE target_id=?", targetID); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, in := range inputs {
		in, kind, e := NormalizeInput(in)
		if e != nil {
			return e
		}
		methods, _ := json.Marshal(in.PermittedMethods)
		if _, e = tx.ExecContext(ctx, "INSERT INTO scope_rules(id,target_id,rule_kind,pattern,pattern_kind,permitted_methods_json,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)", uuid.NewString(), targetID, in.Kind, in.Pattern, kind, string(methods), now, now); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (r *Repository) CreateAll(ctx context.Context, tx *sql.Tx, targetID string, inputs []Input) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, in := range inputs {
		in, kind, err := NormalizeInput(in)
		if err != nil {
			return err
		}
		m, _ := json.Marshal(in.PermittedMethods)
		if _, err = tx.ExecContext(ctx, "INSERT INTO scope_rules(id,target_id,rule_kind,pattern,pattern_kind,permitted_methods_json,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)", uuid.NewString(), targetID, in.Kind, in.Pattern, kind, string(m), now, now); err != nil {
			return err
		}
	}
	return nil
}
