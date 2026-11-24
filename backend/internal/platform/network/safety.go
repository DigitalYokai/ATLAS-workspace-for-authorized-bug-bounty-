package network

import (
	"context"
	"fmt"
	"net"
	"net/netip"
)

type Resolver interface {
	LookupIP(context.Context, string) ([]net.IP, error)
}
type DefaultResolver struct{}

func (DefaultResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}
func PublicDestination(ctx context.Context, r Resolver, host string) error {
	if r == nil {
		r = DefaultResolver{}
	}
	ips, e := r.LookupIP(ctx, host)
	if e != nil {
		return fmt.Errorf("resolve destination: %w", e)
	}
	if len(ips) == 0 {
		return fmt.Errorf("destination has no addresses")
	}
	for _, ip := range ips {
		a, ok := netip.AddrFromSlice(ip)
		if !ok || !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() || a.IsMulticast() || a.IsUnspecified() {
			return fmt.Errorf("unsafe destination address %s", ip)
		}
	}
	return nil
}
