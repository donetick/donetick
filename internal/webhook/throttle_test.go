package webhook

import (
	"testing"
	"time"
)

func newTestThrottle(perMinute, failures int, cooldown time.Duration, now *time.Time) *Throttle {
	t := NewThrottle(perMinute, failures, cooldown)
	t.now = func() time.Time { return *now }
	return t
}

func TestThrottleRateLimitsPerHost(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	th := newTestThrottle(2, 0, time.Minute, &now)

	for i := 0; i < 2; i++ {
		if ok, _ := th.Allow("https://example.com/a"); !ok {
			t.Fatalf("delivery %d should have been allowed", i)
		}
	}

	// Same host, different path: the budget is per host on purpose.
	if ok, reason := th.Allow("https://example.com/b"); ok {
		t.Fatal("expected the third delivery to the same host to be dropped")
	} else if reason == "" {
		t.Fatal("expected a reason to log")
	}

	// A different host has its own budget.
	if ok, _ := th.Allow("https://other.example.com/a"); !ok {
		t.Fatal("expected a different host to be allowed")
	}

	// The window rolls.
	now = now.Add(time.Minute)
	if ok, _ := th.Allow("https://example.com/a"); !ok {
		t.Fatal("expected the budget to reset after a minute")
	}
}

func TestThrottleNormalizesTheHostKey(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	th := newTestThrottle(2, 0, time.Minute, &now)

	// All four spellings reach the same server, so they share one budget.
	for _, raw := range []string{
		"https://example.com/a",
		"https://EXAMPLE.COM/b",
	} {
		if ok, _ := th.Allow(raw); !ok {
			t.Fatalf("%q should have been allowed", raw)
		}
	}
	for _, raw := range []string{
		"https://example.com./c",
		"https://example.com:443/d",
	} {
		if ok, _ := th.Allow(raw); ok {
			t.Fatalf("%q should have shared the exhausted budget", raw)
		}
	}

	// A non-default port is a different listener and keeps its own budget.
	if ok, _ := th.Allow("https://example.com:8443/e"); !ok {
		t.Fatal("a non-default port should have its own budget")
	}
}

func TestThrottleFailuresAreScopedToTheEndpoint(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	th := newTestThrottle(0, 2, 10*time.Minute, &now)

	const dead = "https://hooks.example.com/services/circle-a/dead"
	const live = "https://hooks.example.com/services/circle-b/live"

	th.RecordFailure(dead)
	if parked := th.RecordFailure(dead); !parked {
		t.Fatal("expected the failing endpoint to be parked")
	}
	if ok, _ := th.Allow(dead); ok {
		t.Fatal("expected the failing endpoint to be in cooldown")
	}

	// Another circle's integration on the same host is unaffected.
	if ok, _ := th.Allow(live); !ok {
		t.Fatal("a different endpoint on the same host should still be delivered to")
	}
}

func TestThrottleParksFailingDestination(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	th := newTestThrottle(0, 3, 10*time.Minute, &now)
	const url = "https://example.com/hook"

	if parked := th.RecordFailure(url); parked {
		t.Fatal("one failure should not park the destination")
	}
	th.RecordFailure(url)
	if parked := th.RecordFailure(url); !parked {
		t.Fatal("expected the third consecutive failure to park the destination")
	}
	if ok, _ := th.Allow(url); ok {
		t.Fatal("expected deliveries to be dropped during the cooldown")
	}

	// After the cooldown it gets a clean slate.
	now = now.Add(11 * time.Minute)
	if ok, _ := th.Allow(url); !ok {
		t.Fatal("expected deliveries to resume after the cooldown")
	}
	if parked := th.RecordFailure(url); parked {
		t.Fatal("expected the failure count to have been reset")
	}
}

func TestThrottleSuccessClearsFailures(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	th := newTestThrottle(0, 2, time.Minute, &now)
	const url = "https://example.com/hook"

	th.RecordFailure(url)
	th.RecordSuccess(url)
	if parked := th.RecordFailure(url); parked {
		t.Fatal("a success should have reset the failure count")
	}
}

func TestThrottleDisabledLimits(t *testing.T) {
	now := time.Now()
	th := newTestThrottle(0, 0, time.Minute, &now)

	for i := 0; i < 100; i++ {
		if ok, _ := th.Allow("https://example.com/hook"); !ok {
			t.Fatal("expected no rate limit when perMinute is zero")
		}
		if th.RecordFailure("https://example.com/hook") {
			t.Fatal("expected no circuit breaker when the threshold is zero")
		}
	}
}

func TestThrottlePrunesIdleDestinations(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	th := newTestThrottle(10, 0, time.Minute, &now)

	th.Allow("https://old.example.com/hook")
	now = now.Add(idleTTL + time.Minute)
	th.Allow("https://new.example.com/hook")

	th.mu.Lock()
	defer th.mu.Unlock()
	if _, ok := th.hosts["old.example.com"]; ok {
		t.Fatal("expected the idle destination to be pruned")
	}
}

func TestNilThrottleAllows(t *testing.T) {
	var th *Throttle
	if ok, _ := th.Allow("https://example.com/hook"); !ok {
		t.Fatal("a nil throttle should allow everything")
	}
	th.RecordSuccess("https://example.com/hook")
	if th.RecordFailure("https://example.com/hook") {
		t.Fatal("a nil throttle should never park a destination")
	}
}
