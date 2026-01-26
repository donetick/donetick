package storage

import (
	"fmt"
	"net/url"

	"donetick.com/core/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

// get presigned URL for the object

type URLSignerS3 struct {
	storage    *S3Storage
	PublicHost string
}

func NewURLSignerS3(storage *S3Storage, config *config.Config) *URLSignerS3 {
	return &URLSignerS3{storage: storage,
		PublicHost: config.Storage.AWS.PublicHost,
	}
}

// sign method without expiration:
func (s *URLSignerS3) Sign(rawPath string) (string, error) {
	key := fmt.Sprintf("%s/%s", s.storage.BasePath, rawPath)
	req, _ := s.storage.Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(s.storage.Bucket),
		Key:    aws.String(key),
	})
	urlStr, err := req.Presign(VALID_FOR)
	if err != nil {
		return "", err
	}
	if s.PublicHost != "" {
		parsedURL, err := url.Parse(urlStr)
		if err != nil {
			return "", err
		}
		parsedURL.Host = s.PublicHost
		urlStr = parsedURL.String()
		if err != nil {
			return "", err
		}
	}

	return urlStr, nil
}

func (s *URLSignerS3) IsValid(rawPath string, providedSig string) bool {

	return true
}
