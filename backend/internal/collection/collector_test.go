package collection

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atlas/atlas/backend/internal/scope"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHTTPCollectorRejectsOutOfScopeRedirectWithInjectedClient(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.Hostname() == "example.com" {
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://outside.example/"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		t.Fatalf("redirect request reached transport for %s", r.URL)
		return nil, nil
	})}
	collector := HTTPMetadataCollector{Client: client, Resolve: func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}}
	err := collector.Collect(context.Background(), Request{
		PrimaryDomain: "example.com",
		Configuration: Configuration{TimeoutSeconds: 1},
		Rules:         []scope.Rule{{Kind: "include", Pattern: "example.com", PatternKind: "exact", PermittedMethods: []string{"http_probe"}}},
	}, func(Observation) error { return nil })
	if err == nil {
		t.Fatal("expected redirect scope error")
	}
	if requests != 1 {
		t.Fatalf("requests=%d, want only original request", requests)
	}
}
