package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestGenerateDefaultConfigFile(t *testing.T) {
	dir := t.TempDir()
	if err := generateDefaultConfigFile(dir); err != nil {
		t.Fatalf("generateDefaultConfigFile: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "selfhosted.yaml"))
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	if strings.Contains(string(content), placeholderJWTSecret) {
		t.Fatal("generated config still contains the placeholder JWT secret")
	}

	v := viper.New()
	v.SetConfigFile(filepath.Join(dir, "selfhosted.yaml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("parse generated config: %v", err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("unmarshal generated config: %v", err)
	}
	if err := validateJWTSecret(cfg.Jwt.Secret); err != nil {
		t.Fatalf("generated JWT secret is invalid: %v", err)
	}
	if !cfg.Server.ServeFrontend {
		t.Error("expected serve_frontend to be true")
	}
}

func TestSanitizeBasePath(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "", true},
		{"/", "", true},
		{"/donetick", "/donetick", true},
		{"/donetick/", "/donetick", true},
		{"/api/hassio_ingress/Ab3_x-Yz9", "/api/hassio_ingress/Ab3_x-Yz9", true},
		{"/a/b.c/d~e", "/a/b.c/d~e", true},
		{" ", "", false},                   // whitespace only
		{" /donetick", "", false},          // leading space
		{"/donetick ", "", false},          // trailing space
		{"/x y", "", false},                // space inside
		{"donetick", "", false},            // must be absolute
		{"//evil.example.com", "", false},  // protocol-relative URL
		{"/a//b", "", false},               // empty segment
		{"/a/../b", "", false},             // traversal
		{"/a/./b", "", false},              // dot segment
		{"/..", "", false},                 // traversal
		{`/x"><script>`, "", false},        // HTML injection
		{"/x</script><script>", "", false}, // script break-out
		{"/x?y=1", "", false},              // query
		{"/x#y", "", false},                // fragment
		{"https://evil.example.com", "", false},
	}
	for _, c := range cases {
		got, ok := SanitizeBasePath(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("SanitizeBasePath(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestValidHeaderName(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"", true},                   // not configured
		{"X-Ingress-Path", true},     // Home Assistant
		{"x-forwarded-prefix", true}, // header names are case-insensitive
		{"Prefix", true},             // no "X-" required
		{" ", false},                 // whitespace only
		{" X-Ingress-Path", false},   // leading space
		{"X-Ingress-Path ", false},   // trailing space
		{"X Ingress", false},         // space inside
		{"X-Path:", false},           // pasted with a colon
		{"X-Path\r\nEvil", false},    // header injection via Vary
		{"X--Path", false},           // double hyphen
		{"X-Path-", false},           // trailing hyphen
		{"-X-Path", false},           // leading hyphen
		{"1-Path", false},            // must start with a letter
		{"X_Path", false},            // underscore (nginx drops these by default)
		{"X.Path", false},            // dot
		{"X-Päth", false},            // non-ASCII
		{"X-" + strings.Repeat("A", 62), true},
		{"X-" + strings.Repeat("A", 63), false}, // over 64 chars
	}
	for _, c := range cases {
		if ok := ValidHeaderName(c.in); ok != c.ok {
			t.Errorf("ValidHeaderName(%q) = %v, want %v", c.in, ok, c.ok)
		}
	}
}

func TestValidateBasePathConfig(t *testing.T) {
	s := ServerConfig{BasePath: "/donetick/", BasePathHeader: "X-Ingress-Path"}
	if err := validateBasePathConfig(&s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.BasePath != "/donetick" {
		t.Errorf("BasePath not normalised: %q", s.BasePath)
	}

	if err := validateBasePathConfig(&ServerConfig{BasePath: "/done tick"}); err == nil {
		t.Error("expected error for invalid base_path")
	}
	if err := validateBasePathConfig(&ServerConfig{BasePathHeader: "X Ingress"}); err == nil {
		t.Error("expected error for invalid base_path_header")
	}
}
