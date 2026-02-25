package storage

import (
	"errors"

	"donetick.com/core/config"
)

type URLSigner interface {
	Sign(rawPath string) (string, error)
	IsValid(rawPath string, providedSig string) bool
}

func NewURLSigner(storage Storage, config *config.Config) (URLSigner, error) {

	switch storage := storage.(type) {
	case *LocalStorage:
		return NewURLSignerLocal(config), nil
	case *S3Storage:
		return NewURLSignerS3(storage, config), nil
	default:
		return nil, errors.New("unsupported storage type for URL signing")
	}
}
