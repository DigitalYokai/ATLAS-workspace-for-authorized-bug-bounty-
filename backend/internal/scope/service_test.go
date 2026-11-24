package scope

import "testing"

func TestWildcardMatchingAndExclusionPrecedence(t *testing.T) {
	rules := []Rule{{ID: "include", Kind: Include, Pattern: "example.com", PatternKind: Wildcard, PermittedMethods: []string{"dns_lookup"}}, {ID: "exclude", Kind: Exclude, Pattern: "payments.example.com", PatternKind: Exact}}
	if got := Evaluate(rules, "api.example.com", "dns_lookup"); !got.ActiveApproved || got.Status != "included" {
		t.Fatalf("expected included active approval, got %#v", got)
	}
	if got := Evaluate(rules, "example.com", "dns_lookup"); got.Status != "unknown" {
		t.Fatalf("wildcard matched base domain: %#v", got)
	}
	if got := Evaluate(rules, "payments.example.com", "dns_lookup"); got.Status != "excluded" || got.ActiveApproved {
		t.Fatalf("exclusion did not win: %#v", got)
	}
	if got := Evaluate(rules, "badexample.com", "dns_lookup"); got.Status != "unknown" {
		t.Fatalf("wildcard crossed label boundary: %#v", got)
	}
}
