package outbound

import (
	"net/netip"
	"testing"
)

func TestNormalizePublicHTTPSBaseURLRejectsUnsafeProviderURLs(t *testing.T) {
	cases := []string{
		"http://api.example.com",
		"https://localhost:11434",
		"https://service.localhost",
		"https://127.0.0.1:11434",
		"https://[::1]:11434",
		"https://[fe80::1%25eth0]:11434",
		"https://10.0.0.4/v1",
		"https://100.64.0.1/v1",
		"https://169.254.169.254/latest/meta-data",
		"https://203.0.113.10/v1",
		"https://[2001:db8::1]/v1",
		"https://user:pass@api.example.com",
		"https://api.example.com/v1?target=internal",
		"https://api.example.com/v1#fragment",
	}
	for _, baseURL := range cases {
		t.Run(baseURL, func(t *testing.T) {
			if _, err := NormalizePublicHTTPSBaseURL(baseURL); err == nil {
				t.Fatalf("NormalizePublicHTTPSBaseURL(%q) error = nil, want error", baseURL)
			}
		})
	}
}

func TestNormalizePublicHTTPSBaseURLAcceptsPublicHTTPSProviderURL(t *testing.T) {
	got, err := NormalizePublicHTTPSBaseURL(" https://api.example.com/v1/ ")
	if err != nil {
		t.Fatalf("NormalizePublicHTTPSBaseURL() error = %v", err)
	}
	if got != "https://api.example.com/v1" {
		t.Fatalf("NormalizePublicHTTPSBaseURL() = %q", got)
	}
}

func TestIsPublicProviderAddrClassifiesNetworkBoundaries(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":          true,
		"2606:4700::1111":  true,
		"0.0.0.1":          false,
		"10.1.2.3":         false,
		"127.0.0.1":        false,
		"169.254.1.1":      false,
		"100.64.0.1":       false,
		"192.88.99.1":      false,
		"192.0.2.1":        false,
		"203.0.113.1":      false,
		"64:ff9b::808:808": false,
		"2001:db8::1":      false,
		"2002::1":          false,
	}
	for value, want := range cases {
		t.Run(value, func(t *testing.T) {
			got := IsPublicProviderAddr(netip.MustParseAddr(value))
			if got != want {
				t.Fatalf("IsPublicProviderAddr(%q) = %t, want %t", value, got, want)
			}
		})
	}
}

func TestPickDialAddrRequiresRequestedAddressFamily(t *testing.T) {
	ipv6Only := []netip.Addr{netip.MustParseAddr("2606:4700::1111")}
	if _, ok := pickDialAddr("tcp4", ipv6Only); ok {
		t.Fatal("pickDialAddr(tcp4, ipv6Only) ok = true, want false")
	}
	addr, ok := pickDialAddr("tcp6", ipv6Only)
	if !ok || !addr.Is6() {
		t.Fatalf("pickDialAddr(tcp6, ipv6Only) = %v, %t; want IPv6 true", addr, ok)
	}
}
