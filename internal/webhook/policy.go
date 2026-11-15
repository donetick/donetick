// Package webhook holds the security rules for every outbound HTTP call
// Donetick makes to a user-supplied URL: circle webhooks, Discord notification
// targets and anything else a user can point the server at.
//
// Two things are enforced here. A URL is checked before it is stored, so a bad
// value is rejected while the user is still looking at the form, and the
// resolved IP is checked again at connect time, because DNS can change between
// the two (DNS rebinding).
package webhook

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"donetick.com/core/config"
)

// maxURLLength keeps a webhook URL to something a database column and a log
// line can live with.
const maxURLLength = 2048

// resolveTimeout bounds the DNS lookup done while checking a URL.
const resolveTimeout = 5 * time.Second

var (
	// ErrInvalidURL and friends are returned wrapped; handlers surface the
	// message to the user, so every message has to be safe to show.
	ErrInvalidURL = errors.New("invalid webhook URL")
	// ErrBlockedAddress means the URL resolves somewhere we refuse to call.
	ErrBlockedAddress = errors.New("webhook URL points to a blocked address")
)

// blockedPorts are ports that no webhook receiver listens on, but plenty of
// internal services do. Blocking them removes the cheapest way to turn the
// webhook sender into a port scanner for the host network.
var blockedPorts = map[int]struct{}{
	22:    {}, // ssh
	23:    {}, // telnet
	25:    {}, // smtp
	110:   {}, // pop3
	135:   {}, // msrpc
	137:   {}, // netbios
	139:   {}, // netbios
	143:   {}, // imap
	445:   {}, // smb
	465:   {}, // smtps
	587:   {}, // submission
	1433:  {}, // mssql
	2375:  {}, // docker api
	2376:  {}, // docker api (tls)
	3306:  {}, // mysql
	5432:  {}, // postgres
	5984:  {}, // couchdb
	6379:  {}, // redis
	9200:  {}, // elasticsearch
	11211: {}, // memcached
	27017: {}, // mongodb
}

// metadataHosts are cloud instance-metadata endpoints. 169.254.169.254 is
// already covered by the link-local rule; the IPv6 AWS endpoint lives in the
// unique-local range, which is allowed by default, so it is named explicitly.
var metadataHosts = []net.IP{
	net.ParseIP("fd00:ec2::254"),
}

// nat64Prefix is the well-known NAT64 range: these addresses carry an embedded
// IPv4 address in their last four bytes, so they can reach a private IPv4 host
// without ever looking private themselves.
var nat64Prefix = net.IPNet{
	IP:   net.ParseIP("64:ff9b::"),
	Mask: net.CIDRMask(96, 128),
}

// Policy is the set of destinations this instance is willing to call.
//
// The defaults suit a self-hosted instance: a webhook to Home Assistant or
// n8n on the LAN keeps working, while the destinations that only ever show up
// in an SSRF attempt (the loopback interface, cloud metadata, unroutable
// space) are refused. A multi-tenant deployment should set
// BlockPrivateNetworks, since there a "private" address belongs to the
// operator, not to the user configuring the webhook.
type Policy struct {
	// BlockPrivateNetworks rejects RFC1918, unique-local and carrier-grade
	// NAT addresses.
	BlockPrivateNetworks bool
	// AllowLoopback permits 127.0.0.0/8 and ::1, for instances that really
	// do run the receiver next to the server.
	AllowLoopback bool
	// RequireHTTPS rejects plain http:// destinations.
	RequireHTTPS bool
}

// PolicyFromConfig reads the policy out of the webhook config block.
func PolicyFromConfig(cfg *config.Config) Policy {
	if cfg == nil {
		return Policy{}
	}
	return Policy{
		BlockPrivateNetworks: cfg.WebhookConfig.BlockPrivateNetworks,
		AllowLoopback:        cfg.WebhookConfig.AllowLoopback,
		RequireHTTPS:         cfg.WebhookConfig.RequireHTTPS,
	}
}

// ValidateURL checks a user-supplied webhook URL, including resolving its
// host and checking every address it answers with. Call it before storing a
// URL. It is not a substitute for the connect-time check in Client: DNS can
// answer differently later.
func (p Policy) ValidateURL(ctx context.Context, raw string) error {
	u, err := p.parseURL(raw)
	if err != nil {
		return err
	}
	return p.ValidateHost(ctx, u.Hostname())
}

// ValidateURLSyntax runs the checks that do not need DNS: scheme, credentials,
// port and literal IP addresses. Use it on the send path, where the
// connect-time check covers the rest and a DNS lookup would be a second
// resolution of the same name.
func (p Policy) ValidateURLSyntax(raw string) error {
	_, err := p.parseURL(raw)
	return err
}

