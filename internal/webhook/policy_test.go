package webhook

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestPolicyParseURLRejectsBadURLs(t *testing.T) {
	p := Policy{}

	cases := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"blank", "   "},
		{"relative", "/hooks/abc"},
		{"no host", "https://"},
		{"unsupported scheme", "file:///etc/passwd"},
		{"gopher scheme", "gopher://example.com:70/"},
		{"credentials", "https://user:pass@example.com/hook"},
		{"blocked port", "https://example.com:6379/hook"},
		{"bad port", "https://example.com:99999/hook"},
		{"loopback literal", "http://127.0.0.1:8080/hook"},
		{"loopback v6", "http://[::1]:8080/hook"},
		{"metadata", "http://169.254.169.254/latest/meta-data/"},
		{"metadata decimal", "http://2852039166/"},
		{"octal", "http://0177.0.0.1/hook"},
		{"hex", "http://0x7f.0x1/hook"},
		{"unspecified", "http://0.0.0.0/hook"},
		{"ipv4 mapped loopback", "http://[::ffff:127.0.0.1]/hook"},
		{"too long", "https://example.com/" + string(make([]byte, maxURLLength))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := p.ValidateURLSyntax(tc.url); err == nil {
				t.Fatalf("expected %q to be rejected", tc.url)
			}
		})
	}
}

func TestPolicyParseURLAcceptsNormalWebhooks(t *testing.T) {
	p := Policy{}

	for _, raw := range []string{
		"https://example.com/hooks/abc",
		"http://example.com/hooks/abc",
		"https://example.com:8443/hooks/abc",
		"https://hooks.example.com/services/T000/B000/xyz?token=1",
		"http://192.168.1.50:8123/api/webhook/donetick", // LAN receiver, allowed by default
		// Hostnames that end in a digit are ordinary names, not IP
		// addresses wearing a disguise.
		"http://homeassistant.lan2/api/webhook/donetick",
		"http://host01.example.com/hook",
		"https://s3.us-east-2.example.com/hook",
		"http://0xdeadbeef.example.com/hook",
	} {
		if err := p.ValidateURLSyntax(raw); err != nil {
			t.Fatalf("expected %q to be accepted, got %v", raw, err)
		}
	}
}

func TestPolicyRequireHTTPS(t *testing.T) {
	p := Policy{RequireHTTPS: true}

	if err := p.ValidateURLSyntax("http://example.com/hook"); err == nil {
		t.Fatal("expected plain http to be rejected when RequireHTTPS is set")
	}
	if err := p.ValidateURLSyntax("https://example.com/hook"); err != nil {
		t.Fatalf("expected https to be accepted: %v", err)
	}
}

func TestPolicyAllowLoopback(t *testing.T) {
	if err := (Policy{AllowLoopback: true}).ValidateURLSyntax("http://127.0.0.1:8080/hook"); err != nil {
		t.Fatalf("expected loopback to be accepted when allowed: %v", err)
	}
}

func TestPolicyBlockPrivateNetworks(t *testing.T) {
	p := Policy{BlockPrivateNetworks: true}

	for _, raw := range []string{
		"http://192.168.1.50/hook",
		"http://10.0.0.5/hook",
		"http://172.16.4.4/hook",
		"http://100.64.1.1/hook",   // carrier-grade NAT / tailscale
		"http://[fd00::1]/hook",    // unique local
		"http://[64:ff9b::a00:5]/", // NAT64 wrapping 10.0.0.5
	} {
		if err := p.ValidateURLSyntax(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}

	if err := p.ValidateURLSyntax("https://93.184.216.34/hook"); err != nil {
		t.Fatalf("expected public address to be accepted: %v", err)
	}
}

func TestValidateIPBlockedRanges(t *testing.T) {
	p := Policy{}

	blocked := []string{
		"0.0.0.0",
		"169.254.169.254",
		"fe80::1",
		"224.0.0.1",
		"255.255.255.255",
		"240.0.0.1",
		"198.18.0.1",
		"192.0.0.1",
		"fd00:ec2::254", // AWS IMDS over IPv6, inside the allowed ULA range
	}
	for _, raw := range blocked {
		if err := p.ValidateIP(net.ParseIP(raw)); err == nil {
			t.Fatalf("expected %s to be blocked", raw)
		}
	}

	if err := p.ValidateIP(nil); err == nil {
		t.Fatal("expected a nil IP to be rejected")
	}
	if !errors.Is(p.ValidateIP(net.ParseIP("169.254.169.254")), ErrBlockedAddress) {
		t.Fatal("expected ErrBlockedAddress for the metadata endpoint")
	}
}

func TestValidateHostRejectsAnyPrivateAnswer(t *testing.T) {
	// localhost resolves to loopback on every platform we run on, which
	// exercises the "every answer must pass" path without needing DNS.
	if err := (Policy{}).ValidateHost(context.Background(), "localhost"); err == nil {
		t.Fatal("expected localhost to be rejected")
	}
	if err := (Policy{}).ValidateHost(context.Background(), "donetick.invalid"); err == nil {
		t.Fatal("expected an unresolvable host to be rejected")
	}
}
