package storage

import "errors"

var (
	ErrNotEnoughSpace   = errors.New("not enough space")
	ErrFileSizeTooLarge = errors.New("file size too large")
)