func (p Policy) parseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%w: URL is empty", ErrInvalidURL)
	}
	if len(raw) > maxURLLength {
		return nil, fmt.Errorf("%w: URL is longer than %d characters", ErrInvalidURL, maxURLLength)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidURL, err.Error())
	}

	switch strings.ToLower(u.Scheme) {
	case "https":
	case "http":
		if p.RequireHTTPS {
			return nil, fmt.Errorf("%w: only https:// URLs are allowed", ErrInvalidURL)
		}
	default:
		return nil, fmt.Errorf("%w: scheme %q is not supported, use http or https", ErrInvalidURL, u.Scheme)
	}

	// Credentials in the URL would be logged with it and sent to the
	// destination on every delivery.
	if u.User != nil {
		return nil, fmt.Errorf("%w: credentials in the URL are not allowed", ErrInvalidURL)
	}

	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("%w: URL has no host", ErrInvalidURL)
	}

	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("%w: port %q is not valid", ErrInvalidURL, port)
		}
		if _, blocked := blockedPorts[n]; blocked {
			return nil, fmt.Errorf("%w: port %d is not allowed", ErrInvalidURL, n)
		}
	}

	// A literal IP needs no DNS, so check it here and let callers that skip
	// resolution still get the protection.
	if ip := net.ParseIP(host); ip != nil {
		if err := p.ValidateIP(ip); err != nil {
			return nil, err
		}
	} else if isNumericHost(host) {
		// Things like http://2852039166/ (169.254.169.254 written as a
		// decimal) and http://0x7f.1/ are not valid hostnames, they are
		// IP addresses in a notation that slips past an IP check.
		return nil, fmt.Errorf("%w: host %q is not a valid hostname", ErrInvalidURL, host)
	}

	return u, nil
}

// ValidateHost resolves a hostname and checks every address it answers with.
// A literal IP is checked directly.
func (p Policy) ValidateHost(ctx context.Context, host string) error {
	if ip := net.ParseIP(host); ip != nil {
		return p.ValidateIP(ip)
	}

	// Bound the lookup: this runs inside a request handler, and the
	// hostname is whatever the user typed.
	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("%w: could not resolve host %q", ErrInvalidURL, host)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%w: host %q did not resolve to any address", ErrInvalidURL, host)
	}
	// Every answer has to pass: a host that returns one public and one
	// private address is the classic way around a first-answer-only check.
	for _, ip := range ips {
		if err := p.ValidateIP(ip.IP); err != nil {
			return err
		}
	}
	return nil
}

// ValidateIP reports whether this policy allows connecting to an address.
func (p Policy) ValidateIP(ip net.IP) error {
	if ip == nil {
		return fmt.Errorf("%w: address is not valid", ErrBlockedAddress)
	}

	// An IPv4-mapped or NAT64 address reaches an IPv4 host, so judge it as
	// the IPv4 address it really is.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	} else if nat64Prefix.Contains(ip) {
		ip = net.IPv4(ip[12], ip[13], ip[14], ip[15])
	}

	for _, blocked := range metadataHosts {
		if ip.Equal(blocked) {
			return fmt.Errorf("%w: %s is a cloud metadata endpoint", ErrBlockedAddress, ip)
		}
	}

	switch {
	case ip.IsUnspecified():
		return fmt.Errorf("%w: %s is not a routable address", ErrBlockedAddress, ip)
	case ip.IsLoopback():
		if !p.AllowLoopback {
			return fmt.Errorf("%w: %s is a loopback address", ErrBlockedAddress, ip)
		}
	case ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast():
		// Covers 169.254.169.254, the metadata endpoint of most clouds.
		return fmt.Errorf("%w: %s is a link-local address", ErrBlockedAddress, ip)
	case ip.IsMulticast(), ip.IsInterfaceLocalMulticast():
		return fmt.Errorf("%w: %s is a multicast address", ErrBlockedAddress, ip)
	case isUnroutable(ip):
		return fmt.Errorf("%w: %s is in a reserved range", ErrBlockedAddress, ip)
	case ip.IsPrivate() || isCGNAT(ip):
		if p.BlockPrivateNetworks {
			return fmt.Errorf("%w: %s is a private address", ErrBlockedAddress, ip)
		}
	}

	return nil
}

// isNumericHost reports whether a host is made only of digits, dots and an
// optional hex prefix: the shapes an IP address takes when it is written to
// look like a name. A real hostname always ends in a letter.
func isNumericHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return false
	}
	if strings.HasPrefix(host, "0x") {
		return true
	}
	last := host[len(host)-1]
	return last >= '0' && last <= '9'
}

// isCGNAT reports whether the address is in 100.64.0.0/10, which net.IP does
// not count as private but which routes inside provider and Tailscale
// networks.
func isCGNAT(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127
}

// isUnroutable covers the reserved ranges that have no business receiving a
// webhook: "this network", benchmarking, documentation and the broadcast
// address.
func isUnroutable(ip net.IP) bool {
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	switch {
	case v4[0] == 0: // 0.0.0.0/8
		return true
	case v4[0] == 192 && v4[1] == 0 && v4[2] == 0: // 192.0.0.0/24, IETF protocol assignments
		return true
	case v4[0] == 198 && (v4[1] == 18 || v4[1] == 19): // 198.18.0.0/15, benchmarking
		return true
	case v4.Equal(net.IPv4bcast):
		return true
	case v4[0] >= 240: // 240.0.0.0/4, reserved
		return true
	}
	return false
}
