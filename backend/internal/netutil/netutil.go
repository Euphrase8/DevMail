// Package netutil provides SSRF-safe network primitives used by
// connectivity and DNS checks. Hostnames are resolved and every resolved
// address is validated before a connection is attempted.
package netutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"
)

// ErrNetworkBlocked indicates the destination is not allowed by the SSRF
// policy. The message never leaks whether an address is internal.
var ErrNetworkBlocked = errors.New("destination blocked by network policy")

// IsBlockedIP reports whether an address is loopback, link-local, multicast,
// unspecified, or private (RFC1918 / unique-local), regardless of policy.
func IsBlockedIP(ip netip.Addr) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	ip = ip.Unmap()
	if ip.Is4() {
		a4 := ip.As4()
		// 10/8, 172.16/12, 192.168/16, 169.254/16 metadata+linklocal.
		if a4[0] == 10 || (a4[0] == 172 && a4[1]&0xf0 == 16) || (a4[0] == 192 && a4[1] == 168) || (a4[0] == 169 && a4[1] == 254) {
			return true
		}
		// 100.64/10 CGNAT.
		if a4[0] == 100 && a4[1]&0xc0 == 64 {
			return true
		}
		return false
	}
	// IPv6 unique local fd00::/8 and ULA.
	first := ip.As16()
	return first[0] == 0xfd || first[0] == 0xfc
}

// ResolveSafe resolves a hostname and returns its addresses, rejecting any
// resolution that yields an address outside the network policy.
func ResolveSafe(ctx context.Context, host string, allowPrivate bool) ([]netip.Addr, error) {
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("resolve %s: no addresses", host)
	}
	if !allowPrivate {
		for _, a := range addrs {
			if IsBlockedIP(a) {
				return nil, ErrNetworkBlocked
			}
		}
	}
	return addrs, nil
}

// CheckPort verifies TCP reachability of host:port using a fully-qualified
// address resolved just before the connection attempt.
func CheckPort(ctx context.Context, host string, port int, timeout time.Duration, allowPrivate bool) (ok bool, latency time.Duration, addr string, err error) {
	addrs, err := ResolveSafe(ctx, host, allowPrivate)
	if err != nil {
		return false, 0, "", err
	}
	var lastErr error
	for _, a := range addrs {
		dialCtx, cancel := context.WithTimeout(ctx, timeout)
		start := time.Now()
		var d net.Dialer
		conn, dialErr := d.DialContext(dialCtx, "tcp", net.JoinHostPort(a.String(), fmt.Sprintf("%d", port)))
		latency = time.Since(start)
		cancel()
		if dialErr == nil {
			conn.Close()
			return true, latency, a.String(), nil
		}
		lastErr = dialErr
	}
	return false, 0, "", lastErr
}

// LookupMX returns the MX records for a domain.
func LookupMX(ctx context.Context, host string, timeout time.Duration) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	records, err := net.DefaultResolver.LookupMX(ctx, host)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(records))
	for _, mx := range records {
		out = append(out, fmt.Sprintf("%s (priority %d)", mx.Host, mx.Pref))
	}
	return out, nil
}

// LookupTXT returns the TXT records for a name.
func LookupTXT(ctx context.Context, host string, timeout time.Duration) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return net.DefaultResolver.LookupTXT(ctx, host)
}

// LookupA returns the IPv4 addresses for a name.
func LookupA(ctx context.Context, host string, timeout time.Duration) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}
	return addrs, nil
}
