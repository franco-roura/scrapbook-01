package preview

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"

	"github.com/franco-roura/scrapbook-01/server/internal/telemetry"
)

// ErrBlockedAddress is returned when a link resolves to an address the server
// must not reach: its own network, the Lambda metadata endpoint, and so on.
var ErrBlockedAddress = errors.New("preview: address not allowed")

// blocked are ranges that aren't the public internet. IsPrivate, IsLoopback
// and friends cover most; these fill the gaps.
var blocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),     // "this network"
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),  // IETF protocol assignments
	netip.MustParsePrefix("198.18.0.0/15"), // benchmarking
	netip.MustParsePrefix("240.0.0.0/4"),   // reserved
	netip.MustParsePrefix("64:ff9b::/96"),  // NAT64, could reach IPv4 inside
	netip.MustParsePrefix("2002::/16"),     // 6to4, same
}

// Allowed reports whether ip is a public unicast address.
func Allowed(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() ||
		ip.IsUnspecified() {
		return false
	}
	for _, p := range blocked {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// guard runs on every connection after DNS has resolved, so a hostname that
// points at a private address (or changes to one between lookups) is refused
// too, and so is every redirect hop.
func guard(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !Allowed(ip) {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, host)
	}
	return nil
}

// NewClient is an HTTP client for fetching untrusted links: public addresses
// only, no proxy, bounded time and redirects.
func NewClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: guard}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
	}
	return &http.Client{
		// Spans name only the host: the path is the user's saved link.
		Transport: telemetry.HostOnlyTransport(transport),
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("preview: too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("preview: redirect to %s", req.URL.Scheme)
			}
			return nil
		},
	}
}
