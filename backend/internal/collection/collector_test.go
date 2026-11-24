package collection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCTCollectorNormalizesAndDeduplicatesNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("output") != "json" {
			t.Error("missing json output")
		}
		_, _ = w.Write([]byte(`[{"name_value":"*.api.example.com\napi.example.com"},{"name_value":"api.example.com"},{"name_value":"other.example.net"}]`))
	}))
	defer server.Close()
	var got []Observation
	c := CTCollector{Client: server.Client(), Endpoint: server.URL}
	err := c.Collect(context.Background(), Request{PrimaryDomain: "example.com", Configuration: Configuration{TimeoutSeconds: 5}}, func(o Observation) error { got = append(got, o); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Value != "api.example.com" {
		t.Fatalf("unexpected observations: %#v", got)
	}
	if got[0].ObservedAt.IsZero() {
		t.Fatal("missing provenance timestamp")
	}
	_ = time.Second
}
