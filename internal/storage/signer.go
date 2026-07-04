package storage

type URLSigner interface {
	Sign(rawPath string) (string, error)
	IsValid(rawPath string, providedSig string) bool
	// SignIfLocal signs a locally-stored path and passes through external URLs unchanged.
	SignIfLocal(path string) string
}
