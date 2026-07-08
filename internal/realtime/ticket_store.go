package realtime

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	config "donetick.com/core/config"
	"donetick.com/core/logging"
)

const (
	// ticketTTL is how long an issued SSE ticket remains valid. Tickets are meant
	// to be exchanged for an SSE connection immediately after being minted, so a
	// short lifetime keeps the exposure window (e.g. if the ticket leaks into
	// proxy access logs via the query string) very small.
	ticketTTL = 60 * time.Second
)

// ErrTicketStoreFull is returned by Issue when the store is at capacity and no
// expired tickets could be reclaimed.
var ErrTicketStoreFull = errors.New("ticket store is full")

type ticketEntry struct {
	userID    int
	expiresAt time.Time
}

// TicketStore holds short-lived, single-use tickets that native EventSource
// clients (which cannot send custom Authorization headers) exchange for an
// authenticated SSE connection by passing the ticket as a query parameter.
type TicketStore struct {
	mu         sync.Mutex
	tickets    map[string]ticketEntry
	maxTickets int
	stopCh     chan struct{}
	stopOnce   sync.Once
}

// NewTicketStore creates a TicketStore and starts a background cleanup routine.
func NewTicketStore(cfg *config.Config) *TicketStore {
	max := cfg.RealTimeConfig.MaxSSETickets
	if max <= 0 {
		max = 10000
	}
	s := &TicketStore{
		tickets:    make(map[string]ticketEntry),
		maxTickets: max,
		stopCh:     make(chan struct{}),
	}
	go s.cleanupRoutine()
	return s
}

// Stop halts the background cleanup routine. It is safe to call multiple times.
func (s *TicketStore) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

// Issue creates a new single-use ticket bound to the given user and returns it.
func (s *TicketStore) Issue(userID int) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	ticket := hex.EncodeToString(buf)

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.tickets) >= s.maxTickets {
		// Try to reclaim space by purging anything already expired before
		// rejecting the request outright.
		removed := s.purgeExpiredLocked()
		if len(s.tickets) >= s.maxTickets {
			logging.DefaultLogger().Warnw("SSE ticket store full, rejecting issue",
				"size", len(s.tickets),
				"max", s.maxTickets,
				"purged", removed,
			)
			return "", ErrTicketStoreFull
		}
	}

	s.tickets[ticket] = ticketEntry{
		userID:    userID,
		expiresAt: time.Now().UTC().Add(ticketTTL),
	}

	return ticket, nil
}

// Consume validates a ticket and returns the associated user ID. Tickets are
// single-use: a successful lookup removes the ticket so it cannot be replayed.
func (s *TicketStore) Consume(ticket string) (int, bool) {
	if ticket == "" {
		return 0, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.tickets[ticket]
	if !ok {
		return 0, false
	}

	// Always delete on lookup so it cannot be reused, even if expired.
	delete(s.tickets, ticket)

	if time.Now().UTC().After(entry.expiresAt) {
		return 0, false
	}

	return entry.userID, true
}

// purgeExpiredLocked removes all expired tickets. The caller must hold s.mu.
func (s *TicketStore) purgeExpiredLocked() int {
	now := time.Now().UTC()
	removed := 0
	for ticket, entry := range s.tickets {
		if now.After(entry.expiresAt) {
			delete(s.tickets, ticket)
			removed++
		}
	}
	return removed
}

func (s *TicketStore) cleanupRoutine() {
	ticker := time.NewTicker(ticketTTL)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			removed := s.purgeExpiredLocked()
			remaining := len(s.tickets)
			s.mu.Unlock()

			if removed > 0 {
				logging.DefaultLogger().Debugw("SSE ticket store cleanup",
					"removed", removed,
					"remaining", remaining,
				)
			}
		}
	}
}
