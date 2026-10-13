package netutil

import (
	"context"
	"net/netip"
	"testing"
	"time"
)

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "::1", "10.0.0.5", "172.16.0.1", "192.168.1.10",
		"169.254.169.254", "100.64.0.1", "fe80::1", "fd00::1", "0.0.0.0",
	}
	for _, s := range blocked {
		ip := netip.MustParseAddr(s)
		if !IsBlockedIP(ip) {
			t.Errorf("expected %s to be blocked", s)
		}
	}

	allowed := []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"}
	for _, s := range allowed {
		ip := netip.MustParseAddr(s)
		if IsBlockedIP(ip) {
			t.Errorf("expected %s to be allowed", s)
		}
	}
}

func TestResolveSafeBlocksPrivateByDefault(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := ResolveSafe(ctx, "localhost", false); err == nil {
		t.Fatal("expected localhost to be blocked when private networks are disallowed")
	}

	if _, err := ResolveSafe(ctx, "localhost", true); err != nil {
		t.Fatalf("expected localhost to resolve when private networks are allowed, got %v", err)
	}
}
