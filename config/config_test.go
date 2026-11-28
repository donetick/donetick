package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mcuadros/go-defaults"
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
	if cfg.MCP.Enabled {
		t.Error("expected the MCP server to be off by default")
	}
}

// TestServerInsecureCookies checks that server.insecure_cookies defaults to
// false, can be turned on from YAML or from DT_SERVER_INSECURE_COOKIES, and is
// not flipped back by defaults.SetDefaults. The viper setup mirrors LoadConfig.
func TestServerInsecureCookies(t *testing.T) {
	load := func(t *testing.T, yaml string) Config {
		t.Helper()
		v := viper.NewWithOptions(viper.ExperimentalBindStruct())
		v.SetEnvPrefix("DT")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.SetConfigType("yaml")
		if err := v.ReadConfig(strings.NewReader(yaml)); err != nil {
			t.Fatalf("read config: %v", err)
		}
		v.AutomaticEnv()
		var cfg Config
		if err := v.Unmarshal(&cfg); err != nil {
			t.Fatalf("unmarshal config: %v", err)
		}
		defaults.SetDefaults(&cfg)
		return cfg
	}

	if load(t, selfhostedConfigTemplate).Server.InsecureCookies {
		t.Error("expected insecure_cookies to default to false")
	}
	if !load(t, "server:\n  insecure_cookies: true\n").Server.InsecureCookies {
		t.Error("expected server.insecure_cookies: true to be honored")
	}

	t.Setenv("DT_SERVER_INSECURE_COOKIES", "true")
	if !load(t, selfhostedConfigTemplate).Server.InsecureCookies {
		t.Error("expected DT_SERVER_INSECURE_COOKIES=true to be honored")
	}
}
