package storage

import (
	"fmt"

	"donetick.com/core/config"
)

// NewStorage returns a Storage implementation based on config.Storage.StorageType.
// "local" uses the filesystem; anything else (including empty) uses S3.
func NewStorage(cfg *config.Config) (Storage, error) {
	switch cfg.Storage.StorageType {
	case "local":
		return NewLocalStorage(cfg), nil
	default:
		return NewS3Storage(cfg)
	}
}

// NewURLSigner returns a URLSigner paired with the given Storage.
// For local storage it uses HMAC-SHA256; for S3 it uses SigV4 presigning.
func NewURLSigner(cfg *config.Config, s Storage) (URLSigner, error) {
	switch cfg.Storage.StorageType {
	case "local":
		return NewURLSignerLocal(cfg), nil
	default:
		s3, ok := s.(*S3Storage)
		if !ok {
			return nil, fmt.Errorf("S3 URLSigner requires *S3Storage, got %T", s)
		}
		return NewURLSignerS3(s3, cfg), nil
	}
}
