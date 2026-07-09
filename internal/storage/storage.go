package storage

import (
	"context"
	"io"
)

type Storage interface {
	Save(ctx context.Context, path string, file io.Reader) error
	// SavePublic writes to the public bucket (falls back to the private bucket
	// if no public bucket is configured).
	SavePublic(ctx context.Context, path string, file io.Reader) error
	Delete(ctx context.Context, paths []string) error
	GetURL(ctx context.Context, path string) (string, error)
	// GetPublicURL returns a bare, unsigned URL for objects saved via SavePublic.
	// Falls back to a presigned URL when no public CDN host is configured.
	GetPublicURL(ctx context.Context, path string) (string, error)
	// DeletePublicByURL deletes the object identified by a public URL previously
	// returned by GetPublicURL. If the URL does not belong to this storage
	// backend (e.g. an external OIDC picture URL) the call is a no-op.
	DeletePublicByURL(ctx context.Context, rawURL string) error
	Get(ctx context.Context, path string) (io.ReadCloser, error)
}
