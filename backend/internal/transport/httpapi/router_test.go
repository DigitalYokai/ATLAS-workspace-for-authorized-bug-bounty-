package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/atlas/atlas/backend/internal/platform/database"
)

func newTestAPI(t *testing.T) http.Handler {
	t.Helper()
	h, _ := newTestAPIWithDB(t)
	return h
}
func newTestAPIWithDB(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "atlas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(db, slog.New(slog.NewTextHandler(io.Discard, nil))), db
}
func request(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestTargetCreationPersistsAndRejectsInvalidInput(t *testing.T) {
	h := newTestAPI(t)
	bad := request(t, h, http.MethodPost, "/api/v1/targets", `{"name":"Missing domain"}`)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid request status=%d", bad.Code)
	}
	created := request(t, h, http.MethodPost, "/api/v1/targets", `{"name":"Example","primaryDomain":"example.com","scope":[{"kind":"include","pattern":"*.example.com","permittedMethods":["dns_lookup"]}]}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var target struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &target); err != nil || target.ID == "" {
		t.Fatalf("decode target: %v", err)
	}
	got := request(t, h, http.MethodGet, "/api/v1/targets/"+target.ID, "")
	if got.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", got.Code, got.Body.String())
	}
}

func TestScopeHTTPExclusionPrecedence(t *testing.T) {
	h := newTestAPI(t)
	created := request(t, h, http.MethodPost, "/api/v1/targets", `{"name":"Example","primaryDomain":"example.com","scope":[{"kind":"include","pattern":"*.example.com","permittedMethods":["dns_lookup"]},{"kind":"exclude","pattern":"payments.example.com","permittedMethods":[]}]}`)
	var target struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &target)
	result := request(t, h, http.MethodPost, "/api/v1/targets/"+target.ID+"/scope/validate", `{"host":"payments.example.com","method":"dns_lookup"}`)
	if result.Code != http.StatusOK || !bytes.Contains(result.Body.Bytes(), []byte(`"status":"excluded"`)) {
		t.Fatalf("scope result status=%d body=%s", result.Code, result.Body.String())
	}
}

func TestObservationsSerializeMissingEvidenceAsNull(t *testing.T) {
	h, db := newTestAPIWithDB(t)
	created := request(t, h, http.MethodPost, "/api/v1/targets", `{"name":"Example","primaryDomain":"example.com"}`)
	var target struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &target); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := db.Exec("INSERT INTO observations(id,target_id,collector,observation_type,subject,value,source,evidence_id,observed_at,confidence,raw_metadata_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", "observation", target.ID, "fixture", "technology", "fixture", "fixture", "fixture", nil, now, 1, `{}`, now)
	if err != nil {
		t.Fatal(err)
	}
	response := request(t, h, http.MethodGet, "/api/v1/targets/"+target.ID+"/observations", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data []struct {
			EvidenceID *string `json:"evidenceId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 || body.Data[0].EvidenceID != nil {
		t.Fatalf("unexpected evidence response: %s", response.Body.String())
	}
}

