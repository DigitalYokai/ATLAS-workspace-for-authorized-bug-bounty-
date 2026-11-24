package httpapi

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/atlas/atlas/backend/internal/schedule"
)

func (a *API) listActivity(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("id")
	if targetID != "" && !a.ensureTarget(w, r) {
		return
	}
	limit, err := boundedLimit(r, 50)
	if err != nil {
		fail(w, 400, "validation_failed", err.Error())
		return
	}
	items, err := a.activity.List(r.Context(), targetID, r.URL.Query().Get("type"), r.URL.Query().Get("before"), limit)
	if err != nil {
		fail(w, 500, "internal_error", "could not load activity")
		return
	}
	write(w, 200, map[string]any{"data": items})
}

func (a *API) targetDashboard(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	id := r.PathValue("id")
	metrics := map[string]any{}
	queries := map[string]string{
		"entityCount":                "SELECT count(*) FROM entities WHERE target_id=?",
		"inScopeEntityCount":         "SELECT count(*) FROM entities WHERE target_id=? AND scope_status='included'",
		"outOfScopeObservationCount": "SELECT count(*) FROM observations o JOIN entities e ON e.target_id=o.target_id AND lower(e.canonical_value)=lower(o.value) WHERE o.target_id=? AND e.scope_status='excluded'",
		"relationshipCount":          "SELECT count(*) FROM relationships WHERE target_id=?",
		"pendingNotes":               "SELECT count(*) FROM investigation_notes WHERE target_id=? AND status IN ('unreviewed','reviewing','needs_follow_up')",
		"reviewedEntities":           "SELECT count(*) FROM entities WHERE target_id=? AND investigation_status IN ('reviewed','archived')",
	}
	for key, query := range queries {
		var n int
		if err := a.db.QueryRowContext(r.Context(), query, id).Scan(&n); err != nil {
			fail(w, 500, "internal_error", "could not calculate dashboard")
			return
		}
		metrics[key] = n
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT type,count(*) FROM entities WHERE target_id=? GROUP BY type ORDER BY type", id)
	if err != nil {
		fail(w, 500, "internal_error", "could not load entity metrics")
		return
	}
	defer rows.Close()
	byType := map[string]int{}
	for rows.Next() {
		var typ string
		var n int
		if err = rows.Scan(&typ, &n); err != nil {
			fail(w, 500, "internal_error", "could not read entity metrics")
			return
		}
		byType[typ] = n
	}
	runs, err := a.runs.List(r.Context(), id)
	if err != nil {
		fail(w, 500, "internal_error", "could not load runs")
		return
	}
	if len(runs) > 10 {
		runs = runs[:10]
	}
	changes, err := a.activity.List(r.Context(), id, "", "", 10)
	if err != nil {
		fail(w, 500, "internal_error", "could not load changes")
		return
	}
	metrics["entityCountsByType"] = byType
	metrics["recentRuns"] = runs
	metrics["recentActivity"] = changes
	write(w, 200, metrics)
}
func (a *API) listNotifications(w http.ResponseWriter, r *http.Request) {
	limit, err := boundedLimit(r, 50)
	if err != nil {
		fail(w, 400, "validation_failed", err.Error())
		return
	}
	items, err := a.notifications.List(r.Context(), r.URL.Query().Get("unread") == "true", limit)
	if err != nil {
		fail(w, 500, "internal_error", "could not load notifications")
		return
	}
	write(w, 200, map[string]any{"data": items})
}
func (a *API) updateNotification(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Read bool `json:"read"`
	}
	if err := decode(r, &input); err != nil || !input.Read {
		fail(w, 400, "validation_failed", "read must be true")
		return
	}
	if err := a.notifications.MarkRead(r.Context(), r.PathValue("notificationId")); err != nil {
		fail(w, 500, "internal_error", "could not update notification")
		return
	}
	write(w, 200, map[string]string{"status": "read"})
}
func (a *API) listSchedules(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	items, err := a.schedules.List(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, 500, "internal_error", "could not load schedules")
		return
	}
	write(w, 200, map[string]any{"data": items})
}
func (a *API) createSchedule(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var in schedule.CreateInput
	if err := decode(r, &in); err != nil {
		fail(w, 400, "validation_failed", "invalid request body")
		return
	}
	item, err := a.schedules.Create(r.Context(), r.PathValue("id"), in)
	if err != nil {
		fail(w, 400, "validation_failed", err.Error())
		return
	}
	_ = a.activity.Record(r.Context(), ptr(r.PathValue("id")), "schedule_created", "schedule", item.ID, "Passive collection schedule created")
	write(w, 201, item)
}
func (a *API) updateSchedule(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decode(r, &in); err != nil || in.Enabled == nil {
		fail(w, 400, "validation_failed", "enabled is required")
		return
	}
	if err := a.schedules.SetEnabled(r.Context(), r.PathValue("id"), r.PathValue("scheduleId"), *in.Enabled); err != nil {
		fail(w, 500, "internal_error", "could not update schedule")
		return
	}
	write(w, 200, map[string]bool{"enabled": *in.Enabled})
}
func ptr(v string) *string { return &v }

