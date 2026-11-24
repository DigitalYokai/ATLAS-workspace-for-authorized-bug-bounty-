package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/atlas/atlas/backend/internal/correlation"
	"github.com/atlas/atlas/backend/internal/observation"
	"github.com/google/uuid"
)

func boundedLimit(r *http.Request, fallback int) (int, error) {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return fallback, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > 100 {
		return 0, errors.New("limit must be between 1 and 100")
	}
	return limit, nil
}

func (a *API) updateEntityInvestigation(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var input struct {
		Status     *string `json:"status"`
		Bookmarked *bool   `json:"bookmarked"`
	}
	if err := decode(r, &input); err != nil {
		fail(w, 400, "validation_failed", "invalid request body")
		return
	}
	if input.Status == nil && input.Bookmarked == nil {
		fail(w, 400, "validation_failed", "provide status or bookmarked")
		return
	}
	if input.Status != nil && !map[string]bool{"unreviewed": true, "reviewing": true, "reviewed": true, "needs_follow_up": true, "archived": true}[*input.Status] {
		fail(w, 400, "validation_failed", "invalid investigation status")
		return
	}
	var status string
	var bookmarked int
	err := a.db.QueryRowContext(r.Context(), "SELECT investigation_status,bookmarked FROM entities WHERE target_id=? AND id=?", r.PathValue("id"), r.PathValue("entityId")).Scan(&status, &bookmarked)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "not_found", "entity not found")
		return
	}
	if err != nil {
		fail(w, 500, "internal_error", "could not load entity")
		return
	}
	if input.Status != nil {
		status = *input.Status
	}
	if input.Bookmarked != nil {
		if *input.Bookmarked {
			bookmarked = 1
		} else {
			bookmarked = 0
		}
	}
	_, err = a.db.ExecContext(r.Context(), "UPDATE entities SET investigation_status=?,bookmarked=?,updated_at=? WHERE target_id=? AND id=?", status, bookmarked, time.Now().UTC().Format(time.RFC3339Nano), r.PathValue("id"), r.PathValue("entityId"))
	if err != nil {
		fail(w, 500, "internal_error", "could not update investigation state")
		return
	}
	write(w, 200, map[string]any{"status": status, "bookmarked": bookmarked == 1})
}

