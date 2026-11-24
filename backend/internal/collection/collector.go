package collection

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/atlas/atlas/backend/internal/platform/network"
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
	var resolver network.Resolver = network.DefaultResolver{}
	if c.Resolve != nil {
		resolver = resolverAdapter{lookup: c.Resolve}
	}
	if err := network.PublicDestination(ctx, resolver, r.PrimaryDomain); err != nil {
		return err
	}
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: time.Duration(r.Configuration.TimeoutSeconds) * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			d := scope.Evaluate(r.Rules, req.URL.Hostname(), "http_probe")
			if !d.ActiveApproved || network.PublicDestination(req.Context(), resolver, req.URL.Hostname()) != nil {
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

type resolverAdapter struct{ lookup DNSLookup }

func (r resolverAdapter) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return r.lookup(ctx, host)
}

type TLSCollector struct{ Resolve DNSLookup }

func (c TLSCollector) Name() string { return "tls_metadata" }
func (c TLSCollector) Active() bool { return true }
func (c TLSCollector) Collect(ctx context.Context, r Request, emit func(Observation) error) error {
	if d := scope.Evaluate(r.Rules, r.PrimaryDomain, "tls_handshake"); !d.ActiveApproved {
		return fmt.Errorf("scope denied TLS collection: %s", d.Reason)
	}
	var resolver network.Resolver = network.DefaultResolver{}
	if c.Resolve != nil {
		resolver = resolverAdapter{lookup: c.Resolve}
	}
	if err := network.PublicDestination(ctx, resolver, r.PrimaryDomain); err != nil {
		return err
	}
	conn, err := tls.DialWithDialer(&net.Dialer{}, "tcp", net.JoinHostPort(r.PrimaryDomain, "443"), &tls.Config{ServerName: r.PrimaryDomain, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.HandshakeContext(ctx); err != nil {
		return err
	}
	cert := conn.ConnectionState().PeerCertificates[0]
	meta, _ := json.Marshal(map[string]any{"issuer": cert.Issuer.String(), "not_before": cert.NotBefore.UTC().Format(time.RFC3339), "not_after": cert.NotAfter.UTC().Format(time.RFC3339), "dns_names": cert.DNSNames})
	return emit(Observation{EntityType: "certificate", Value: cert.SerialNumber.String(), Source: c.Name(), Confidence: 1, Metadata: string(meta), ObservedAt: time.Now().UTC()})
}

// CTCollector is registered for passive collection. Its HTTP client is injected for fixture-backed tests.
type CTCollector struct {
	Client   *http.Client
	Endpoint string
}

func (c CTCollector) Name() string { return "certificate_transparency" }
func (c CTCollector) Active() bool { return false }
func (c CTCollector) Collect(ctx context.Context, r Request, emit func(Observation) error) error {
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: time.Duration(r.Configuration.TimeoutSeconds) * time.Second}
	}
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = "https://crt.sh/"
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse CT endpoint: %w", err)
	}
	q := u.Query()
	q.Set("q", "%."+r.PrimaryDomain)
	q.Set("output", "json")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CT service returned %d", resp.StatusCode)
	}
	var records []struct {
		NameValue string `json:"name_value"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&records); err != nil {
		return fmt.Errorf("decode CT records: %w", err)
	}
	seen := map[string]bool{}
	for _, record := range records {
		for _, raw := range strings.Split(record.NameValue, "\n") {
			host := strings.TrimPrefix(strings.TrimSpace(strings.ToLower(raw)), "*.")
			if host == "" || seen[host] || !(host == r.PrimaryDomain || strings.HasSuffix(host, "."+r.PrimaryDomain)) {
				continue
			}
			if _, err := scope.NormalizeHost(host); err != nil {
				continue
			}
			seen[host] = true
			kind := "subdomain"
			if host == r.PrimaryDomain {
				kind = "domain"
			}
			if err := emit(Observation{EntityType: kind, Value: host, Source: c.Name(), Confidence: 1, Metadata: `{"classification":"observed"}`, ObservedAt: time.Now().UTC()}); err != nil {
				return err
			}
		}
	}
	return nil
}
