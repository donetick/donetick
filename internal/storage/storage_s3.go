package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"donetick.com/core/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3Storage struct {
	Bucket   string
	BasePath string
	Client   *s3.S3
	Key      string
}

const (
	VALID_FOR = 1000 * 365 * 24 * 60 * 60 // 1000 years
)

func NewS3Storage(config *config.Config) (*S3Storage, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:   aws.String(config.Storage.AWS.Region),
		Endpoint: aws.String(config.Storage.AWS.Endpoint),
		Credentials: credentials.NewStaticCredentials(
			config.Storage.AWS.AccessKey,
			config.Storage.AWS.SecretKey,
			"",
		),
	})
	if err != nil {
		return nil, err
	}
	return &S3Storage{
		Bucket:   config.Storage.AWS.BucketName,
		BasePath: config.Storage.AWS.BasePath,
		Client:   s3.New(sess),
	}, nil
}
func (s *S3Storage) Save(ctx context.Context, path string, file io.Reader) error {
	key := fmt.Sprintf("%s/%s", s.BasePath, path)

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
	var err error
	for _, path := range paths {
		key := s.BasePath + path
		_, e := s.Client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.Bucket),
			Key:    aws.String(key),
		})
		if e != nil {
			err = e
		}
	}
	return err
}

func (s *S3Storage) GetURL(ctx context.Context, path string) (string, error) {
	key := s.BasePath + path
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
