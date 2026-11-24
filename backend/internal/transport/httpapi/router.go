package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/atlas/atlas/backend/internal/activity"
	"github.com/atlas/atlas/backend/internal/auth"
	"github.com/atlas/atlas/backend/internal/collection"
	"github.com/atlas/atlas/backend/internal/correlation"
	"github.com/atlas/atlas/backend/internal/investigation"
	"github.com/atlas/atlas/backend/internal/notification"
	"github.com/atlas/atlas/backend/internal/observation"
	"github.com/atlas/atlas/backend/internal/schedule"
	"github.com/atlas/atlas/backend/internal/scope"
	"github.com/atlas/atlas/backend/internal/target"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type API struct {
	db            *sql.DB
	targets       *target.Service
	scopes        *scope.Repository
	notes         *investigation.Repository
	ingest        *observation.Ingestor
	runs          *collection.Service
	correlation   *correlation.Service
	auth          *auth.Service
	activity      *activity.Service
	notifications *notification.Service
	schedules     *schedule.Service
	log           *slog.Logger
}

func New(db *sql.DB, log *slog.Logger) http.Handler {
	sc := scope.NewRepository(db)
	tr := target.NewRepository(db)
	runs := collection.NewService(db, []collection.Collector{collection.DNSCollector{}, collection.HTTPMetadataCollector{}, collection.TLSCollector{}, collection.CTCollector{}})
	corr := correlation.NewService(db)
	activities := activity.NewService(db)
	notices := notification.NewService(db)
	runs.SetCompletionProcessor(func(ctx context.Context, run collection.Run) error {
		_ = activities.Record(ctx, &run.TargetID, "collection_finished", "run", run.ID, "Collection run "+string(run.Status))
		if run.Status == collection.Failed {
			_ = notices.Create(ctx, &run.TargetID, "collection_failed", "run:"+run.ID, "Collection failed", "A collection run failed", "/targets/"+run.TargetID+"/runs")
		}
		return corr.QueueRun(ctx, run.TargetID, run.ID)
	})
	api := &API{db: db, targets: target.NewService(db, tr, sc), scopes: sc, notes: investigation.NewRepository(db), ingest: observation.NewIngestor(db, sc), runs: runs, correlation: corr, auth: auth.NewService(db), activity: activities, notifications: notices, schedules: schedule.NewService(db, runs), log: log}
	return api.routes()
}
func (a *API) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("POST /api/v1/auth/login", a.login)
	m.HandleFunc("POST /api/v1/auth/logout", a.logout)
	m.HandleFunc("GET /api/v1/auth/me", a.me)
	m.HandleFunc("GET /api/v1/auth/csrf", a.csrf)
	m.HandleFunc("GET /api/v1/dashboard", a.dashboard)
	m.HandleFunc("GET /api/v1/targets", a.listTargets)
	m.HandleFunc("POST /api/v1/targets", a.createTarget)
	m.HandleFunc("GET /api/v1/targets/{id}", a.getTarget)
	m.HandleFunc("GET /api/v1/targets/{id}/dashboard", a.targetDashboard)
	m.HandleFunc("PATCH /api/v1/targets/{id}", a.patchTarget)
	m.HandleFunc("GET /api/v1/targets/{id}/scope", a.listScope)
	m.HandleFunc("PUT /api/v1/targets/{id}/scope", a.replaceScope)
	m.HandleFunc("POST /api/v1/targets/{id}/scope/validate", a.validateScope)
	m.HandleFunc("GET /api/v1/targets/{id}/notes", a.listNotes)
	m.HandleFunc("POST /api/v1/targets/{id}/notes", a.createNote)
	m.HandleFunc("GET /api/v1/targets/{id}/runs", a.listRuns)
	m.HandleFunc("POST /api/v1/targets/{id}/runs", a.createRun)
	m.HandleFunc("GET /api/v1/runs/{id}", a.getRun)
	m.HandleFunc("POST /api/v1/runs/{id}/cancel", a.cancelRun)
	m.HandleFunc("GET /api/v1/runs/{id}/events", a.runEvents)
	m.HandleFunc("GET /api/v1/targets/{id}/entities", a.listEntities)
	m.HandleFunc("GET /api/v1/targets/{id}/entities/search", a.listEntities)
	m.HandleFunc("GET /api/v1/targets/{id}/entities/{entityId}", a.getEntity)
	m.HandleFunc("PATCH /api/v1/targets/{id}/entities/{entityId}/investigation", a.updateEntityInvestigation)
	m.HandleFunc("GET /api/v1/targets/{id}/entities/{entityId}/relationships", a.entityRelationships)
	m.HandleFunc("GET /api/v1/targets/{id}/entities/{entityId}/observations", a.entityObservations)
	m.HandleFunc("GET /api/v1/targets/{id}/entities/{entityId}/tags", a.listEntityTags)
	m.HandleFunc("POST /api/v1/targets/{id}/entities/{entityId}/tags", a.addEntityTag)
	m.HandleFunc("GET /api/v1/targets/{id}/graph", a.graph)
	m.HandleFunc("POST /api/v1/targets/{id}/observations", a.createManualObservation)
	m.HandleFunc("POST /api/v1/targets/{id}/relationships", a.createManualRelationship)
	m.HandleFunc("GET /api/v1/targets/{id}/snapshots", a.listSnapshots)
	m.HandleFunc("POST /api/v1/targets/{id}/snapshots", a.createSnapshot)
	m.HandleFunc("GET /api/v1/targets/{id}/snapshots/compare", a.compareSnapshots)
	m.HandleFunc("GET /api/v1/targets/{id}/observations", a.listObservations)
	m.HandleFunc("GET /api/v1/targets/{id}/evidence/{evidenceId}", a.getEvidence)
	m.HandleFunc("GET /api/v1/activity", a.listActivity)
	m.HandleFunc("GET /api/v1/targets/{id}/activity", a.listActivity)
	m.HandleFunc("GET /api/v1/notifications", a.listNotifications)
	m.HandleFunc("PATCH /api/v1/notifications/{notificationId}", a.updateNotification)
	m.HandleFunc("GET /api/v1/targets/{id}/schedules", a.listSchedules)
	m.HandleFunc("POST /api/v1/targets/{id}/schedules", a.createSchedule)
	m.HandleFunc("PATCH /api/v1/targets/{id}/schedules/{scheduleId}", a.updateSchedule)
	m.HandleFunc("GET /api/v1/settings", a.getSettings)
	m.HandleFunc("PATCH /api/v1/settings", a.updateSettings)
	m.HandleFunc("GET /api/v1/settings/backup", a.downloadBackup)
	m.HandleFunc("GET /api/v1/targets/{id}/exports/{format}", a.exportTarget)
	return middleware(m, a.log, a.auth)
}
func middleware(next http.Handler, log *slog.Logger, sessions *auth.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-CSRF-Token")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		if r.URL.Path == "/healthz" || r.URL.Path == "/api/v1/auth/login" {
			next.ServeHTTP(w, r)
			return
		}
		required, err := sessions.Required(r.Context())
		if err != nil {
			fail(w, 500, "internal_error", "could not check authentication")
			return
		}
		if required {
			cookie, cookieErr := r.Cookie(auth.SessionCookie)
			if cookieErr != nil {
				fail(w, 401, "unauthorized", "login required")
				return
			}
			session, sessionErr := sessions.Session(r.Context(), cookie.Value)
			if sessionErr != nil {
				fail(w, 401, "unauthorized", "invalid or expired session")
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				if r.Header.Get("X-CSRF-Token") != session.CSRFToken {
					fail(w, 403, "csrf_failed", "valid CSRF token required")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func write(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, code, msg string) {
	write(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}
func (a *API) listTargets(w http.ResponseWriter, r *http.Request) {
	status := target.Status(r.URL.Query().Get("status"))
	if status == "" {
		status = target.StatusActive
	}
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		var e error
		limit, e = strconv.Atoi(s)
		if e != nil || limit < 1 || limit > 100 {
			fail(w, 400, "validation_failed", "limit must be between 1 and 100")
			return
		}
	}
	items, e := a.targets.List(r.Context(), status, limit)
	if e != nil {
		fail(w, 500, "internal_error", "could not list targets")
		return
	}
	write(w, 200, map[string]any{"data": items})
}

type createTargetRequest struct {
	target.CreateInput
	Scope []scope.Input `json:"scope"`
}

func (a *API) createTarget(w http.ResponseWriter, r *http.Request) {
	var in createTargetRequest
	if e := decode(r, &in); e != nil {
		fail(w, 400, "validation_failed", "invalid request body")
		return
	}
	t, e := a.targets.Create(r.Context(), in.CreateInput, in.Scope)
	if e != nil {
		fail(w, 400, "validation_failed", e.Error())
		return
	}
	write(w, 201, t)
}
func (a *API) getTarget(w http.ResponseWriter, r *http.Request) {
	t, e := a.targets.Get(r.Context(), r.PathValue("id"))
	if errors.Is(e, sql.ErrNoRows) {
		fail(w, 404, "not_found", "target not found")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error", "could not load target")
		return
	}
	ec, rc, oc, e := a.targets.Counts(r.Context(), t.ID)
	if e != nil {
		fail(w, 500, "internal_error", "could not load target summary")
		return
	}
	write(w, 200, map[string]any{"target": t, "summary": map[string]int{"entityCount": ec, "relationshipCount": rc, "observationCount": oc}})
}
func (a *API) patchTarget(w http.ResponseWriter, r *http.Request) {
	var in target.UpdateInput
	if e := decode(r, &in); e != nil {
		fail(w, 400, "validation_failed", "invalid request body")
		return
	}
	t, e := a.targets.Update(r.Context(), r.PathValue("id"), in)
	if errors.Is(e, sql.ErrNoRows) {
		fail(w, 404, "not_found", "target not found")
		return
	}
	if e != nil {
		fail(w, 400, "validation_failed", e.Error())
		return
	}
	write(w, 200, t)
}
func (a *API) ensureTarget(w http.ResponseWriter, r *http.Request) bool {
	_, e := a.targets.Get(r.Context(), r.PathValue("id"))
	if errors.Is(e, sql.ErrNoRows) {
		fail(w, 404, "not_found", "target not found")
		return false
	}
	if e != nil {
		fail(w, 500, "internal_error", "could not load target")
		return false
	}
	return true
}
func (a *API) listScope(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	rules, e := a.scopes.List(r.Context(), r.PathValue("id"))
	if e != nil {
		fail(w, 500, "internal_error", "could not list scope")
		return
	}
	write(w, 200, map[string]any{"data": rules})
}
func (a *API) replaceScope(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var rules []scope.Input
	if e := decode(r, &rules); e != nil {
		fail(w, 400, "validation_failed", "invalid request body")
		return
	}
	if e := a.scopes.Replace(r.Context(), r.PathValue("id"), rules); e != nil {
		fail(w, 400, "validation_failed", e.Error())
		return
	}
	a.listScope(w, r)
}
func (a *API) validateScope(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var in struct {
		Host   string `json:"host"`
		Method string `json:"method"`
	}
	if e := decode(r, &in); e != nil {
		fail(w, 400, "validation_failed", "invalid request body")
		return
	}
	rules, e := a.scopes.List(r.Context(), r.PathValue("id"))
	if e != nil {
		fail(w, 500, "internal_error", "could not evaluate scope")
		return
	}
	write(w, 200, scope.Evaluate(rules, in.Host, in.Method))
}
func (a *API) dashboard(w http.ResponseWriter, r *http.Request) {
	items, e := a.targets.List(r.Context(), target.StatusActive, 100)
	if e != nil {
		fail(w, 500, "internal_error", "could not load dashboard")
		return
	}
	var entities, reviewed int
	if e = a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM entities").Scan(&entities); e != nil {
		fail(w, 500, "internal_error", "could not load dashboard metrics")
		return
	}
	if e = a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM entities WHERE investigation_status IN ('reviewed','archived')").Scan(&reviewed); e != nil {
		fail(w, 500, "internal_error", "could not load dashboard metrics")
		return
	}
	write(w, 200, map[string]any{"activeTargetCount": len(items), "totalEntityCount": entities, "reviewedEntityCount": reviewed, "recentTargets": items})
}
func (a *API) listNotes(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	items, e := a.notes.List(r.Context(), r.PathValue("id"))
	if e != nil {
		fail(w, 500, "internal_error", "could not list notes")
		return
	}
	write(w, 200, map[string]any{"data": items})
}
func (a *API) createNote(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var in struct {
		Title string `json:"title"`
		Body  string `json:"bodyMarkdown"`
	}
	if e := decode(r, &in); e != nil || strings.TrimSpace(in.Title) == "" {
		fail(w, 400, "validation_failed", "note title is required")
		return
	}
	n, e := a.notes.Create(r.Context(), r.PathValue("id"), in.Title, in.Body)
	if e != nil {
		fail(w, 500, "internal_error", "could not create note")
		return
	}
	write(w, 201, n)
}

func (a *API) listRuns(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	runs, e := a.runs.List(r.Context(), r.PathValue("id"))
	if e != nil {
		fail(w, 500, "internal_error", "could not list runs")
		return
	}
	write(w, 200, map[string]any{"data": runs})
}
func (a *API) createRun(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var in collection.CreateInput
	if e := decode(r, &in); e != nil {
		fail(w, 400, "validation_failed", "invalid request body")
		return
	}
	run, e := a.runs.Start(r.Context(), r.PathValue("id"), in)
	if e != nil {
		fail(w, 400, "validation_failed", e.Error())
		return
	}
	_ = a.activity.Record(r.Context(), ptr(run.TargetID), "collection_started", "run", run.ID, "Collection run started")
	write(w, 202, run)
}
func (a *API) getRun(w http.ResponseWriter, r *http.Request) {
	run, e := a.runs.Get(r.Context(), r.PathValue("id"))
	if errors.Is(e, sql.ErrNoRows) {
		fail(w, 404, "not_found", "collection run not found")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error", "could not load collection run")
		return
	}
	write(w, 200, run)
}
func (a *API) cancelRun(w http.ResponseWriter, r *http.Request) {
	if !a.runs.Cancel(r.PathValue("id")) {
		fail(w, 409, "not_running", "collection run is not running")
		return
	}
	write(w, 202, map[string]string{"status": "cancelling"})
}
func (a *API) runEvents(w http.ResponseWriter, r *http.Request) {
	run, e := a.runs.Get(r.Context(), r.PathValue("id"))
	if errors.Is(e, sql.ErrNoRows) {
		fail(w, 404, "not_found", "collection run not found")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error", "could not load run")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, ok := w.(http.Flusher)
	if !ok {
		fail(w, 500, "stream_unsupported", "streaming unavailable")
		return
	}
	after := ""
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		events, e := a.runs.Events(r.Context(), run.ID, after)
		if e != nil {
			return
		}
		for _, event := range events {
			after = event.CreatedAt.Format(time.RFC3339Nano)
			b, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(w, "event: progress\ndata: %s\n\n", b)
			flusher.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
func (a *API) listEntities(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	typ := r.URL.Query().Get("type")
	status := r.URL.Query().Get("scopeStatus")
	investigationStatus := r.URL.Query().Get("investigationStatus")
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))
	technology := strings.TrimSpace(r.URL.Query().Get("technology"))
	observedAfter := strings.TrimSpace(r.URL.Query().Get("observedAfter"))
	minConfidence := strings.TrimSpace(r.URL.Query().Get("minConfidence"))
	cursor := r.URL.Query().Get("cursor")
	limit, e := boundedLimit(r, 50)
	if e != nil {
		fail(w, 400, "validation_failed", e.Error())
		return
	}
	q := "SELECT id,type,canonical_value,display_value,scope_status,first_seen_at,last_seen_at,investigation_status,bookmarked FROM entities WHERE target_id=?"
	args := []any{r.PathValue("id")}
	if typ != "" {
		q += " AND type=?"
		args = append(args, typ)
	}
	if status != "" {
		q += " AND scope_status=?"
		args = append(args, status)
	}
	if investigationStatus != "" {
		q += " AND investigation_status=?"
		args = append(args, investigationStatus)
	}
	if search != "" {
		q += " AND (lower(canonical_value) LIKE ? OR lower(display_value) LIKE ? OR id IN (SELECT entity_id FROM entity_search WHERE entity_search MATCH ?))"
		like := "%" + strings.ToLower(search) + "%"
		args = append(args, like, like, `"`+strings.ReplaceAll(search, `"`, "")+`"`)
	}
	if tag != "" {
		q += " AND EXISTS (SELECT 1 FROM entity_tags et JOIN tags tg ON tg.id=et.tag_id WHERE et.entity_id=entities.id AND tg.target_id=entities.target_id AND lower(tg.name)=lower(?))"
		args = append(args, tag)
	}
	if technology != "" {
		q += " AND EXISTS (SELECT 1 FROM relationships rt JOIN entities tech ON tech.id=rt.destination_entity_id WHERE rt.target_id=entities.target_id AND rt.source_entity_id=entities.id AND rt.relationship_type='USES_TECHNOLOGY' AND lower(tech.canonical_value)=lower(?))"
		args = append(args, technology)
	}
	if observedAfter != "" {
		if _, err := time.Parse(time.RFC3339, observedAfter); err != nil {
			fail(w, 400, "validation_failed", "observedAfter must be RFC3339")
			return
		}
		q += " AND last_seen_at>=?"
		args = append(args, observedAfter)
	}
	if minConfidence != "" {
		confidence, err := strconv.ParseFloat(minConfidence, 64)
		if err != nil || confidence < 0 || confidence > 1 {
			fail(w, 400, "validation_failed", "minConfidence must be between 0 and 1")
			return
		}
		q += " AND EXISTS (SELECT 1 FROM relationships rc WHERE rc.target_id=entities.target_id AND (rc.source_entity_id=entities.id OR rc.destination_entity_id=entities.id) AND rc.confidence>=?)"
		args = append(args, confidence)
	}
	if cursor != "" {
		q += " AND last_seen_at<?"
		args = append(args, cursor)
	}
	q += " ORDER BY last_seen_at DESC LIMIT ?"
	args = append(args, limit+1)
	rows, e := a.db.QueryContext(r.Context(), q, args...)
	if e != nil {
		fail(w, 500, "internal_error", "could not list entities")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	nextCursor := ""
	lastCursor := ""
	for rows.Next() {
		var id, t, c, d, s, first, last, investigation string
		var bookmarked int
		if e = rows.Scan(&id, &t, &c, &d, &s, &first, &last, &investigation, &bookmarked); e != nil {
			fail(w, 500, "internal_error", "could not read entity")
			return
		}
		if len(out) == limit {
			nextCursor = lastCursor
			break
		}
		out = append(out, map[string]any{"id": id, "type": t, "canonicalValue": c, "displayValue": d, "scopeStatus": s, "firstSeenAt": first, "lastSeenAt": last, "investigationStatus": investigation, "bookmarked": bookmarked == 1})
		lastCursor = last
	}
	write(w, 200, map[string]any{"data": out, "nextCursor": nextCursor})
}
func (a *API) getEntity(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var id, t, c, d, s, metadata, first, last, investigation string
	var bookmarked int
	e := a.db.QueryRowContext(r.Context(), "SELECT id,type,canonical_value,display_value,scope_status,metadata_json,first_seen_at,last_seen_at,investigation_status,bookmarked FROM entities WHERE target_id=? AND id=?", r.PathValue("id"), r.PathValue("entityId")).Scan(&id, &t, &c, &d, &s, &metadata, &first, &last, &investigation, &bookmarked)
	if errors.Is(e, sql.ErrNoRows) {
		fail(w, 404, "not_found", "entity not found")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error", "could not load entity")
		return
	}
	write(w, 200, map[string]any{"id": id, "type": t, "canonicalValue": c, "displayValue": d, "scopeStatus": s, "metadata": json.RawMessage(metadata), "firstSeenAt": first, "lastSeenAt": last, "investigationStatus": investigation, "bookmarked": bookmarked == 1})
}
func (a *API) listObservations(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	rows, e := a.db.QueryContext(r.Context(), "SELECT id,collector,observation_type,subject,value,source,evidence_id,observed_at,confidence,raw_metadata_json,origin FROM observations WHERE target_id=? ORDER BY observed_at DESC LIMIT 100", r.PathValue("id"))
	if e != nil {
		fail(w, 500, "internal_error", "could not list observations")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, c, t, subject, value, source, metadata, observed, origin string
		var evidence sql.NullString
		var confidence float64
		if e = rows.Scan(&id, &c, &t, &subject, &value, &source, &evidence, &observed, &confidence, &metadata, &origin); e != nil {
			fail(w, 500, "internal_error", "could not read observation")
			return
		}
		var evidenceID any
		if evidence.Valid {
			evidenceID = evidence.String
		}
		out = append(out, map[string]any{"id": id, "collector": c, "observationType": t, "subject": subject, "value": value, "source": source, "evidenceId": evidenceID, "observedAt": observed, "confidence": confidence, "metadata": json.RawMessage(metadata), "origin": origin})
	}
	write(w, 200, map[string]any{"data": out})
}
func (a *API) getEvidence(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var id, kind, source, hash string
	var content sql.NullString
	var size int
	var collected string
	e := a.db.QueryRowContext(r.Context(), "SELECT id,evidence_type,source,content_hash,content,content_size,collected_at FROM evidence WHERE target_id=? AND id=?", r.PathValue("id"), r.PathValue("evidenceId")).Scan(&id, &kind, &source, &hash, &content, &size, &collected)
	if errors.Is(e, sql.ErrNoRows) {
		fail(w, 404, "not_found", "evidence not found")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error", "could not load evidence")
		return
	}
	write(w, 200, map[string]any{"id": id, "type": kind, "source": source, "contentHash": hash, "content": content, "contentSize": size, "collectedAt": collected})
}
