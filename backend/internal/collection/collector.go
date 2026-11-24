package collection

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/atlas/atlas/backend/internal/scope"
)

type Observation struct {
	EntityType string
	Value      string
	Source     string
	Confidence float64
	Metadata   string
	Evidence   string
	ObservedAt time.Time
}
type Request struct {
	TargetID, PrimaryDomain string
	Mode                    Mode
	Rules                   []scope.Rule
	Configuration           Configuration
}
type Collector interface {
	Name() string
	Active() bool
	Collect(context.Context, Request, func(Observation) error) error
}
type DNSLookup func(context.Context, string) ([]net.IP, error)

type DNSCollector struct{ Lookup DNSLookup }

func (c DNSCollector) Name() string { return "dns" }
func (c DNSCollector) Active() bool { return false }
func (c DNSCollector) Collect(ctx context.Context, r Request, emit func(Observation) error) error {
	lookup := c.Lookup
	if lookup == nil {
		lookup = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		}
	}
	ips, err := lookup(ctx, r.PrimaryDomain)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", r.PrimaryDomain, err)
	}
	for _, ip := range ips {
		if err := emit(Observation{EntityType: "ip_address", Value: ip.String(), Source: c.Name(), Confidence: 1, ObservedAt: time.Now().UTC(), Metadata: `{"record_type":"A_or_AAAA"}`}); err != nil {
			return err
		}
	}
	return nil
}

// HTTPMetadataCollector records headers and title-sized metadata only; it never executes response content.
type HTTPMetadataCollector struct {
	Client  *http.Client
	Resolve DNSLookup
}

func (c HTTPMetadataCollector) Name() string { return "http_metadata" }
func (c HTTPMetadataCollector) Active() bool { return true }
func (c HTTPMetadataCollector) Collect(ctx context.Context, r Request, emit func(Observation) error) error {
	if decision := scope.Evaluate(r.Rules, r.PrimaryDomain, "http_probe"); !decision.ActiveApproved {
		return fmt.Errorf("scope denied HTTP collection: %s", decision.Reason)
	}
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: time.Duration(r.Configuration.TimeoutSeconds) * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			d := scope.Evaluate(r.Rules, req.URL.Hostname(), "http_probe")
			if !d.ActiveApproved {
				return fmt.Errorf("redirect scope denied")
			}
			return nil
		}}
	}
	u := url.URL{Scheme: "https", Host: r.PrimaryDomain}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	server := strings.TrimSpace(resp.Header.Get("Server"))
	if server != "" {
		if err := emit(Observation{EntityType: "technology", Value: server, Source: c.Name(), Confidence: 1, ObservedAt: time.Now().UTC(), Metadata: `{"header":"Server","classification":"confirmed"}`}); err != nil {
			return err
		}
	}
	if powered := strings.TrimSpace(resp.Header.Get("X-Powered-By")); powered != "" {
		if err := emit(Observation{EntityType: "technology", Value: powered, Source: c.Name(), Confidence: .7, ObservedAt: time.Now().UTC(), Metadata: `{"header":"X-Powered-By","classification":"inferred"}`}); err != nil {
			return err
		}
	}
	return emit(Observation{EntityType: "url", Value: u.String(), Source: c.Name(), Confidence: 1, ObservedAt: time.Now().UTC(), Metadata: fmt.Sprintf(`{"status":%d}`, resp.StatusCode)})
}
