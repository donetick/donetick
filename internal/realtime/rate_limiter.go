package realtime

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter for WebSocket connections
type RateLimiter struct {
	connections map[string]*ConnectionRateLimit
	mu          sync.RWMutex
	cleanupTick *time.Ticker
	done        chan struct{}
}

// ConnectionRateLimit tracks rate limiting for a specific connection
type ConnectionRateLimit struct {
	tokens     int
	maxTokens  int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxTokens int, refillInterval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		connections: make(map[string]*ConnectionRateLimit),
		cleanupTick: time.NewTicker(5 * time.Minute), // Cleanup old connections every 5 minutes
		done:        make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a connection is allowed to perform an action
func (rl *RateLimiter) Allow(connectionID string, maxTokens int, refillRate time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	conn, exists := rl.connections[connectionID]
	if !exists {
		conn = &ConnectionRateLimit{
			tokens:     maxTokens,
			maxTokens:  maxTokens,
			lastRefill: time.Now(),
		}
		rl.connections[connectionID] = conn
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(conn.lastRefill)
	tokensToAdd := int(elapsed / refillRate)

	if tokensToAdd > 0 {
		conn.tokens = min(conn.maxTokens, conn.tokens+tokensToAdd)
		conn.lastRefill = now
	}

	// Check if we have tokens available
	if conn.tokens > 0 {
		conn.tokens--
		return true
	}

	return false
}

// Remove removes a connection from rate limiting
func (rl *RateLimiter) Remove(connectionID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.connections, connectionID)
}

// Stop stops the rate limiter and cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.done)
	rl.cleanupTick.Stop()
}

// cleanup removes old inactive connections
func (rl *RateLimiter) cleanup() {
	for {
		select {
		case <-rl.done:
			return
		case <-rl.cleanupTick.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-10 * time.Minute) // Remove connections inactive for 10+ minutes

			for id, conn := range rl.connections {
				conn.mu.Lock()
				if conn.lastRefill.Before(cutoff) {
					delete(rl.connections, id)
				}
				conn.mu.Unlock()
			}
			rl.mu.Unlock()
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
