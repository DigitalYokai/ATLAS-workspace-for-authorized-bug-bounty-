package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/atlas/atlas/backend/internal/collection"
	"github.com/atlas/atlas/backend/internal/investigation"
	"github.com/atlas/atlas/backend/internal/scope"
	"github.com/atlas/atlas/backend/internal/target"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

type API struct {
	targets *target.Service
	scopes  *scope.Repository
	notes   *investigation.Repository
	runs    *collection.Service
	log     *slog.Logger
}

func New(db *sql.DB, log *slog.Logger) http.Handler {
	sc := scope.NewRepository(db)
	tr := target.NewRepository(db)
	api := &API{targets: target.NewService(db, tr, sc), scopes: sc, notes: investigation.NewRepository(db), runs: collection.NewService(db, []collection.Collector{collection.DNSCollector{}, collection.HTTPMetadataCollector{}}), log: log}
	return api.routes()
}
func (a *API) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("GET /api/v1/dashboard", a.dashboard)
	m.HandleFunc("GET /api/v1/targets", a.listTargets)
	m.HandleFunc("POST /api/v1/targets", a.createTarget)
	m.HandleFunc("GET /api/v1/targets/{id}", a.getTarget)
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
	return middleware(m, a.log)
}
func middleware(next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
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
	write(w, 200, map[string]any{"activeTargetCount": len(items), "recentTargets": items})
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
