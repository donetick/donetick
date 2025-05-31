package mfa

import (
	"context"
	"time"

	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
)

type CleanupService struct {
	userRepo *uRepo.UserRepository
	ticker   *time.Ticker
	done     chan bool
}

func NewCleanupService(userRepo *uRepo.UserRepository) *CleanupService {
	return &CleanupService{
		userRepo: userRepo,
		ticker:   time.NewTicker(60 * time.Minute),
		done:     make(chan bool),
	}
}

func (s *CleanupService) Start(ctx context.Context) {
	logger := logging.FromContext(ctx)
	logger.Info("MFA cleanup service started")

	go func() {
		for {
			select {
			case <-s.done:
				logger.Info("MFA cleanup service stopped")
				return
			case <-s.ticker.C:
				if err := s.userRepo.CleanupExpiredMFASessions(ctx); err != nil {
					logger.Errorw("Failed to cleanup expired MFA sessions", "error", err)
				} else {
					logger.Debug("Successfully cleaned up expired MFA sessions")
				}
			}
		}
	}()
}

// Stop stops the cleanup service
func (s *CleanupService) Stop() {
	s.ticker.Stop()
	s.done <- true
}
