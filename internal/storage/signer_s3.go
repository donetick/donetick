package storage

import (
	"fmt"
	"net/url"
	"strings"

	"donetick.com/core/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

// URLSignerS3 returns fetchable URLs for objects in the S3 bucket.
//
// In the default (signed) mode it returns SigV4 presigned URLs with a
// VALID_FOR lifetime. When the storage is configured with
// `public_read: true` the signer returns unsigned URLs instead — the
// caller is responsible for making the bucket anonymously readable.

type URLSignerS3 struct {
	storage    *S3Storage
	PublicHost string
	PublicRead bool
}

func NewURLSignerS3(storage *S3Storage, config *config.Config) *URLSignerS3 {
	return &URLSignerS3{
		storage:    storage,
		PublicHost: config.Storage.PublicHost,
		PublicRead: config.Storage.PublicRead,
	}
}

func (s *URLSignerS3) Sign(rawPath string) (string, error) {
	key := fmt.Sprintf("%s/%s", s.storage.BasePath, rawPath)
	req, _ := s.storage.Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(s.storage.Bucket),
		Key:    aws.String(key),
	})

	var urlStr string
	if s.PublicRead {
		// Build the request URL without running the signing handlers so the
		// result has no X-Amz-* query params. The bucket is expected to be
		// configured for anonymous GETs.
		req.Build()
		if req.Error != nil {
			return "", req.Error
		}
		urlStr = req.HTTPRequest.URL.String()
	} else {
		presigned, err := req.Presign(VALID_FOR)
		if err != nil {
			return "", err
		}
		urlStr = presigned
	}

	if s.PublicHost != "" {
		parsedURL, err := url.Parse(urlStr)
		if err != nil {
			return "", err
		}
		parsedURL.Host = s.PublicHost
		urlStr = parsedURL.String()
	}

	return urlStr, nil
}

func (s *URLSignerS3) IsValid(rawPath string, providedSig string) bool {

	return true
}

// SignIfLocal returns a fetchable URL for a locally-stored path (by calling
// Sign) and passes through external URLs (e.g. from an OIDC `picture` claim
// or legacy rows that stored a full URL) unchanged. Use this wherever a
// user.Image field is being serialized back to a client so handlers that
// persist raw storage paths produce URLs browsers can actually fetch.
func (s *URLSignerS3) SignIfLocal(path string) string {
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	signed, err := s.Sign(path)
	if err != nil {
		return ""
	}
	return signed
}
