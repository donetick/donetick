package webhook

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"
)

// defaultMaxResponseBytes is how much of a receiver's response we are willing
// to read. We do not use the body, we only drain enough of it to let the
// connection be reused.
const defaultMaxResponseBytes int64 = 64 << 10

// ErrRedirectBlocked is returned when a receiver answers with a redirect.
// Following one would hand the destination choice to the receiver, which is
// exactly what the policy checks are there to prevent.
var ErrRedirectBlocked = errors.New("webhook destination returned a redirect, which is not followed")

// Client is an http.Client for user-supplied destinations. On top of the
// policy check done when the URL was stored it re-checks the address the
// connection actually goes to, so a hostname that resolved to a public
// address at save time cannot be re-pointed at an internal one later.
type Client struct {
	http             *http.Client
	maxResponseBytes int64
}

// NewClient builds a client that enforces policy at connect time.
func NewClient(policy Policy, timeout time.Duration, maxResponseBytes int64) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultMaxResponseBytes
	}

	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		// Control runs after the address is resolved and before the
		// socket connects, which is the only point where the address
		// we are about to talk to is known for certain.
		Control: func(network, address string, _ syscall.RawConn) error {
			switch network {
			case "tcp", "tcp4", "tcp6":
			default:
				return fmt.Errorf("%w: network %q is not allowed", ErrBlockedAddress, network)
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrBlockedAddress, address)
			}
			return policy.ValidateIP(net.ParseIP(host))
		},
	}

	return &Client{
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DialContext:           dialer.DialContext,
				TLSHandshakeTimeout:   timeout,
				ResponseHeaderTimeout: timeout,
				MaxIdleConns:          50,
				MaxIdleConnsPerHost:   2,
				IdleConnTimeout:       60 * time.Second,
				DisableCompression:    true,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return ErrRedirectBlocked
			},
		},
		maxResponseBytes: maxResponseBytes,
	}
}

// Do sends the request and drains a bounded part of the response, so a
// receiver cannot hold a worker open by streaming an endless body. The
// response body is already closed when Do returns.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	_, _ = io.CopyN(io.Discard, resp.Body, c.maxResponseBytes)
	return resp, nil
}

// PostJSON sends a JSON body to url with the given headers.
func (c *Client) PostJSON(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, newBodyReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.Do(req)
}
