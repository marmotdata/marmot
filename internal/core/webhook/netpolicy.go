package webhook

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// errDisallowedAddress is returned when a webhook connection would reach a loopback, private or otherwise internal address.
var errDisallowedAddress = errors.New("webhook destination address is not allowed")

// disallowedPrefixes covers internal ranges not already handled by the netip.Addr predicates in isDisallowedIP.
var disallowedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
}

// isDisallowedIP reports whether ip is an address webhooks must never connect to, unwrapping IPv4-mapped IPv6 addresses first.
func isDisallowedIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return true
	}
	for _, prefix := range disallowedPrefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

// dialControl rejects connections to disallowed addresses after DNS resolution, which covers redirects and DNS rebinding as well as the stored URL.
func dialControl(_, address string, _ syscall.RawConn) error {
	addrPort, err := netip.ParseAddrPort(address)
	if err != nil || isDisallowedIP(addrPort.Addr()) {
		return errDisallowedAddress
	}
	return nil
}

// newDeliveryClient returns an HTTP client that only connects to public addresses, does not use environment proxies so the dial check sees the real destination and does not follow redirects.
func newDeliveryClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   dialControl,
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