func (a *API) getSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), "SELECT key,value_json FROM app_settings ORDER BY key")
	if err != nil {
		fail(w, 500, "internal_error", "could not load settings")
		return
	}
	defer rows.Close()
	out := map[string]any{}
	for rows.Next() {
		var key, value string
		if err = rows.Scan(&key, &value); err != nil {
			fail(w, 500, "internal_error", "could not read settings")
			return
		}
		var decoded any
		_ = json.Unmarshal([]byte(value), &decoded)
		out[key] = decoded
	}
	write(w, 200, out)
}
func (a *API) updateSettings(w http.ResponseWriter, r *http.Request) {
	var input map[string]any
	if err := decode(r, &input); err != nil {
		fail(w, 400, "validation_failed", "invalid settings")
		return
	}
	allowed := map[string]bool{"scheduler_paused": true, "default_concurrency": true, "evidence_retention_days": true, "evidence_max_bytes": true}
	for key, value := range input {
		if !allowed[key] {
			fail(w, 400, "validation_failed", "unknown setting")
			return
		}
		if key == "default_concurrency" {
			n, ok := value.(float64)
			if !ok || n < 1 || n > 4 {
				fail(w, 400, "validation_failed", "default_concurrency must be 1 through 4")
				return
			}
		}
		if key == "evidence_retention_days" {
			n, ok := value.(float64)
			if !ok || n < 0 {
				fail(w, 400, "validation_failed", "evidence_retention_days must be non-negative")
				return
			}
		}
		if key == "evidence_max_bytes" {
			n, ok := value.(float64)
			if !ok || n < 1024 {
				fail(w, 400, "validation_failed", "evidence_max_bytes must be at least 1024")
				return
			}
		}
		if key == "scheduler_paused" {
			if _, ok := value.(bool); !ok {
				fail(w, 400, "validation_failed", "scheduler_paused must be boolean")
				return
			}
		}
		encoded, _ := json.Marshal(value)
		if _, err := a.db.ExecContext(r.Context(), "UPDATE app_settings SET value_json=?,updated_at=? WHERE key=?", string(encoded), time.Now().UTC().Format(time.RFC3339Nano), key); err != nil {
			fail(w, 500, "internal_error", "could not update settings")
			return
		}
	}
	write(w, 200, input)
}

