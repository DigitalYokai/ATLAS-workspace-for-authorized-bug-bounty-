package correlation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/atlas/atlas/backend/internal/scope"
	"github.com/google/uuid"
)

type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service {
	s := &Service{db: db}
	_, _ = db.Exec("UPDATE correlation_jobs SET status='pending',started_at=NULL,updated_at=? WHERE status='running'", time.Now().UTC().Format(time.RFC3339Nano))
	s.repairMalformedLegacyDNSRelations()
	go s.resumePending()
	go s.backfillHistoricalRuns()
	return s
}

// repairMalformedLegacyDNSRelations removes a transient Phase 3.0 backfill
// artefact where an old DNS observation used its IP as a domain subject.
func (s *Service) repairMalformedLegacyDNSRelations() {
	rows, err := s.db.Query(`SELECT source.id FROM relationships r
		JOIN entities source ON source.id=r.source_entity_id
		JOIN entities destination ON destination.id=r.destination_entity_id
		WHERE r.relationship_type='RESOLVES_TO' AND source.type='domain' AND destination.type='ip_address' AND source.canonical_value=destination.canonical_value`)
	if err != nil {
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	_ = rows.Close()
	for _, id := range ids {
		_, _ = s.db.Exec("DELETE FROM relationships WHERE source_entity_id=? AND relationship_type='RESOLVES_TO'", id)
		_, _ = s.db.Exec("DELETE FROM entities WHERE id=? AND NOT EXISTS (SELECT 1 FROM relationships WHERE source_entity_id=? OR destination_entity_id=?)", id, id, id)
	}
}

func (s *Service) resumePending() {
	rows, err := s.db.Query("SELECT id FROM correlation_jobs WHERE status='pending' ORDER BY created_at")
	if err != nil {
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	_ = rows.Close()
	for _, id := range ids {
		go s.process(context.Background(), id)
	}
}

// backfillHistoricalRuns lets Phase 3 build relationships from observations
// collected before correlation was introduced. Partial runs are safe to
// correlate, but only fully completed runs can create automatic snapshots.
func (s *Service) backfillHistoricalRuns() {
	rows, err := s.db.Query("SELECT target_id,id FROM collection_runs WHERE status IN ('completed','partially_completed') ORDER BY created_at")
	if err != nil {
		return
	}
	pending := [][2]string{}
	for rows.Next() {
		var targetID, runID string
		if rows.Scan(&targetID, &runID) == nil {
			pending = append(pending, [2]string{targetID, runID})
		}
	}
	_ = rows.Close()
	for _, run := range pending {
		_ = s.QueueRun(context.Background(), run[0], run[1])
	}
}

func (s *Service) QueueRun(ctx context.Context, targetID, runID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, "INSERT INTO correlation_jobs(id,target_id,collection_run_id,status,created_at,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(collection_run_id) DO UPDATE SET status='pending',error_summary='',started_at=NULL,completed_at=NULL,updated_at=excluded.updated_at", uuid.NewString(), targetID, runID, "pending", now, now)
	if err != nil {
		return err
	}
	var id string
	if err = s.db.QueryRowContext(ctx, "SELECT id FROM correlation_jobs WHERE collection_run_id=?", runID).Scan(&id); err != nil {
		return err
	}
	go s.process(context.Background(), id)
	return nil
}

func (s *Service) process(ctx context.Context, jobID string) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, "UPDATE correlation_jobs SET status='running',started_at=?,updated_at=? WHERE id=? AND status='pending'", now, now, jobID)
	if err != nil {
		return
	}
	claimed, _ := result.RowsAffected()
	if claimed != 1 {
		return
	}
	var targetID, runID string
	if err = s.db.QueryRowContext(ctx, "SELECT target_id,collection_run_id FROM correlation_jobs WHERE id=?", jobID).Scan(&targetID, &runID); err != nil {
		return
	}
	err = s.CorrelateRun(ctx, targetID, runID)
	if err != nil {
		_, _ = s.db.ExecContext(context.Background(), "UPDATE correlation_jobs SET status='failed',error_summary=?,completed_at=?,updated_at=? WHERE id=?", err.Error(), time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), jobID)
		return
	}
	finished := time.Now().UTC().Format(time.RFC3339Nano)
	_, _ = s.db.ExecContext(context.Background(), "UPDATE correlation_jobs SET status='completed',completed_at=?,updated_at=? WHERE id=?", finished, finished, jobID)
	var status string
	if s.db.QueryRowContext(ctx, "SELECT status FROM collection_runs WHERE id=?", runID).Scan(&status) == nil && status == "completed" {
		_ = s.CreateAutomaticSnapshot(context.Background(), targetID, runID)
	}
}

