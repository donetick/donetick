package user

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"donetick.com/core/config"
	storage "donetick.com/core/internal/storage"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newProfilePhotoHandler(t *testing.T) (*Handler, *gorm.DB, *storage.LocalStorage) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&uModel.User{}))

	cfg := &config.Config{}
	cfg.Database.Type = "sqlite"
	cfg.Storage.BasePath = t.TempDir()
	cfg.Jwt.Secret = "secret"
	ls := storage.NewLocalStorage(cfg)
	signer := storage.NewURLSignerLocal(cfg)

	return &Handler{userRepo: uRepo.NewUserRepository(db, cfg), storage: ls, signer: signer}, db, ls
}

func postProfilePhoto(t *testing.T, h *Handler, currentUser *uModel.UserDetails) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "new.jpg")
	require.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader([]byte("new photo bytes")))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	r := gin.New()
	r.POST("/api/v1/users/profile_photo", func(c *gin.Context) {
		c.Set("id", currentUser)
		h.updateProfilePhoto(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/profile_photo", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// Reproduces the reported "F5" finding: a user sets their own Image field to
// a traversal path ("x/profiles/../users/<victim>/<uuid>.ext") pointing at
// another user's private attachment, then uploads a new profile photo. The
// handler's cleanup of the previous image must not delete anything outside
// the profiles/ tree.
func TestUpdateProfilePhotoDoesNotDeleteOutsideProfilesTree(t *testing.T) {
	h, db, ls := newProfilePhotoHandler(t)

	victimKey := "users/1/secret.txt"
	victimFull := filepath.Join(ls.BasePath, victimKey)
	require.NoError(t, os.MkdirAll(filepath.Dir(victimFull), 0o755))
	require.NoError(t, os.WriteFile(victimFull, []byte("victim secret"), 0o644))

	attacker := uModel.User{ID: 2, Image: "x/profiles/../" + victimKey}
	require.NoError(t, db.Create(&attacker).Error)

	rec := postProfilePhoto(t, h, &uModel.UserDetails{User: attacker})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected profile photo upload to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	if _, err := os.Stat(victimFull); err != nil {
		t.Fatalf("victim's file must survive the traversal delete, but stat failed: %v", err)
	}
}

// The traversal fix alone still left "profiles/<other id>/..." reachable,
// which deletes another user's profile photo. Cleanup must be scoped to the
// caller's own profiles/<id>/ prefix.
func TestUpdateProfilePhotoDoesNotDeleteAnotherUsersProfilePhoto(t *testing.T) {
	h, db, ls := newProfilePhotoHandler(t)

	victimKey := "profiles/1/victim.jpg"
	victimFull := filepath.Join(ls.BasePath, victimKey)
	require.NoError(t, os.MkdirAll(filepath.Dir(victimFull), 0o755))
	require.NoError(t, os.WriteFile(victimFull, []byte("victim photo"), 0o644))

	attacker := uModel.User{ID: 2, Image: victimKey}
	require.NoError(t, db.Create(&attacker).Error)

	rec := postProfilePhoto(t, h, &uModel.UserDetails{User: attacker})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	if _, err := os.Stat(victimFull); err != nil {
		t.Fatalf("another user's profile photo must survive, but stat failed: %v", err)
	}
}

// The caller's own previous photo is still cleaned up — the scoping fix must
// not turn the cleanup into a no-op.
func TestUpdateProfilePhotoDeletesOwnPreviousPhoto(t *testing.T) {
	h, db, ls := newProfilePhotoHandler(t)

	ownKey := "profiles/2/old.jpg"
	ownFull := filepath.Join(ls.BasePath, ownKey)
	require.NoError(t, os.MkdirAll(filepath.Dir(ownFull), 0o755))
	require.NoError(t, os.WriteFile(ownFull, []byte("old photo"), 0o644))

	u := uModel.User{ID: 2, Image: ownKey}
	require.NoError(t, db.Create(&u).Error)

	rec := postProfilePhoto(t, h, &uModel.UserDetails{User: u})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	if _, err := os.Stat(ownFull); !os.IsNotExist(err) {
		t.Fatalf("own previous photo should have been deleted, stat err: %v", err)
	}
}

// Exercise the real S3 URL builder and handler together: URLs persisted by an
// upload must route cleanup to the same bucket, without trusting arbitrary URLs.
func TestUpdateProfilePhotoCleansUpManagedS3URLs(t *testing.T) {
	for _, backend := range []struct {
		name, publicHost, publicBucket, wantBucket string
	}{
		{"CDN", "cdn.example.com", "public-bucket", "public-bucket"},
		{"presigned public", "", "public-bucket", "public-bucket"},
		{"presigned private fallback", "", "", "private-bucket"},
	} {
		t.Run(backend.name, func(t *testing.T) {
			for _, tc := range []struct {
				name, key  string
				change     func(string) string
				wantDelete bool
			}{
				{name: "own photo", key: "profiles/2/old.jpg", wantDelete: true},
				{name: "expired signature", key: "profiles/2/old.jpg", change: func(u string) string {
					return strings.SplitN(u, "?", 2)[0] + "?X-Amz-Date=20000101T000000Z&X-Amz-Signature=expired"
				}, wantDelete: true},
				{name: "other user", key: "profiles/1/old.jpg"},
				{name: "private attachment", key: "users/2/secret.jpg"},
				{name: "traversal", key: "profiles/2/../1/old.jpg"},
				{name: "encoded traversal", key: "profiles/2/old.jpg", change: func(u string) string {
					return strings.Replace(u, "/old.jpg", "/%2e%2e/1/old.jpg", 1)
				}},
				{name: "external origin", key: "profiles/2/old.jpg", change: func(u string) string {
					return "https://external.example.com/" + strings.SplitN(u, "://", 2)[1]
				}},
				{name: "wrong base path", key: "profiles/2/old.jpg", change: func(u string) string {
					return strings.Replace(u, "/assets/", "/elsewhere/", 1)
				}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					h, db, _ := newProfilePhotoHandler(t)
					var mu sync.Mutex
					var deleted []string
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method == http.MethodDelete {
							mu.Lock()
							deleted = append(deleted, r.URL.Path)
							mu.Unlock()
							w.WriteHeader(http.StatusNoContent)
						}
					}))
					t.Cleanup(srv.Close)
					sess, err := session.NewSession(&aws.Config{
						Region: aws.String("us-east-1"), Endpoint: aws.String(srv.URL),
						Credentials:      credentials.NewStaticCredentials("key", "secret", ""),
						S3ForcePathStyle: aws.Bool(true),
					})
					require.NoError(t, err)
					h.storage = &storage.S3Storage{
						Bucket: "private-bucket", PublicBucket: backend.publicBucket,
						PublicHost: backend.publicHost, BasePath: "assets", Client: s3.New(sess),
					}
					image, err := h.storage.GetPublicURL(t.Context(), tc.key)
					require.NoError(t, err)
					if tc.change != nil {
						image = tc.change(image)
					}
					u := uModel.User{ID: 2, Image: image}
					require.NoError(t, db.Create(&u).Error)
					rec := postProfilePhoto(t, h, &uModel.UserDetails{User: u})
					require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
					mu.Lock()
					defer mu.Unlock()
					if tc.wantDelete {
						require.Equal(t, []string{"/" + backend.wantBucket + "/assets/profiles/2/old.jpg"}, deleted)
					} else {
						require.Empty(t, deleted)
					}
				})
			}
		})
	}
}

