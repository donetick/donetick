package webhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientSendsSignedJSON(t *testing.T) {
	var (
		gotBody      string
		gotSignature string
		gotUserAgent string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		gotSignature = r.Header.Get(HeaderSignature)
		gotUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// The test server listens on loopback, so the client has to allow it.
	c := NewClient(Policy{AllowLoopback: true}, 5*time.Second, 0)

	ts := time.Unix(1764600000, 0)
	body := []byte(`{"type":"task.created"}`)
	resp, err := c.PostJSON(context.Background(), srv.URL, body, map[string]string{
		HeaderSignature: Sign("s3cret", ts, body),
	})
	if err != nil {
		t.Fatalf("PostJSON: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if gotBody != string(body) {
		t.Fatalf("body = %q", gotBody)
	}
	if gotSignature != Sign("s3cret", ts, body) {
		t.Fatalf("signature header = %q", gotSignature)
	}
	if gotUserAgent != UserAgent {
		t.Fatalf("user agent = %q", gotUserAgent)
	}
}

func TestClientBlocksAtConnectTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the request should never have reached the server")
	}))
	defer srv.Close()

	// Loopback is not allowed, so the dialer refuses the connection even
	// though the URL itself was handed straight to the client. This is the
	// check that survives a hostname being re-pointed after it was saved.
	c := NewClient(Policy{}, 5*time.Second, 0)
	_, err := c.PostJSON(context.Background(), srv.URL, []byte(`{}`), nil)
	if err == nil {
		t.Fatal("expected the connection to be refused")
	}
	if !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("expected ErrBlockedAddress, got %v", err)
	}
}

func TestClientDoesNotFollowRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()

	c := NewClient(Policy{AllowLoopback: true}, 5*time.Second, 0)
	_, err := c.PostJSON(context.Background(), srv.URL, []byte(`{}`), nil)
	if err == nil || !errors.Is(err, ErrRedirectBlocked) {
		t.Fatalf("expected the redirect to be refused, got %v", err)
	}
}

func TestClientFailsWhenTheResponseStalls(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Flush a 200 and then go quiet: the status says success, but
		// the delivery never completes.
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-release
	}))
	defer srv.Close()
	defer close(release)

	c := NewClient(Policy{AllowLoopback: true}, 300*time.Millisecond, 0)
	resp, err := c.PostJSON(context.Background(), srv.URL, []byte(`{}`), nil)
	if err == nil {
		t.Fatalf("expected a stalled body to fail the delivery, got status %d", resp.StatusCode)
	}
	if resp != nil {
		t.Fatal("expected no response to be returned on a drain failure")
	}
}

func TestClientBoundsResponseRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Far more than the cap below; the client must not try to read
		// all of it.
		for i := 0; i < 64; i++ {
			if _, err := w.Write([]byte(strings.Repeat("x", 1024))); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	c := NewClient(Policy{AllowLoopback: true}, 5*time.Second, 512)
	resp, err := c.PostJSON(context.Background(), srv.URL, []byte(`{}`), nil)
	if err != nil {
		t.Fatalf("PostJSON: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
