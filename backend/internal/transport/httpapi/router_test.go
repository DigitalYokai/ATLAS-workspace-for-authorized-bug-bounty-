package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/atlas/atlas/backend/internal/platform/database"
)

func newTestAPI(t *testing.T) http.Handler {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "atlas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