func TestValidateUserImage(t *testing.T) {
	const uid = 2

	for _, tc := range []struct {
		name  string
		in    string
		want  string
		isErr bool
	}{
		{name: "empty clears the field", in: "", want: ""},
		{name: "https url", in: "https://cdn.example.com/a.jpg", want: "https://cdn.example.com/a.jpg"},
		{name: "own key", in: "profiles/2/a.jpg", want: "profiles/2/a.jpg"},
		{name: "own signed key round-trips", in: "profiles/2/a.jpg?expires=1&sig=deadbeef", want: "profiles/2/a.jpg"},
		{name: "legacy base-path prefixed own key", in: "uploads/profiles/2/a.jpg", want: "profiles/2/a.jpg"},
		{name: "traversal to private upload", in: "x/profiles/../users/1/secret.txt", isErr: true},
		{name: "another user's profile key", in: "profiles/1/a.jpg", isErr: true},
		{name: "private upload key", in: "users/1/secret.txt", isErr: true},
		{name: "escaping the base path", in: "profiles/2/../../etc/passwd", isErr: true},
		{name: "non-http scheme", in: "file:///etc/passwd", isErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateUserImage(tc.in, uid)
			if tc.isErr {
				require.Error(t, err, "expected %q to be rejected, got %q", tc.in, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
