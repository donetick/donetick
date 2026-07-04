package storage

import (
	"context"
	"time"

	storageModel "donetick.com/core/internal/storage/model"
	storageRepo "donetick.com/core/internal/storage/repo"
	"donetick.com/core/logging"
)

type DraftCleanupService struct {
	storage     Storage
	storageRepo *storageRepo.StorageRepository
	ticker      *time.Ticker
	done        chan bool
}

func NewDraftCleanupService(storage Storage, repo *storageRepo.StorageRepository) *DraftCleanupService {
	return &DraftCleanupService{
		storage:     storage,
		storageRepo: repo,
		ticker:      time.NewTicker(24 * time.Hour),
		done:        make(chan bool),
	}
}

func (s *DraftCleanupService) Start(ctx context.Context) {
	logger := logging.FromContext(ctx)
	logger.Info("Draft attachment cleanup service started")

	go func() {
		for {
			select {
			case <-s.done:
				logger.Info("Draft attachment cleanup service stopped")
				return
			case <-s.ticker.C:
				s.run(ctx)
			}
		}
	}()
}

func (s *DraftCleanupService) Stop() {
	s.ticker.Stop()
	s.done <- true
}

func (s *DraftCleanupService) run(ctx context.Context) {
	logger := logging.FromContext(ctx)

	cutoff := time.Now().UTC().Add(-24 * time.Hour).Unix()
	files, err := s.storageRepo.GetExpiredDraftFiles(ctx, cutoff)
	if err != nil {
		logger.Errorw("Failed to query expired draft attachments", "error", err)
		return
	}
	if len(files) == 0 {
		return
	}

	// group by userID so RemoveFileRecords can update per-user storage usage correctly
	byUser := make(map[int][]*storageModel.StorageFile)
	for _, f := range files {
		byUser[f.UserID] = append(byUser[f.UserID], f)
	}

	cleaned := 0
	for userID, group := range byUser {
		paths := make([]string, len(group))
		for i, f := range group {
			paths[i] = f.FilePath
		}

		if err := s.storage.Delete(ctx, paths); err != nil {
			logger.Errorw("Failed to delete expired draft files from storage", "userID", userID, "error", err)
			continue
		}

		if err := s.storageRepo.RemoveFileRecords(ctx, group, userID); err != nil {
			logger.Errorw("Failed to remove expired draft file records", "userID", userID, "error", err)
			continue
		}
		cleaned += len(group)
	}

	logger.Infow("Expired draft attachments cleaned up", "count", cleaned)
}
