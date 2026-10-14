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