func (a *API) entityRelationships(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	limit, err := boundedLimit(r, 50)
	if err != nil {
		fail(w, 400, "validation_failed", err.Error())
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT rel.id,rel.target_id,rel.source_entity_id,rel.destination_entity_id,rel.relationship_type,rel.confidence,rel.first_seen_at,rel.last_seen_at,rel.status,rel.assertion_kind,
	CASE WHEN rel.source_entity_id=? THEN destination.id ELSE source.id END,
	CASE WHEN rel.source_entity_id=? THEN destination.type ELSE source.type END,
	CASE WHEN rel.source_entity_id=? THEN destination.display_value ELSE source.display_value END
	FROM relationships rel JOIN entities source ON source.id=rel.source_entity_id JOIN entities destination ON destination.id=rel.destination_entity_id
	WHERE rel.target_id=? AND (? IN (rel.source_entity_id,rel.destination_entity_id)) ORDER BY rel.last_seen_at DESC LIMIT ?`, r.PathValue("entityId"), r.PathValue("entityId"), r.PathValue("entityId"), r.PathValue("id"), r.PathValue("entityId"), limit)
	if err != nil {
		fail(w, 500, "internal_error", "could not load relationships")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var rel correlation.Relationship
		var first, last string
		var relatedID, relatedType, relatedValue string
		if err = rows.Scan(&rel.ID, &rel.TargetID, &rel.SourceEntityID, &rel.DestinationEntityID, &rel.RelationshipType, &rel.Confidence, &first, &last, &rel.Status, &rel.AssertionKind, &relatedID, &relatedType, &relatedValue); err != nil {
			fail(w, 500, "internal_error", "could not read relationship")
			return
		}
		rel.FirstSeenAt, _ = time.Parse(time.RFC3339Nano, first)
		rel.LastSeenAt, _ = time.Parse(time.RFC3339Nano, last)
		out = append(out, map[string]any{"relationship": rel, "relatedEntity": map[string]string{"id": relatedID, "type": relatedType, "displayValue": relatedValue}})
	}
	write(w, 200, map[string]any{"data": out})
}

func (a *API) entityObservations(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var canonical string
	err := a.db.QueryRowContext(r.Context(), "SELECT canonical_value FROM entities WHERE target_id=? AND id=?", r.PathValue("id"), r.PathValue("entityId")).Scan(&canonical)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "not_found", "entity not found")
		return
	}
	if err != nil {
		fail(w, 500, "internal_error", "could not load entity")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT id,collector,observation_type,subject,value,source,evidence_id,observed_at,confidence,raw_metadata_json,origin FROM observations WHERE target_id=? AND (lower(value)=? OR lower(subject)=?) ORDER BY observed_at DESC LIMIT 100", r.PathValue("id"), canonical, canonical)
	if err != nil {
		fail(w, 500, "internal_error", "could not load observations")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, c, t, subject, value, source, observed, metadata, origin string
		var evidence sql.NullString
		var confidence float64
		if err = rows.Scan(&id, &c, &t, &subject, &value, &source, &evidence, &observed, &confidence, &metadata, &origin); err != nil {
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

func (a *API) addEntityTag(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var input struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := decode(r, &input); err != nil || strings.TrimSpace(input.Name) == "" {
		fail(w, 400, "validation_failed", "tag name is required")
		return
	}
	var exists int
	err := a.db.QueryRowContext(r.Context(), "SELECT 1 FROM entities WHERE target_id=? AND id=?", r.PathValue("id"), r.PathValue("entityId")).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "not_found", "entity not found")
		return
	}
	if err != nil {
		fail(w, 500, "internal_error", "could not load entity")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tagID := uuid.NewString()
	_, err = a.db.ExecContext(r.Context(), "INSERT INTO tags(id,target_id,name,color,created_at) VALUES(?,?,?,?,?) ON CONFLICT(target_id,name) DO NOTHING", tagID, r.PathValue("id"), strings.TrimSpace(input.Name), input.Color, now)
	if err != nil {
		fail(w, 500, "internal_error", "could not create tag")
		return
	}
	if err = a.db.QueryRowContext(r.Context(), "SELECT id FROM tags WHERE target_id=? AND name=?", r.PathValue("id"), strings.TrimSpace(input.Name)).Scan(&tagID); err != nil {
		fail(w, 500, "internal_error", "could not load tag")
		return
	}
	_, err = a.db.ExecContext(r.Context(), "INSERT INTO entity_tags(target_id,entity_id,tag_id,created_at) VALUES(?,?,?,?) ON CONFLICT(entity_id,tag_id) DO NOTHING", r.PathValue("id"), r.PathValue("entityId"), tagID, now)
	if err != nil {
		fail(w, 500, "internal_error", "could not tag entity")
		return
	}
	write(w, 201, map[string]string{"id": tagID, "name": strings.TrimSpace(input.Name)})
}

func (a *API) graph(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	entityID := r.URL.Query().Get("entityId")
	if entityID == "" {
		fail(w, 400, "validation_failed", "entityId is required")
		return
	}
	limit, err := boundedLimit(r, 50)
	if err != nil {
		fail(w, 400, "validation_failed", err.Error())
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT id,target_id,source_entity_id,destination_entity_id,relationship_type,confidence,first_seen_at,last_seen_at,status,assertion_kind FROM relationships WHERE target_id=? AND (? IN (source_entity_id,destination_entity_id)) ORDER BY last_seen_at DESC LIMIT ?", r.PathValue("id"), entityID, limit)
	if err != nil {
		fail(w, 500, "internal_error", "could not load graph")
		return
	}
	defer rows.Close()
	edges := []correlation.Relationship{}
	nodeIDs := map[string]bool{}
	for rows.Next() {
		var edge correlation.Relationship
		var first, last string
		if err = rows.Scan(&edge.ID, &edge.TargetID, &edge.SourceEntityID, &edge.DestinationEntityID, &edge.RelationshipType, &edge.Confidence, &first, &last, &edge.Status, &edge.AssertionKind); err != nil {
			fail(w, 500, "internal_error", "could not read graph")
			return
		}
		edge.FirstSeenAt, _ = time.Parse(time.RFC3339Nano, first)
		edge.LastSeenAt, _ = time.Parse(time.RFC3339Nano, last)
		edges = append(edges, edge)
		nodeIDs[edge.SourceEntityID] = true
		nodeIDs[edge.DestinationEntityID] = true
	}
	nodes := []correlation.Node{}
	for id := range nodeIDs {
		var n correlation.Node
		if err = a.db.QueryRowContext(r.Context(), "SELECT id,type,display_value FROM entities WHERE target_id=? AND id=?", r.PathValue("id"), id).Scan(&n.ID, &n.Type, &n.DisplayValue); err == nil {
			nodes = append(nodes, n)
		}
	}
	write(w, 200, correlation.Graph{Nodes: nodes, Edges: edges})
}

func (a *API) createManualObservation(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var input observation.Input
	if err := decode(r, &input); err != nil {
		fail(w, 400, "validation_failed", "invalid request body")
		return
	}
	input.Origin = "manual"
	if input.Source == "" {
		input.Source = "manual"
	}
	if input.ObservedAt.IsZero() {
		input.ObservedAt = time.Now().UTC()
	}
	if input.Metadata == "" {
		input.Metadata = `{}`
	}
	if err := a.ingest.Ingest(r.Context(), r.PathValue("id"), "", input); err != nil {
		fail(w, 400, "validation_failed", err.Error())
		return
	}
	write(w, 201, map[string]string{"status": "created"})
}

func (a *API) createManualRelationship(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var input struct {
		SourceEntityID      string  `json:"sourceEntityId"`
		DestinationEntityID string  `json:"destinationEntityId"`
		RelationshipType    string  `json:"relationshipType"`
		AssertionKind       string  `json:"assertionKind"`
		Confidence          float64 `json:"confidence"`
		Rationale           string  `json:"rationale"`
	}
	if err := decode(r, &input); err != nil {
		fail(w, 400, "validation_failed", "invalid request body")
		return
	}
	if !correlation.RelationshipTypes[input.RelationshipType] || (input.AssertionKind != "manual" && input.AssertionKind != "hypothesis") || strings.TrimSpace(input.Rationale) == "" || input.Confidence < 0 || input.Confidence > 1 {
		fail(w, 400, "validation_failed", "invalid manual relationship")
		return
	}
	for _, entityID := range []string{input.SourceEntityID, input.DestinationEntityID} {
		var count int
		if err := a.db.QueryRowContext(r.Context(), "SELECT count(*) FROM entities WHERE target_id=? AND id=?", r.PathValue("id"), entityID).Scan(&count); err != nil || count != 1 {
			fail(w, 400, "validation_failed", "relationship entities must belong to target")
			return
		}
	}
	now := time.Now().UTC()
	relationshipID := uuid.NewString()
	_, err := a.db.ExecContext(r.Context(), "INSERT INTO relationships(id,target_id,source_entity_id,destination_entity_id,relationship_type,confidence,first_seen_at,last_seen_at,status,assertion_kind,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,'current',?,?,?) ON CONFLICT(target_id,source_entity_id,destination_entity_id,relationship_type) DO UPDATE SET confidence=excluded.confidence,assertion_kind=excluded.assertion_kind,last_seen_at=excluded.last_seen_at,status='current',updated_at=excluded.updated_at", relationshipID, r.PathValue("id"), input.SourceEntityID, input.DestinationEntityID, input.RelationshipType, input.Confidence, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), input.AssertionKind, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		fail(w, 500, "internal_error", "could not create relationship")
		return
	}
	if err = a.db.QueryRowContext(r.Context(), "SELECT id FROM relationships WHERE target_id=? AND source_entity_id=? AND destination_entity_id=? AND relationship_type=?", r.PathValue("id"), input.SourceEntityID, input.DestinationEntityID, input.RelationshipType).Scan(&relationshipID); err != nil {
		fail(w, 500, "internal_error", "could not load relationship")
		return
	}
	_, err = a.db.ExecContext(r.Context(), "INSERT INTO relationship_evidence(id,target_id,relationship_id,rationale,created_at) VALUES(?,?,?,?,?)", uuid.NewString(), r.PathValue("id"), relationshipID, input.Rationale, now.Format(time.RFC3339Nano))
	if err != nil {
		fail(w, 500, "internal_error", "could not record relationship rationale")
		return
	}
	write(w, 201, map[string]string{"id": relationshipID})
}

func (a *API) listSnapshots(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	items, err := a.correlation.ListSnapshots(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, 500, "internal_error", "could not list snapshots")
		return
	}
	write(w, 200, map[string]any{"data": items})
}
func (a *API) createSnapshot(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if err := decode(r, &input); err != nil {
		fail(w, 400, "validation_failed", "invalid request body")
		return
	}
	snapshot, err := a.correlation.CreateManualSnapshot(r.Context(), r.PathValue("id"), input.Name)
	if err != nil {
		fail(w, 500, "internal_error", "could not create snapshot")
		return
	}
	write(w, 201, snapshot)
}
func (a *API) compareSnapshots(w http.ResponseWriter, r *http.Request) {
	if !a.ensureTarget(w, r) {
		return
	}
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if from == "" || to == "" {
		fail(w, 400, "validation_failed", "from and to snapshots are required")
		return
	}
	comparison, err := a.correlation.CompareSnapshots(r.Context(), r.PathValue("id"), from, to)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "not_found", "snapshot not found")
		return
	}
	if err != nil {
		fail(w, 500, "internal_error", "could not compare snapshots")
		return
	}
	write(w, 200, comparison)
}
