package webhook

import (
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

// idleTTL is how long a destination with no traffic is kept before its state
// is dropped, so the map cannot grow with every URL ever configured.
const idleTTL = time.Hour

// Throttle bounds what a single destination can be made to receive.
//
// It exists for two reasons, and they want different granularity:
//
// A webhook URL is attacker-chosen, so without a limit the instance can be
// pointed at a third party and used to amplify traffic at them. That budget
// is per host, because varying the path would otherwise buy a fresh one.
//
// A receiver that is down or gone would soak up a worker on every event
// forever, so after enough consecutive failures it is parked for a cooldown.
// That state is per endpoint: one circle's dead webhook path must not pause
// another circle's working integration on the same host.
type Throttle struct {
	perMinute        int
	failureThreshold int
	cooldown         time.Duration

	// now is a seam for tests.
	now func() time.Time

	mu        sync.Mutex
	hosts     map[string]*hostState
	endpoints map[string]*endpointState
}

type hostState struct {
	windowStart time.Time
	count       int
	lastSeen    time.Time
}

type endpointState struct {
	failures     int
	blockedUntil time.Time
	lastSeen     time.Time
}

// NewThrottle returns a throttle. A non-positive perMinute or
// failureThreshold turns off that half of it.
func NewThrottle(perMinute, failureThreshold int, cooldown time.Duration) *Throttle {
	if cooldown <= 0 {
		cooldown = 15 * time.Minute
	}
	return &Throttle{
		perMinute:        perMinute,
		failureThreshold: failureThreshold,
		cooldown:         cooldown,
		now:              time.Now,
		hosts:            make(map[string]*hostState),
		endpoints:        make(map[string]*endpointState),
	}
}

// Allow reports whether a delivery to rawURL may be attempted, and if not,
// why. The reason is meant for the log, not for the user.
func (t *Throttle) Allow(rawURL string) (bool, string) {
	if t == nil {
		return true, ""
	}

	hostKey, endpointKey := throttleKeys(rawURL)
	now := t.now()

	t.mu.Lock()
	defer t.mu.Unlock()

	t.pruneLocked(now)

	if e := t.endpoints[endpointKey]; e != nil {
		e.lastSeen = now
		if !e.blockedUntil.IsZero() {
			if now.Before(e.blockedUntil) {
				return false, "destination is in failure cooldown"
			}
			// Cooldown is over: give the endpoint a clean slate so
			// one more failure does not immediately park it again.
			e.blockedUntil = time.Time{}
			e.failures = 0
		}
	}

	if t.perMinute > 0 {
		h := t.hosts[hostKey]
		if h == nil {
			h = &hostState{windowStart: now}
			t.hosts[hostKey] = h
		}
		h.lastSeen = now

		if now.Sub(h.windowStart) >= time.Minute {
			h.windowStart = now
			h.count = 0
		}
		if h.count >= t.perMinute {
			return false, "destination rate limit reached"
		}
		h.count++
	}

	return true, ""
}

// RecordSuccess clears the failure count for an endpoint.
func (t *Throttle) RecordSuccess(rawURL string) {
	if t == nil {
		return
	}
	_, endpointKey := throttleKeys(rawURL)

	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.endpoints, endpointKey)
}

// RecordFailure counts a failed delivery and parks the endpoint once it has
// failed failureThreshold times in a row. It reports whether this call was
// the one that parked it, so the caller can log that once.
func (t *Throttle) RecordFailure(rawURL string) bool {
	if t == nil || t.failureThreshold <= 0 {
		return false
	}
	_, endpointKey := throttleKeys(rawURL)
	now := t.now()

	t.mu.Lock()
	defer t.mu.Unlock()

	e := t.endpoints[endpointKey]
	if e == nil {
		e = &endpointState{}
		t.endpoints[endpointKey] = e
	}
	e.lastSeen = now
	e.failures++

	if e.failures >= t.failureThreshold && e.blockedUntil.IsZero() {
		e.blockedUntil = now.Add(t.cooldown)
		return true
	}
	return false
}

func (t *Throttle) pruneLocked(now time.Time) {
	for k, h := range t.hosts {
		if now.Sub(h.lastSeen) > idleTTL {
			delete(t.hosts, k)
		}
	}
	for k, e := range t.endpoints {
		if now.Sub(e.lastSeen) > idleTTL && now.After(e.blockedUntil) {
			delete(t.endpoints, k)
		}
	}
}

// throttleKeys returns the host the rate limit is counted against and the
// endpoint the failure state belongs to.
//
// Both are normalized: "example.com", "EXAMPLE.COM", "example.com." and
// "example.com:443" all reach the same server, so they have to share one
// budget rather than each getting their own.
func throttleKeys(rawURL string) (host, endpoint string) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL, rawURL
	}

	scheme := strings.ToLower(u.Scheme)
	host = strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if port := u.Port(); port != "" && !isDefaultPort(scheme, port) {
		host = net.JoinHostPort(host, port)
	}

	// The path and query identify the individual integration: a Discord
	// webhook, one circle's receiver path, a Slack-style token URL.
	endpoint = scheme + "://" + host + u.EscapedPath()
	if u.RawQuery != "" {
		endpoint += "?" + u.RawQuery
	}
	return host, endpoint
}

func isDefaultPort(scheme, port string) bool {
	return (scheme == "http" && port == "80") || (scheme == "https" && port == "443")
}
