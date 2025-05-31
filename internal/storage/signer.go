package storage

type URLSigner interface {
	Sign(rawPath string) (string, error)
	IsValid(rawPath string, providedSig string) bool
}
