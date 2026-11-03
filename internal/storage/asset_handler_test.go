package storage

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"donetick.com/core/config"
	"github.com/gin-gonic/gin"
)

func newTestAssetHandler(t *testing.T) (*Handler, *LocalStorage) {
	t.Helper()
	dir := t.TempDir()
	ls := &LocalStorage{BasePath: dir}
	signer := newTestSigner()
	h := NewHandler(ls, nil, nil, nil, signer, &config.Config{})
	return h, ls
}

func serveAsset(h *Handler, rawPath string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets/"+rawPath, nil)
	c.Request = req
	// Mirror what gin's router does for a */filepath wildcard param: it keeps
	// the raw, uncleaned remainder of the path (including "..") as-is.
	c.Params = gin.Params{{Key: "filepath", Value: "/" + rawPath}}
	h.AssetHandler(c)
	return w
}

// Reproduces the reported bypass: a private key under users/<id>/ is served
// unsigned and without auth by prefixing it with "profiles/../", which looks
// public to the uncleaned prefix check but resolves to the private file once
// storage.Get cleans it.
func TestAssetHandlerRejectsTraversalToPrivateFile(t *testing.T) {
	h, ls := newTestAssetHandler(t)

	privateKey := "users/1/secret.txt"
	full := filepath.Join(ls.BasePath, privateKey)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("secret contents"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Control: a plain unsigned request for the private key is rejected.
	w := serveAsset(h, privateKey)
	if w.Code != http.StatusForbidden {
		t.Fatalf("plain unsigned request: expected 403, got %d", w.Code)
	}

	// Attack: traversal through the public "profiles/" prefix must still be rejected.
	w = serveAsset(h, "profiles/../"+privateKey)
	if w.Code == http.StatusOK {
		t.Fatalf("traversal via profiles/../ bypassed the signature check and returned the file: %q", w.Body.String())
	}

	// Negative control: the same traversal form for a nonexistent key must 404,
	// not 403, to prove the harness can tell the difference. Our fix should
	// reject it before the storage lookup, so 400 (invalid path) is expected here.
	w = serveAsset(h, "profiles/../users/1/nope.txt")
	if w.Code == http.StatusOK {
		t.Fatalf("traversal to nonexistent key unexpectedly returned 200")
	}
}

func TestAssetHandlerServesPublicProfileAsset(t *testing.T) {
	h, ls := newTestAssetHandler(t)

	publicKey := "profiles/2/photo.jpg"
	full := filepath.Join(ls.BasePath, publicKey)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("jpg bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := serveAsset(h, publicKey)
	if w.Code != http.StatusOK {
		t.Fatalf("legit public asset request: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "jpg bytes") {
		t.Fatalf("unexpected body: %q", w.Body.String())
	}
}
