package webhook

import (
	"net/url"
	"sync"
	"time"
)

// idleTTL is how long a destination with no traffic is kept before its state
// is dropped, so the map cannot grow with every URL ever configured.
const idleTTL = time.Hour

// Throttle bounds what a single destination can be made to receive.
//
// It exists for two reasons. A webhook URL is attacker-chosen, so without a
// limit the instance can be pointed at a third party and used to amplify
// traffic at them. And a receiver that is down or gone would otherwise soak
// up a worker on every event forever; after enough consecutive failures the
// destination is parked for a cooldown.
type Throttle struct {
	perMinute        int
	failureThreshold int
	cooldown         time.Duration

	// now is a seam for tests.
	now func() time.Time

	mu    sync.Mutex
	state map[string]*destState
}

type destState struct {
	windowStart  time.Time
	count        int
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
		state:            make(map[string]*destState),
	}
}

// Allow reports whether a delivery to rawURL may be attempted, and if not,
// why. The reason is meant for the log, not for the user.
func (t *Throttle) Allow(rawURL string) (bool, string) {
	if t == nil {
		return true, ""
	}

	key := destinationKey(rawURL)
	now := t.now()

	t.mu.Lock()
	defer t.mu.Unlock()

	t.pruneLocked(now)

	s := t.state[key]
	if s == nil {
		s = &destState{windowStart: now}
		t.state[key] = s
	}
	s.lastSeen = now

	if !s.blockedUntil.IsZero() {
		if now.Before(s.blockedUntil) {
			return false, "destination is in failure cooldown"
		}
		// Cooldown is over: give the destination a clean slate so one
		// more failure does not immediately park it again.
		s.blockedUntil = time.Time{}
		s.failures = 0
	}

	if t.perMinute > 0 {
		if now.Sub(s.windowStart) >= time.Minute {
			s.windowStart = now
			s.count = 0
		}
		if s.count >= t.perMinute {
			return false, "destination rate limit reached"
		}
		s.count++
	}

	return true, ""
}

// RecordSuccess clears the failure count for a destination.
func (t *Throttle) RecordSuccess(rawURL string) {
	if t == nil {
		return
	}
	key := destinationKey(rawURL)

	t.mu.Lock()
	defer t.mu.Unlock()

	if s := t.state[key]; s != nil {
		s.failures = 0
		s.blockedUntil = time.Time{}
	}
}

// RecordFailure counts a failed delivery and parks the destination once it
// has failed failureThreshold times in a row. It reports whether this call
// was the one that parked it, so the caller can log that once.
func (t *Throttle) RecordFailure(rawURL string) bool {
	if t == nil || t.failureThreshold <= 0 {
		return false
	}
	key := destinationKey(rawURL)
	now := t.now()

	t.mu.Lock()
	defer t.mu.Unlock()

	s := t.state[key]
	if s == nil {
		s = &destState{windowStart: now}
		t.state[key] = s
	}
	s.lastSeen = now
	s.failures++

	if s.failures >= t.failureThreshold && s.blockedUntil.IsZero() {
		s.blockedUntil = now.Add(t.cooldown)
		return true
	}
	return false
}

func (t *Throttle) pruneLocked(now time.Time) {
	for k, s := range t.state {
		if now.Sub(s.lastSeen) > idleTTL && now.After(s.blockedUntil) {
			delete(t.state, k)
		}
	}
}

// destinationKey groups limits by host, not by full URL: a sender aimed at a
// third party would otherwise just vary the path to get a fresh budget.
func destinationKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Host
}