func TestManualObservationIsSeparatedAndTargetDataIsIsolated(t *testing.T) {
	h := newTestAPI(t)
	create := func(name, domain string) string {
		response := request(t, h, http.MethodPost, "/api/v1/targets", `{"name":"`+name+`","primaryDomain":"`+domain+`"}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
		}
		var target struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &target); err != nil {
			t.Fatal(err)
		}
		return target.ID
	}
	first, second := create("First", "example.com"), create("Second", "other.example.com")
	created := request(t, h, http.MethodPost, "/api/v1/targets/"+first+"/observations", `{"entityType":"technology","observationType":"manual","subject":"example.com","value":"nginx","source":"manual","confidence":1,"metadata":"{}"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("manual observation status=%d body=%s", created.Code, created.Body.String())
	}
	firstObservations := request(t, h, http.MethodGet, "/api/v1/targets/"+first+"/observations", "")
	if firstObservations.Code != http.StatusOK || !bytes.Contains(firstObservations.Body.Bytes(), []byte(`"origin":"manual"`)) {
		t.Fatalf("manual provenance missing: %s", firstObservations.Body.String())
	}
	secondObservations := request(t, h, http.MethodGet, "/api/v1/targets/"+second+"/observations", "")
	if secondObservations.Code != http.StatusOK || bytes.Contains(secondObservations.Body.Bytes(), []byte("nginx")) {
		t.Fatalf("target isolation failed: %s", secondObservations.Body.String())
	}
}

func TestEntityTagsCanBeAddedAndListed(t *testing.T) {
	h, db := newTestAPIWithDB(t)
	created := request(t, h, http.MethodPost, "/api/v1/targets", `{"name":"Example","primaryDomain":"example.com"}`)
	var target struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &target); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec("INSERT INTO entities(id,target_id,type,canonical_value,display_value,metadata_json,scope_status,first_seen_at,last_seen_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", "entity", target.ID, "domain", "example.com", "example.com", `{}`, "included", now, now, now, now); err != nil {
		t.Fatal(err)
	}
	added := request(t, h, http.MethodPost, "/api/v1/targets/"+target.ID+"/entities/entity/tags", `{"name":"priority"}`)
	if added.Code != http.StatusCreated {
		t.Fatalf("tag status=%d body=%s", added.Code, added.Body.String())
	}
	listed := request(t, h, http.MethodGet, "/api/v1/targets/"+target.ID+"/entities/entity/tags", "")
	if listed.Code != http.StatusOK || !bytes.Contains(listed.Body.Bytes(), []byte("priority")) {
		t.Fatalf("tags response=%s", listed.Body.String())
	}
}

func TestGraphRespectsRelationshipLimitAndFilters(t *testing.T) {
	h, db := newTestAPIWithDB(t)
	created := request(t, h, http.MethodPost, "/api/v1/targets", `{"name":"Example","primaryDomain":"example.com"}`)
	var target struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &target); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, entity := range []struct{ id, typ, value string }{{"root", "domain", "example.com"}, {"subdomain", "subdomain", "api.example.com"}, {"ip", "ip_address", "192.0.2.10"}} {
		if _, err := db.Exec("INSERT INTO entities(id,target_id,type,canonical_value,display_value,metadata_json,scope_status,first_seen_at,last_seen_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", entity.id, target.ID, entity.typ, entity.value, entity.value, `{}`, "included", now, now, now, now); err != nil {
			t.Fatal(err)
		}
	}
	for _, edge := range []struct{ id, destination, typ string }{{"edge-one", "subdomain", "HAS_SUBDOMAIN"}, {"edge-two", "ip", "RESOLVES_TO"}} {
		if _, err := db.Exec("INSERT INTO relationships(id,target_id,source_entity_id,destination_entity_id,relationship_type,confidence,first_seen_at,last_seen_at,status,assertion_kind,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", edge.id, target.ID, "root", edge.destination, edge.typ, 1, now, now, "current", "observed", now, now); err != nil {
			t.Fatal(err)
		}
	}
	limited := request(t, h, http.MethodGet, "/api/v1/targets/"+target.ID+"/graph?rootEntityId=root&limit=1", "")
	if limited.Code != http.StatusOK || !bytes.Contains(limited.Body.Bytes(), []byte(`"truncated":true`)) {
		t.Fatalf("limited graph=%d %s", limited.Code, limited.Body.String())
	}
	filtered := request(t, h, http.MethodGet, "/api/v1/targets/"+target.ID+"/graph?rootEntityId=root&relationshipTypes=HAS_SUBDOMAIN", "")
	if filtered.Code != http.StatusOK || bytes.Contains(filtered.Body.Bytes(), []byte("RESOLVES_TO")) || !bytes.Contains(filtered.Body.Bytes(), []byte("HAS_SUBDOMAIN")) {
		t.Fatalf("filtered graph=%d %s", filtered.Code, filtered.Body.String())
	}
}

func TestTargetExportIncludesObservationProvenance(t *testing.T) {
	h := newTestAPI(t)
	created := request(t, h, http.MethodPost, "/api/v1/targets", `{"name":"Example","primaryDomain":"example.com"}`)
	var target struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &target); err != nil {
		t.Fatal(err)
	}
	observation := request(t, h, http.MethodPost, "/api/v1/targets/"+target.ID+"/observations", `{"entityType":"technology","observationType":"manual","subject":"example.com","value":"nginx","source":"analyst","confidence":0.8,"metadata":"{}"}`)
	if observation.Code != http.StatusCreated {
		t.Fatalf("observation=%d %s", observation.Code, observation.Body.String())
	}
	exported := request(t, h, http.MethodGet, "/api/v1/targets/"+target.ID+"/exports/json", "")
	if exported.Code != http.StatusOK || !bytes.Contains(exported.Body.Bytes(), []byte(`"source":"analyst"`)) || !bytes.Contains(exported.Body.Bytes(), []byte(`"confidence":0.8`)) || !bytes.Contains(exported.Body.Bytes(), []byte(`"scopeStatus"`)) {
		t.Fatalf("export=%d %s", exported.Code, exported.Body.String())
	}
}

func TestBackupDownloadProducesSQLiteDatabase(t *testing.T) {
	h := newTestAPI(t)
	response := request(t, h, http.MethodGet, "/api/v1/settings/backup", "")
	if response.Code != http.StatusOK {
		t.Fatalf("backup status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Body.Len() < 100 || response.Header().Get("Content-Disposition") == "" {
		t.Fatalf("invalid backup response: bytes=%d headers=%v", response.Body.Len(), response.Header())
	}
}
