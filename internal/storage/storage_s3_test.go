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

// newRecordingS3Buckets is newRecordingS3 with a separate public bucket. It
// records the full request path ("/<bucket>/<key>") so assertions can tell
// which bucket a request was addressed to.
func newRecordingS3Buckets(t *testing.T, basePath, publicHost string) (*S3Storage, map[string][]string) {
	t.Helper()
	var mu sync.Mutex
	seen := map[string][]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen[r.Method] = append(seen[r.Method], r.URL.Path)
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
	return &S3Storage{
		Bucket:       "private-bucket",
		PublicBucket: "public-bucket",
		PublicHost:   publicHost,
		BasePath:     basePath,
		Client:       s3.New(sess),
	}, seen
}

// SavePublic writes to the public bucket, so DeletePublic must delete from the
// public bucket. Routing it to Delete leaves the object publicly readable
// forever while the DB row that pointed at it is gone.
func TestS3DeletePublicTargetsTheBucketSavePublicWrote(t *testing.T) {
	s, seen := newRecordingS3Buckets(t, "assets", "")
	ctx := context.Background()
	path := "profiles/1/photo.jpg"

	if err := s.SavePublic(ctx, path, strings.NewReader("data")); err != nil {
		t.Fatalf("SavePublic: %v", err)
	}
	if err := s.DeletePublic(ctx, []string{path}); err != nil {
		t.Fatalf("DeletePublic: %v", err)
	}

	if len(seen[http.MethodPut]) != 1 || len(seen[http.MethodDelete]) != 1 {
		t.Fatalf("expected one PUT and one DELETE, got %v", seen)
	}
	if put, del := seen[http.MethodPut][0], seen[http.MethodDelete][0]; put != del {
		t.Fatalf("DeletePublic removed %q, but SavePublic wrote %q", del, put)
	}
	if want := "/public-bucket/assets/" + path; seen[http.MethodDelete][0] != want {
		t.Fatalf("DeletePublic targeted %q, want %q", seen[http.MethodDelete][0], want)
	}
}

// Delete must stay on the private bucket even when a public bucket exists.
func TestS3DeleteTargetsThePrivateBucket(t *testing.T) {
	s, seen := newRecordingS3Buckets(t, "assets", "")
	path := "users/1/file.pdf"

	if err := s.Delete(context.Background(), []string{path}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if want := "/private-bucket/assets/" + path; len(seen[http.MethodDelete]) != 1 || seen[http.MethodDelete][0] != want {
		t.Fatalf("Delete targeted %v, want [%q]", seen[http.MethodDelete], want)
	}
}

// A URL handed out by GetPublicURL must round-trip back to the object it names.
func TestS3DeletePublicByURLTargetsThePublicBucket(t *testing.T) {
	s, seen := newRecordingS3Buckets(t, "assets", "pub.example.com")
	ctx := context.Background()
	path := "profiles/1/photo.jpg"

	url, err := s.GetPublicURL(ctx, path)
	if err != nil {
		t.Fatalf("GetPublicURL: %v", err)
	}
	if err := s.DeletePublicByURL(ctx, url); err != nil {
		t.Fatalf("DeletePublicByURL: %v", err)
	}

	if want := "/public-bucket/assets/" + path; len(seen[http.MethodDelete]) != 1 || seen[http.MethodDelete][0] != want {
		t.Fatalf("DeletePublicByURL targeted %v, want [%q]", seen[http.MethodDelete], want)
	}

	// External URLs (e.g. an OIDC picture claim) must be left alone.
	if err := s.DeletePublicByURL(ctx, "https://lh3.googleusercontent.com/a/photo.jpg"); err != nil {
		t.Fatalf("DeletePublicByURL(external): %v", err)
	}
	if len(seen[http.MethodDelete]) != 1 {
		t.Fatalf("external URL triggered a delete: %v", seen[http.MethodDelete])
	}
}

// With no public bucket configured, public objects live in the private bucket
// and DeletePublic must still find them there.
func TestS3DeletePublicFallsBackToPrivateBucket(t *testing.T) {
	s, seen := newRecordingS3(t, "assets")
	ctx := context.Background()
	path := "profiles/1/photo.jpg"

	if err := s.SavePublic(ctx, path, strings.NewReader("data")); err != nil {
		t.Fatalf("SavePublic: %v", err)
	}
	if err := s.DeletePublic(ctx, []string{path}); err != nil {
		t.Fatalf("DeletePublic: %v", err)
	}

	if len(seen[http.MethodPut]) != 1 || len(seen[http.MethodDelete]) != 1 {
		t.Fatalf("expected one PUT and one DELETE, got %v", seen)
	}
	if put, del := seen[http.MethodPut][0], seen[http.MethodDelete][0]; put != del {
		t.Fatalf("DeletePublic removed %q, but SavePublic wrote %q", del, put)
	}
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
