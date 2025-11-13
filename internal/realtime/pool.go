package realtime

import (
	"sync"
	"time"

	"donetick.com/core/config"
)

// ConnectionPool manages WebSocket connections for a specific circle
type ConnectionPool struct {
	CircleID    int
	connections map[string]*Connection
	userConns   map[int][]*Connection // userID -> connections
	mu          sync.RWMutex
	config      *config.RealTimeConfig
	stats       ConnectionPoolStats
}

// ConnectionPoolStats tracks metrics for a connection pool
type ConnectionPoolStats struct {
	ActiveConnections int64
	TotalMessages     int64
	mu                sync.RWMutex
}

// NewConnectionPool creates a new connection pool for a circle
func NewConnectionPool(circleID int, config *config.RealTimeConfig) *ConnectionPool {
	return &ConnectionPool{
		CircleID:    circleID,
		connections: make(map[string]*Connection),
		userConns:   make(map[int][]*Connection),
		config:      config,
		stats:       ConnectionPoolStats{},
	}
}

// AddConnection adds a connection to the pool
func (p *ConnectionPool) AddConnection(conn *Connection) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check max connections limit
	if len(p.connections) >= p.config.MaxConnections {
		return ErrMaxConnectionsReached
	}

	// Check per-user connection limit
	userConnections := p.userConns[conn.UserID]
	if len(userConnections) >= p.config.MaxConnectionsPerUser {
		return ErrUserMaxConnectionsReached
	}

	// Add connection
	p.connections[conn.ID] = conn
	p.userConns[conn.UserID] = append(p.userConns[conn.UserID], conn)

	p.stats.mu.Lock()
	p.stats.ActiveConnections++
	p.stats.mu.Unlock()

	return nil
}

// RemoveConnection removes a connection from the pool
func (p *ConnectionPool) RemoveConnection(conn *Connection) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Remove from connections map
	delete(p.connections, conn.ID)

	// Remove from user connections
	userConnections := p.userConns[conn.UserID]
	for i, userConn := range userConnections {
		if userConn.ID == conn.ID {
			// Remove from slice efficiently
			lastIdx := len(userConnections) - 1
			userConnections[i] = userConnections[lastIdx]
			userConnections[lastIdx] = nil // Prevent memory leak
			p.userConns[conn.UserID] = userConnections[:lastIdx]
			break
		}
	}

	// Clean up empty user connection slice to prevent memory accumulation
	if len(p.userConns[conn.UserID]) == 0 {
		delete(p.userConns, conn.UserID)
	}

	p.stats.mu.Lock()
	if p.stats.ActiveConnections > 0 {
		p.stats.ActiveConnections--
	}
	p.stats.mu.Unlock()
}

// Broadcast sends an event to all connections in the pool
func (p *ConnectionPool) Broadcast(event *Event) {
	p.mu.RLock()
	connections := make([]*Connection, 0, len(p.connections))
	for _, conn := range p.connections {
		if !conn.IsClosed() {
			connections = append(connections, conn)
		}
	}
	p.mu.RUnlock()

	// Send to connections outside of the lock
	for _, conn := range connections {
		if conn.SendEvent(event) {
			p.stats.mu.Lock()
			p.stats.TotalMessages++
			p.stats.mu.Unlock()
		}
	}
}

// BroadcastToUser sends an event to all connections for a specific user
func (p *ConnectionPool) BroadcastToUser(userID int, event *Event) {
	p.mu.RLock()
	userConnections := make([]*Connection, len(p.userConns[userID]))
	copy(userConnections, p.userConns[userID])
	p.mu.RUnlock()

	for _, conn := range userConnections {
		if !conn.IsClosed() {
			if conn.SendEvent(event) {
				p.stats.mu.Lock()
				p.stats.TotalMessages++
				p.stats.mu.Unlock()
			}
		}
	}
}

// GetConnection returns a connection by ID
func (p *ConnectionPool) GetConnection(connectionID string) (*Connection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	conn, exists := p.connections[connectionID]
	return conn, exists
}

// GetUserConnections returns all connections for a user
func (p *ConnectionPool) GetUserConnections(userID int) []*Connection {
	p.mu.RLock()
	defer p.mu.RUnlock()

	userConnections := p.userConns[userID]
	result := make([]*Connection, len(userConnections))
	copy(result, userConnections)
	return result
}

// CleanupStaleConnections removes connections that haven't been active
func (p *ConnectionPool) CleanupStaleConnections(threshold time.Duration) {
	p.mu.RLock()
	staleConnections := make([]*Connection, 0)
	for _, conn := range p.connections {
		if conn.IsStale(threshold) {
			staleConnections = append(staleConnections, conn)
		}
	}
	p.mu.RUnlock()

	// Close stale connections
	for _, conn := range staleConnections {
		conn.Close()
		p.RemoveConnection(conn)
	}
}

// IsEmpty returns true if the pool has no active connections
func (p *ConnectionPool) IsEmpty() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.connections) == 0
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, conn := range p.connections {
		conn.Close()
	}

	p.connections = make(map[string]*Connection)
	p.userConns = make(map[int][]*Connection)

	p.stats.mu.Lock()
	p.stats.ActiveConnections = 0
	p.stats.mu.Unlock()
}

// GetStats returns current pool statistics
func (p *ConnectionPool) GetStats() ConnectionPoolStats {
	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()
	return p.stats
}

// GetConnectionCount returns the number of active connections
func (p *ConnectionPool) GetConnectionCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.connections)
}

// GetUserCount returns the number of unique users connected
func (p *ConnectionPool) GetUserCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.userConns)
}
