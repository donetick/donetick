package user

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"donetick.com/core/config"
	"donetick.com/core/internal/storage"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"github.com/gin-gonic/gin"
)

// fakePresignedStorage's GetPublicURL mimics S3Storage's private-bucket
// fallback: a resolved URL with query params, distinct from the raw key.
// If updateProfilePhoto regresses to persisting GetPublicURL's result
// instead of the bare storage key, this makes the difference observable.
type fakePresignedStorage struct {
	deletedPaths []string
}

var _ storage.Storage = (*fakePresignedStorage)(nil)

func (f *fakePresignedStorage) Save(ctx context.Context, path string, file io.Reader) error {
	return nil
}

func (f *fakePresignedStorage) SavePublic(ctx context.Context, path string, file io.Reader) error {
	return nil
}

func (f *fakePresignedStorage) Delete(ctx context.Context, paths []string) error {
	f.deletedPaths = append(f.deletedPaths, paths...)
	return nil
}

func (f *fakePresignedStorage) GetURL(ctx context.Context, path string) (string, error) {
	return f.presign(path), nil
}

func (f *fakePresignedStorage) GetPublicURL(ctx context.Context, path string) (string, error) {
	// Simulates S3Storage.GetPublicURL's fallback when no public_host/
	// public_bucket is configured: an already-presigned URL, not a bare key.
	return f.presign(path), nil
}

func (f *fakePresignedStorage) presign(path string) string {
	return fmt.Sprintf("https://minio.example.com/%s?X-Amz-Expires=604800&X-Amz-Signature=deadbeef", path)
}

func (f *fakePresignedStorage) DeletePublicByURL(ctx context.Context, rawURL string) error {
	return nil
}

func (f *fakePresignedStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

// noopSigner is a minimal storage.URLSigner good enough for building the
// handler's JSON response; the test asserts against the DB row, not this.
type noopSigner struct{}

func (noopSigner) Sign(rawPath string) (string, error) { return rawPath, nil }
func (noopSigner) IsValid(rawPath string, _ url.Values) bool { return true }
func (noopSigner) SignIfLocal(path string) string { return path }
func (noopSigner) SignAndGetPublicURL(rawPath string) (string, error) {
	return rawPath, nil
}

func buildProfilePhotoRequest(t *testing.T) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "avatar.jpg")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write([]byte("fake-jpeg-bytes")); err != nil {
		t.Fatalf("failed to write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/profile_photo", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestUpdateProfilePhoto_PersistsRawKeyNotResolvedURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDeletionDB(t) // shared in-memory sqlite + migrations helper
	repo := uRepo.NewUserRepository(db, &config.Config{})

	testUser := &uModel.User{
		ID:          1,
		Username:    "testuser",
		DisplayName: "Test User",
		CircleID:    1,
	}
	if err := db.Create(testUser).Error; err != nil {
		t.Fatalf("failed to seed test user: %v", err)
	}

	fakeStorage := &fakePresignedStorage{}
	h := &Handler{
		userRepo: repo,
		storage:  fakeStorage,
		signer:   noopSigner{},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = buildProfilePhotoRequest(t)
	c.Set("id", &uModel.UserDetails{User: *testUser})

	h.updateProfilePhoto(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	updated, err := repo.GetUserByID(context.Background(), testUser.ID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}

	if strings.HasPrefix(updated.Image, "http://") || strings.HasPrefix(updated.Image, "https://") {
		t.Fatalf("expected a bare storage key to be persisted, got a resolved URL: %s", updated.Image)
	}
	if !strings.HasPrefix(updated.Image, fmt.Sprintf("profiles/%d/", testUser.ID)) {
		t.Fatalf("expected image key under profiles/%d/, got: %s", testUser.ID, updated.Image)
	}
	if !strings.HasSuffix(updated.Image, ".jpg") {
		t.Fatalf("expected .jpg extension preserved, got: %s", updated.Image)
	}
}
