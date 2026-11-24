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
