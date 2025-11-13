package realtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/logging"
	"go.uber.org/zap"
)

// RealTimeService manages WebSocket connections and event broadcasting
type RealTimeService struct {
	config          *config.RealTimeConfig
	connectionPools map[int]*ConnectionPool // circleID -> ConnectionPool
	broadcaster     *EventBroadcaster
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	logger          *zap.SugaredLogger
	started         bool
	stats           *ServiceStats
}

// ServiceStats tracks real-time service metrics
type ServiceStats struct {
	TotalConnections   int64
	ActiveConnections  int64
	EventsPublished    int64
	EventsDelivered    int64
	ConnectionsDropped int64
	CirclesActive      int64
	mu                 sync.RWMutex
}

// NewRealTimeService creates a new real-time service instance
func NewRealTimeService(cfg *config.Config) *RealTimeService {
	// Validate configuration
	if err := validateConfig(&cfg.RealTimeConfig); err != nil {
		panic(fmt.Sprintf("Invalid real-time configuration: %v", err))
	}

	ctx, cancel := context.WithCancel(context.Background())

	service := &RealTimeService{
		config:          &cfg.RealTimeConfig,
		connectionPools: make(map[int]*ConnectionPool),
		mu:              sync.RWMutex{},
		ctx:             ctx,
		cancel:          cancel,
		started:         false,
		stats:           &ServiceStats{},
	}

	service.broadcaster = NewEventBroadcaster(service, cfg)

	return service
}

// Start initializes the real-time service
func (s *RealTimeService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	s.logger = logging.FromContext(ctx)
	s.logger.Info("Starting real-time service")

	if !s.config.Enabled {
		s.logger.Info("Real-time service is disabled in configuration")
		return nil
	}

	// Start cleanup routine
	go s.cleanupRoutine()

	s.started = true
	s.logger.Infow("Real-time service started",
		"websocket_enabled", s.config.WebSocketEnabled,
		"max_connections", s.config.MaxConnections)

	return nil
}

// Stop gracefully shuts down the real-time service
func (s *RealTimeService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	s.logger.Info("Stopping real-time service")

	// Cancel context to signal shutdown
	s.cancel()

	// Close all connection pools
	for circleID, pool := range s.connectionPools {
		pool.Close()
		delete(s.connectionPools, circleID)
	}

	s.started = false
	s.logger.Info("Real-time service stopped")

	return nil
}

// GetEventBroadcaster returns the event broadcaster
func (s *RealTimeService) GetEventBroadcaster() *EventBroadcaster {
	return s.broadcaster
}

// GetConnectionPool gets or creates a connection pool for a circle
func (s *RealTimeService) GetConnectionPool(circleID int) *ConnectionPool {
	s.mu.Lock()
	defer s.mu.Unlock()

	pool, exists := s.connectionPools[circleID]
	if !exists {
		pool = NewConnectionPool(circleID, s.config)
		s.connectionPools[circleID] = pool
		s.updateStats()
	}

	return pool
}

// AddConnection adds a WebSocket connection to the appropriate circle pool
func (s *RealTimeService) AddConnection(conn *Connection) error {
	if !s.started || !s.config.Enabled {
		return ErrServiceNotEnabled
	}

	pool := s.GetConnectionPool(conn.CircleID)
	return pool.AddConnection(conn)
}

// RemoveConnection removes a WebSocket connection from its circle pool
func (s *RealTimeService) RemoveConnection(conn *Connection) {
	s.mu.RLock()
	pool, exists := s.connectionPools[conn.CircleID]
	s.mu.RUnlock()

	if exists {
		pool.RemoveConnection(conn)
		s.updateStats()
	}
}

// BroadcastToCircle sends an event to all connections in a specific circle
func (s *RealTimeService) BroadcastToCircle(circleID int, event *Event) {
	if !s.started || !s.config.Enabled {
		return
	}

	s.mu.RLock()
	pool, exists := s.connectionPools[circleID]
	s.mu.RUnlock()

	if exists {
		pool.Broadcast(event)
		s.stats.mu.Lock()
		s.stats.EventsPublished++
		s.stats.mu.Unlock()
	}
}

// GetStats returns current service statistics
func (s *RealTimeService) GetStats() ServiceStats {
	s.stats.mu.RLock()
	defer s.stats.mu.RUnlock()

	// Create a copy without the mutex to avoid copying sync.RWMutex
	return ServiceStats{
		TotalConnections:   s.stats.TotalConnections,
		ActiveConnections:  s.stats.ActiveConnections,
		EventsPublished:    s.stats.EventsPublished,
		EventsDelivered:    s.stats.EventsDelivered,
		ConnectionsDropped: s.stats.ConnectionsDropped,
		CirclesActive:      s.stats.CirclesActive,
		// Note: deliberately omitting 'mu' field to avoid copying mutex
	}
}

// cleanupRoutine periodically cleans up stale connections
func (s *RealTimeService) cleanupRoutine() {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.performCleanup()
		}
	}
}

// performCleanup removes stale connections and empty pools
func (s *RealTimeService) performCleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for circleID, pool := range s.connectionPools {
		// Clean up stale connections in the pool
		pool.CleanupStaleConnections(s.config.StaleThreshold)

		// Remove empty pools
		if pool.IsEmpty() {
			pool.Close()
			delete(s.connectionPools, circleID)
		}
	}

	s.updateStats()
}

// updateStats recalculates service statistics
func (s *RealTimeService) updateStats() {
	var activeConnections int64

	for _, pool := range s.connectionPools {
		stats := pool.GetStats()
		activeConnections += stats.ActiveConnections
	}

	s.stats.mu.Lock()
	s.stats.ActiveConnections = activeConnections
	s.stats.CirclesActive = int64(len(s.connectionPools))
	s.stats.mu.Unlock()
}

// IsHealthy returns true if the service is running properly
func (s *RealTimeService) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started && s.config.Enabled
}

// validateConfig validates real-time service configuration
func validateConfig(cfg *config.RealTimeConfig) error {
	if cfg.MaxConnections <= 0 {
		return fmt.Errorf("maxConnections must be positive, got %d", cfg.MaxConnections)
	}

	if cfg.MaxConnectionsPerUser <= 0 {
		return fmt.Errorf("maxConnectionsPerUser must be positive, got %d", cfg.MaxConnectionsPerUser)
	}

	if cfg.MaxConnectionsPerUser > cfg.MaxConnections {
		return fmt.Errorf("maxConnectionsPerUser (%d) cannot exceed maxConnections (%d)",
			cfg.MaxConnectionsPerUser, cfg.MaxConnections)
	}

	if cfg.EventQueueSize <= 0 {
		return fmt.Errorf("eventQueueSize must be positive, got %d", cfg.EventQueueSize)
	}

	if cfg.CleanupInterval <= 0 {
		return fmt.Errorf("cleanupInterval must be positive, got %v", cfg.CleanupInterval)
	}

	if cfg.StaleThreshold <= 0 {
		return fmt.Errorf("staleThreshold must be positive, got %v", cfg.StaleThreshold)
	}

	if cfg.StaleThreshold <= cfg.CleanupInterval {
		return fmt.Errorf("staleThreshold (%v) should be greater than cleanupInterval (%v)",
			cfg.StaleThreshold, cfg.CleanupInterval)
	}

	return nil
}
