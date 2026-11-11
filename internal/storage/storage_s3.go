package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"donetick.com/core/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3Storage struct {
	Bucket       string
	PublicBucket string
	PublicHost   string
	BasePath     string
	Client       *s3.S3
	Key          string
}

// VALID_FOR is the lifetime of a presigned URL. AWS SigV4 caps presigned
// URL expiration at 7 days, so this is the maximum we can ask for.
const VALID_FOR = 7 * 24 * time.Hour

func NewS3Storage(config *config.Config) (*S3Storage, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:   aws.String(config.Storage.Region),
		Endpoint: aws.String(config.Storage.Endpoint),
		Credentials: credentials.NewStaticCredentials(
			config.Storage.AccessKey,
			config.Storage.SecretKey,
			"",
		),
		S3ForcePathStyle: aws.Bool(config.Storage.PathStyle),
	})
	if err != nil {
		return nil, err
	}
	return &S3Storage{
		Bucket:       config.Storage.BucketName,
		PublicBucket: config.Storage.PublicBucketName,
		PublicHost:   config.Storage.PublicBucketHost,
		BasePath:     config.Storage.BasePath,
		Client:       s3.New(sess),
	}, nil
}

// key maps a storage path (as stored in storage_files rows and in the user
// Image column) to the S3 object key. Every read, write and delete must build
// keys through this, otherwise a delete silently targets a key that no save
// ever wrote and S3 answers 204 as if it had worked.
func (s *S3Storage) key(path string) string {
	return fmt.Sprintf("%s/%s", s.BasePath, path)
}

func (s *S3Storage) Save(ctx context.Context, path string, file io.Reader) error {
	key := s.key(path)

	// Read the file into a buffer to create an io.ReadSeeker
	buf, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(buf)

	_, err = s.Client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
		Body:   reader,
	})
	if err != nil {
		return err
	}

	return nil
}
func (s *S3Storage) Delete(ctx context.Context, paths []string) error {
	return s.deleteFrom(ctx, s.Bucket, paths)
}

// DeletePublic deletes objects written by SavePublic, which land in the public
// bucket when one is configured. Deleting them through Delete would issue the
// request against the private bucket and leave the public object readable.
func (s *S3Storage) DeletePublic(ctx context.Context, paths []string) error {
	return s.deleteFrom(ctx, s.publicBucket(), paths)
}

func (s *S3Storage) deleteFrom(ctx context.Context, bucket string, paths []string) error {
	var err error
	for _, path := range paths {
		_, e := s.Client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(s.key(path)),
		})
		if e != nil {
			err = e
		}
	}
	return err
}

func (s *S3Storage) GetURL(ctx context.Context, path string) (string, error) {
	key := s.key(path)
	req, _ := s.Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	})
	// make this indefinite valid for 1000 years
	urlStr, err := req.Presign(VALID_FOR)
	if err != nil {
		return "", err
	}
	return urlStr, nil
}

func (s *S3Storage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	return nil, errors.New("Get method not implemented for S3Storage, use GetObject with the key")
}

func (s *S3Storage) publicBucket() string {
	if s.PublicBucket != "" {
		return s.PublicBucket
	}
	return s.Bucket
}

func (s *S3Storage) SavePublic(ctx context.Context, path string, file io.Reader) error {
	key := s.key(path)
	buf, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	_, err = s.Client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.publicBucket()),
		Key:    aws.String(key),
		Body:   bytes.NewReader(buf),
	})
	return err
}

func (s *S3Storage) GetPublicURL(ctx context.Context, path string) (string, error) {
	key := s.key(path)
	if s.PublicHost != "" {
		return fmt.Sprintf("https://%s/%s", s.PublicHost, key), nil
	}
	if s.PublicBucket != "" {
		// Public bucket with no CDN host — generate a presigned URL from the public bucket.
		req, _ := s.Client.GetObjectRequest(&s3.GetObjectInput{
			Bucket: aws.String(s.PublicBucket),
			Key:    aws.String(key),
		})
		return req.Presign(VALID_FOR)
	}
	// No public bucket configured — fall back to the private bucket presigned URL.
	return s.GetURL(ctx, path)
}

// DeletePublicByURL deletes the S3 object identified by a URL previously
// returned by GetPublicURL. Only URLs whose host matches PublicHost are
// acted on; all other URLs (external OIDC picture URLs, etc.) are ignored.
// This means it is a no-op when PublicHost is unset, including the
// PublicBucket-without-CDN case where GetPublicURL returns a presigned URL —
// those are not matched on purpose, since a presigned URL carries a query
// signature and is not a stable identifier for the object.
func (s *S3Storage) DeletePublicByURL(ctx context.Context, rawURL string) error {
	if s.PublicHost == "" {
		return nil
	}
	prefix := fmt.Sprintf("https://%s/%s/", s.PublicHost, s.BasePath)
	if !strings.HasPrefix(rawURL, prefix) {
		return nil
	}
	path := strings.TrimPrefix(rawURL, prefix)
	// PublicHost serves the public bucket, so the object must be removed from
	// there — Delete would target the private bucket.
	return s.DeletePublic(ctx, []string{path})
}
