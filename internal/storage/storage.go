package storage

import (
	"context"
	"errors"
	"io"

	"donetick.com/core/config"
)

type Storage interface {
	Save(ctx context.Context, path string, file io.Reader) error
	Delete(ctx context.Context, paths []string) error
	GetURL(ctx context.Context, path string) (string, error)
	Get(ctx context.Context, path string) (io.ReadCloser, error)
}

func NewStorage(config *config.Config) (Storage, error) {
	if config.Storage.Local != nil && config.Storage.Local.BasePath != "" {
		return NewLocalStorage(config)
	}

	if config.Storage.AWS != nil {
		return NewS3Storage(config)
	}
	return nil, errors.New("no storage configuration found.")
}
