package storage

import (
	"context"
	"io"
)

type Storage interface {
	Save(ctx context.Context, path string, file io.Reader) error
	Delete(ctx context.Context, paths []string) error
	GetURL(ctx context.Context, path string) (string, error)
	Get(ctx context.Context, path string) (io.ReadCloser, error)
}
