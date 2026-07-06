package storage

import "net/url"

type URLSigner interface {
	Sign(rawPath string) (string, error)
	// IsValid checks whether the signature for rawPath is valid.
	// query contains the full set of URL query parameters (e.g. "sig", "expires").
	IsValid(rawPath string, query url.Values) bool
	// SignIfLocal signs a locally-stored path and passes through external URLs unchanged.
	SignIfLocal(path string) string

	SignAndGetPublicURL(rawPath string) (string, error)
}
