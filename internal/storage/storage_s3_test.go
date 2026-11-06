package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// newRecordingS3 returns an S3Storage backed by a fake path-style S3 endpoint
// that records the object path of every request, keyed by HTTP method.
func newRecordingS3(t *testing.T, basePath string) (*S3Storage, map[string][]string) {
	t.Helper()
	var mu sync.Mutex
	seen := map[string][]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen[r.Method] = append(seen[r.Method], strings.TrimPrefix(r.URL.Path, "/bucket/"))
		mu.Unlock()
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(srv.Close)

	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-east-1"),
		Endpoint:         aws.String(srv.URL),
		Credentials:      credentials.NewStaticCredentials("key", "secret", ""),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	return &S3Storage{Bucket: "bucket", BasePath: basePath, Client: s3.New(sess)}, seen
}

// Delete must remove the same object key that Save wrote, otherwise the
// object is orphaned in the bucket while its storage_files row is deleted.
func TestS3DeleteRemovesTheKeySaveWrote(t *testing.T) {
	for _, basePath := range []string{"assets", ""} {
		t.Run("base_path="+basePath, func(t *testing.T) {
			s, seen := newRecordingS3(t, basePath)
			ctx := context.Background()
			path := "users/1/file.pdf"

			if err := s.Save(ctx, path, strings.NewReader("data")); err != nil {
				t.Fatalf("Save: %v", err)
			}
			if err := s.Delete(ctx, []string{path}); err != nil {
				t.Fatalf("Delete: %v", err)
			}

			if len(seen[http.MethodPut]) != 1 || len(seen[http.MethodDelete]) != 1 {
				t.Fatalf("expected one PUT and one DELETE, got %v", seen)
			}
			if put, del := seen[http.MethodPut][0], seen[http.MethodDelete][0]; put != del {
				t.Fatalf("Delete removed %q, but Save wrote %q", del, put)
			}
		})
	}
}
