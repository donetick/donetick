package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"donetick.com/core/config"
)

type LocalStorage struct {
	PublicHost string
	BasePath   string
}

func NewLocalStorage(config *config.Config) (*LocalStorage, error) {
	return &LocalStorage{PublicHost: config.Storage.PublicHost + "api/v1/assets", BasePath: "./assets"}, nil //TODO: do error checking in case the path is invalid, no permissions, etc.
}

func sanitizePath(p string) (string, error) {
	clean := filepath.Clean(p)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
		return "", errors.New("invalid asset path")
	}
	if strings.Contains(clean, ".."+string(os.PathSeparator)) {
		return "", errors.New("invalid asset path")
	}
	return clean, nil
}

func (l *LocalStorage) Save(ctx context.Context, path string, file io.Reader) error {
	cleanPath, err := sanitizePath(path)
	if err != nil {
		return err
	}
	fullPath := filepath.Join(l.BasePath, cleanPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), os.ModePerm); err != nil {
		return err
	}
	outFile, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	_, err = io.Copy(outFile, file)
	return err
}

func (l *LocalStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	cleanPath, err := sanitizePath(path)
	if err != nil {
		return nil, err
	}
	fullPath := filepath.Join(l.BasePath, cleanPath)
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	return file, nil
}
func (l *LocalStorage) Delete(ctx context.Context, paths []string) error {
	var errs []string
	for _, path := range paths {
		cleanPath, err := sanitizePath(path)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if err := os.Remove(filepath.Join(l.BasePath, cleanPath)); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (l *LocalStorage) GetURL(ctx context.Context, path string) (string, error) {
	cleanPath, err := sanitizePath(path)
	if err != nil {
		return "", err
	}
	return filepath.Join(l.BasePath, cleanPath), nil
}