func (a *API) downloadBackup(w http.ResponseWriter, r *http.Request) {
	dir, err := os.MkdirTemp("", "atlas-backup-")
	if err != nil {
		fail(w, 500, "internal_error", "could not create backup workspace")
		return
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "atlas-backup.db")
	if _, err = a.db.ExecContext(r.Context(), "VACUUM INTO ?", path); err != nil {
		fail(w, 500, "internal_error", "could not create database backup")
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		fail(w, 500, "internal_error", "could not read database backup")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", "attachment; filename=atlas-backup.db")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (a *API) exportTarget(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	targetID := r.PathValue("id")
	format := r.PathValue("format")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	switch format {
	case "json", "graph":
		rows, err := a.db.QueryContext(r.Context(), "SELECT id,type,canonical_value,display_value,scope_status,first_seen_at,last_seen_at FROM entities WHERE target_id=? ORDER BY type,canonical_value", targetID)
		if err != nil {
			fail(w, 500, "internal_error", "could not export entities")
			return
		}
		defer rows.Close()
		entities := []map[string]string{}
		for rows.Next() {
			var id, typ, canonical, display, scope, first, last string
			_ = rows.Scan(&id, &typ, &canonical, &display, &scope, &first, &last)
			entities = append(entities, map[string]string{"id": id, "type": typ, "canonicalValue": canonical, "displayValue": display, "scopeStatus": scope, "firstSeenAt": first, "lastSeenAt": last})
		}
		observations, err := a.exportObservations(r, targetID)
		if err != nil {
			fail(w, 500, "internal_error", "could not export observations")
			return
		}
		payload := map[string]any{"generatedAt": now, "targetId": targetID, "entities": entities, "observations": observations}
		if format == "graph" {
			graph := httptestGraph(a, r, targetID)
			payload["graph"] = graph
		}
		w.Header().Set("Content-Disposition", "attachment; filename=atlas-"+format+".json")
		write(w, 200, payload)
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=atlas-assets.csv")
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{"asset", "type", "scope_status", "first_observed", "last_observed", "max_confidence", "sources"})
		rows, err := a.db.QueryContext(r.Context(), "SELECT e.display_value,e.type,e.scope_status,e.first_seen_at,e.last_seen_at,COALESCE(MAX(o.confidence),0),COALESCE(GROUP_CONCAT(DISTINCT o.source),'') FROM entities e LEFT JOIN observations o ON o.target_id=e.target_id AND lower(o.value)=lower(e.canonical_value) WHERE e.target_id=? GROUP BY e.id ORDER BY e.display_value", targetID)
		if err != nil {
			fail(w, 500, "internal_error", "could not export assets")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var v, t, s, f, l, sources string
			var confidence float64
			_ = rows.Scan(&v, &t, &s, &f, &l, &confidence, &sources)
			_ = writer.Write([]string{v, t, s, f, l, strconv.FormatFloat(confidence, 'f', -1, 64), sources})
		}
		writer.Flush()
	case "markdown":
		var name, domain string
		if err := a.db.QueryRowContext(r.Context(), "SELECT name,primary_domain FROM targets WHERE id=?", targetID).Scan(&name, &domain); err != nil {
			fail(w, 500, "internal_error", "could not export target")
			return
		}
		var entities, notes int
		_ = a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM entities WHERE target_id=?", targetID).Scan(&entities)
		_ = a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM investigation_notes WHERE target_id=?", targetID).Scan(&notes)
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=atlas-investigation.md")
		_, _ = w.Write([]byte("# ATLAS investigation summary\n\nGenerated: " + now + "\n\nTarget: " + name + " (`" + domain + "`)\n\n- Entities: " + strconv.Itoa(entities) + "\n- Investigation notes: " + strconv.Itoa(notes) + "\n\nAll intelligence is evidence-backed and not a vulnerability finding.\n"))
	default:
		fail(w, 400, "validation_failed", "format must be json, csv, markdown, or graph")
	}
}

func (a *API) exportObservations(r *http.Request, targetID string) ([]map[string]any, error) {
	rows, err := a.db.QueryContext(r.Context(), "SELECT o.id,o.observation_type,o.subject,o.value,o.source,o.observed_at,o.confidence,o.origin,o.evidence_id,COALESCE(e.scope_status,'unknown') FROM observations o LEFT JOIN entities e ON e.target_id=o.target_id AND lower(e.canonical_value)=lower(o.value) WHERE o.target_id=? ORDER BY o.observed_at DESC", targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, typ, subject, value, source, observedAt, origin, scope string
		var confidence float64
		var evidenceID sql.NullString
		if err := rows.Scan(&id, &typ, &subject, &value, &source, &observedAt, &confidence, &origin, &evidenceID, &scope); err != nil {
			return nil, err
		}
		item := map[string]any{"id": id, "observationType": typ, "subject": subject, "value": value, "source": source, "observedAt": observedAt, "confidence": confidence, "origin": origin, "scopeStatus": scope}
		if evidenceID.Valid {
			item["evidenceId"] = evidenceID.String
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func httptestGraph(a *API, r *http.Request, targetID string) map[string]any {
	rows, err := a.db.QueryContext(r.Context(), "SELECT id,source_entity_id,destination_entity_id,relationship_type,confidence,status,assertion_kind FROM relationships WHERE target_id=? LIMIT 100", targetID)
	if err != nil {
		return map[string]any{"nodes": []any{}, "edges": []any{}}
	}
	defer rows.Close()
	edges := []map[string]any{}
	for rows.Next() {
		var id, source, destination, typ, status, assertion string
		var confidence float64
		_ = rows.Scan(&id, &source, &destination, &typ, &confidence, &status, &assertion)
		edges = append(edges, map[string]any{"id": id, "sourceEntityId": source, "destinationEntityId": destination, "relationshipType": typ, "confidence": confidence, "status": status, "assertionKind": assertion})
	}
	return map[string]any{"edges": edges}
}
