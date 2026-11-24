package network

import (
	"context"
	"net"
	"testing"
)

type fakeResolver struct{ ips []net.IP }

func (f fakeResolver) LookupIP(context.Context, string) ([]net.IP, error) { return f.ips, nil }
func TestPublicDestinationRejectsPrivateAddress(t *testing.T) {
	if err := PublicDestination(context.Background(), fakeResolver{[]net.IP{net.ParseIP("127.0.0.1")}}, "example.com"); err == nil {
		t.Fatal("expected loopback rejection")
	}
	if err := PublicDestination(context.Background(), fakeResolver{[]net.IP{net.ParseIP("8.8.8.8")}}, "example.com"); err != nil {
		t.Fatalf("public address rejected: %v", err)
	}
}
