package scope

import (
	"fmt"
	"strings"
)

var allowedMethods = map[string]bool{"passive_discovery": true, "dns_lookup": true, "tls_handshake": true, "http_probe": true}

func NormalizeHost(value string) (string, error) {
	h := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if h == "" || len(h) > 253 {
		return "", fmt.Errorf("invalid hostname")
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", fmt.Errorf("invalid hostname")
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return "", fmt.Errorf("invalid hostname")
			}
		}
	}
	return h, nil
}
func NormalizeInput(in Input) (Input, PatternKind, error) {
	p := strings.TrimSpace(strings.ToLower(in.Pattern))
	kind := Exact
	if strings.HasPrefix(p, "*.") {
		kind = Wildcard
		p = p[2:]
	}
	if _, err := NormalizeHost(p); err != nil {
		return Input{}, "", err
	}
	if in.Kind != Include && in.Kind != Exclude {
		return Input{}, "", fmt.Errorf("rule kind must be include or exclude")
	}
	if in.Kind == Exclude && len(in.PermittedMethods) > 0 {
		return Input{}, "", fmt.Errorf("exclusions cannot permit methods")
	}
	for _, method := range in.PermittedMethods {
		if !allowedMethods[method] {
			return Input{}, "", fmt.Errorf("unsupported permitted method %q", method)
		}
	}
	in.Pattern = p
	return in, kind, nil
}
func matches(r Rule, host string) bool {
	if r.PatternKind == Exact {
		return host == r.Pattern
	}
	return strings.HasSuffix(host, "."+r.Pattern) && host != r.Pattern
}
func Evaluate(rules []Rule, rawHost, method string) Decision {
	host, err := NormalizeHost(rawHost)
	if err != nil {
		return Decision{Host: rawHost, Method: method, Status: "unknown", Reason: "host is invalid or ambiguous"}
	}
	for _, r := range rules {
		if r.Kind == Exclude && matches(r, host) {
			return Decision{Host: host, Method: method, Status: "excluded", Reason: "matching exclusion rule", MatchedRuleID: r.ID}
		}
	}
	for _, r := range rules {
		if r.Kind == Include && matches(r, host) {
			approved := false
			for _, m := range r.PermittedMethods {
				if m == method {
					approved = true
				}
			}
			reason := "matching inclusion rule"
			if !approved {
				reason = "method is not permitted by matching inclusion rule"
			}
			return Decision{Host: host, Method: method, Status: "included", ActiveApproved: approved, Reason: reason, MatchedRuleID: r.ID}
		}
	}
	return Decision{Host: host, Method: method, Status: "unknown", Reason: "no inclusion rule matched"}
}