func (s *Service) CorrelateRun(ctx context.Context, targetID, runID string) error {
	var primary string
	if err := s.db.QueryRowContext(ctx, "SELECT primary_domain FROM targets WHERE id=?", targetID).Scan(&primary); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,collector,observation_type,subject,value,evidence_id,observed_at,confidence FROM observations WHERE target_id=? AND collection_run_id=? ORDER BY observed_at", targetID, runID)
	if err != nil {
		return err
	}
	type persistedObservation struct {
		id, collector, typ, subject, value, observed string
		evidence                                     sql.NullString
		confidence                                   float64
	}
	observations := []persistedObservation{}
	for rows.Next() {
		var item persistedObservation
		if err = rows.Scan(&item.id, &item.collector, &item.typ, &item.subject, &item.value, &item.evidence, &item.observed, &item.confidence); err != nil {
			rows.Close()
			return err
		}
		observations = append(observations, item)
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, item := range observations {
		at, _ := time.Parse(time.RFC3339Nano, item.observed)
		sourceValue := primary
		if net.ParseIP(item.subject) == nil {
			if host, normalizeErr := scope.NormalizeHost(item.subject); normalizeErr == nil {
				sourceValue = host
			}
		}
		var sourceType, destinationType, relationshipType, assertion string
		switch {
		case item.collector == "dns" || item.typ == "dns_resolution":
			sourceType, destinationType, relationshipType, assertion = "domain", "ip_address", ResolvesTo, "observed"
		case item.collector == "certificate_transparency" || item.typ == "certificate_transparency":
			sourceType, destinationType, relationshipType, assertion = "domain", "subdomain", HasSubdomain, "inferred"
		case item.collector == "http_metadata" && (item.typ == "technology" || item.typ == "http_technology"):
			sourceType, destinationType, relationshipType, assertion = "domain", "technology", UsesTechnology, "observed"
		case item.collector == "http_metadata" && (item.typ == "url" || item.typ == "http_endpoint"):
			sourceType, destinationType, relationshipType, assertion = "domain", "url", HasEndpoint, "observed"
		case item.collector == "tls_metadata" || item.typ == "certificate":
			sourceType, destinationType, relationshipType, assertion = "domain", "certificate", PresentsCertificate, "observed"
		default:
			continue
		}
		sourceID, err := s.ensureEntity(ctx, targetID, sourceType, sourceValue, at)
		if err != nil {
			return err
		}
		destinationID, err := s.findEntity(ctx, targetID, destinationType, item.value)
		if err != nil {
			continue
		}
		if err = s.upsertRelationship(ctx, targetID, sourceID, destinationID, relationshipType, assertion, item.confidence, at, item.id, item.evidence); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureEntity(ctx context.Context, targetID, typ, value string, at time.Time) (string, error) {
	canonical, err := canonical(typ, value)
	if err != nil {
		return "", err
	}
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, "INSERT INTO entities(id,target_id,type,canonical_value,display_value,metadata_json,scope_status,first_seen_at,last_seen_at,created_at,updated_at) VALUES(?,?,?,?,?,'{}','unknown',?,?,?,?) ON CONFLICT(target_id,type,canonical_value) DO UPDATE SET last_seen_at=excluded.last_seen_at,updated_at=excluded.updated_at", id, targetID, typ, canonical, value, at.Format(time.RFC3339Nano), at.Format(time.RFC3339Nano), now, now)
	if err != nil {
		return "", err
	}
	return s.findEntity(ctx, targetID, typ, value)
}

func (s *Service) findEntity(ctx context.Context, targetID, typ, value string) (string, error) {
	canonical, err := canonical(typ, value)
	if err != nil {
		return "", err
	}
	var id string
	err = s.db.QueryRowContext(ctx, "SELECT id FROM entities WHERE target_id=? AND type=? AND canonical_value=?", targetID, typ, canonical).Scan(&id)
	return id, err
}

func (s *Service) upsertRelationship(ctx context.Context, targetID, sourceID, destinationID, typ, assertion string, confidence float64, observed time.Time, observationID string, evidence sql.NullString) error {
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, "INSERT INTO relationships(id,target_id,source_entity_id,destination_entity_id,relationship_type,confidence,first_seen_at,last_seen_at,status,assertion_kind,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,'current',?,?,?) ON CONFLICT(target_id,source_entity_id,destination_entity_id,relationship_type) DO UPDATE SET confidence=MAX(relationships.confidence,excluded.confidence),first_seen_at=MIN(relationships.first_seen_at,excluded.first_seen_at),last_seen_at=MAX(relationships.last_seen_at,excluded.last_seen_at),status='current',updated_at=excluded.updated_at", id, targetID, sourceID, destinationID, typ, confidence, observed.Format(time.RFC3339Nano), observed.Format(time.RFC3339Nano), assertion, now, now)
	if err != nil {
		return err
	}
	var relationshipID string
	if err = s.db.QueryRowContext(ctx, "SELECT id FROM relationships WHERE target_id=? AND source_entity_id=? AND destination_entity_id=? AND relationship_type=?", targetID, sourceID, destinationID, typ).Scan(&relationshipID); err != nil {
		return err
	}
	evidenceID := any(nil)
	if evidence.Valid {
		evidenceID = evidence.String
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO relationship_evidence(id,target_id,relationship_id,observation_id,evidence_id,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(relationship_id,observation_id) DO NOTHING", uuid.NewString(), targetID, relationshipID, observationID, evidenceID, now)
	return err
}

func canonical(typ, value string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", fmt.Errorf("empty %s value", typ)
	}
	switch typ {
	case "domain", "subdomain":
		return scope.NormalizeHost(v)
	case "url":
		// Existing ingestion guarantees a non-empty URL; correlation keeps a
		// stable, conservative key without attempting fuzzy URL matching.
		return strings.ToLower(strings.TrimSuffix(v, "#")), nil
	default:
		return strings.ToLower(strings.Join(strings.Fields(v), " ")), nil
	}
}

func hash(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])
}
