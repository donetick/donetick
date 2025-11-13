package realtime

import (
	"errors"
)

var (
	// Service errors
	ErrServiceNotEnabled         = errors.New("real-time service is not enabled")
	ErrServiceNotStarted         = errors.New("real-time service is not started")
	ErrMaxConnectionsReached     = errors.New("maximum connections reached")
	ErrUserMaxConnectionsReached = errors.New("maximum connections per user reached")

	// Connection errors
	ErrConnectionClosed   = errors.New("connection is closed")
	ErrInvalidCircleID    = errors.New("invalid circle ID")
	ErrUnauthorizedCircle = errors.New("user not authorized for circle")
	ErrInvalidEventType   = errors.New("invalid event type")

	// Authentication errors
	ErrInvalidToken = errors.New("invalid authentication token")
	ErrTokenExpired = errors.New("authentication token expired")
	ErrMissingToken = errors.New("missing authentication token")
)
