package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig_FromCommandLine(t *testing.T) {
	// Backup os.Args and DT_ENV
	origArgs := os.Args
	origEnv := os.Getenv("DT_ENV")
	origCommandLine := pflag.CommandLine

	// Get current working directory to restore later
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	// Change working directory to the repository root so relative paths like "./config" work
	if err := os.Chdir(".."); err != nil {
		t.Fatalf("failed to change wd: %v", err)
	}

	defer func() {
		os.Args = origArgs
		os.Setenv("DT_ENV", origEnv)
		pflag.CommandLine = origCommandLine
		_ = os.Chdir(currentDir)
		viper.Reset()
	}()

	// Create a temp directory for config files
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a temp config file
	testConfigContent := `
name: "test-custom-config"
jwt:
  secret: "a_very_secure_secret_that_is_at_least_32_chars_long_and_not_weak_12345!"
`
	tempFile := filepath.Join(tempDir, "custom.yaml")
	if err := os.WriteFile(tempFile, []byte(testConfigContent), 0644); err != nil {
		t.Fatalf("failed to write temp config file: %v", err)
	}

	// 1. Test --config flag
	os.Args = []string{"cmd", "--config", tempFile}
	pflag.CommandLine = pflag.NewFlagSet("cmd", pflag.ContinueOnError)
	viper.Reset()
	cfgFlag := LoadConfig()
	assert.Equal(t, "test-custom-config", cfgFlag.Name)

	// 2. Test -c flag
	os.Args = []string{"cmd", "-c", tempFile}
	pflag.CommandLine = pflag.NewFlagSet("cmd", pflag.ContinueOnError)
	viper.Reset()
	cfgShorthand := LoadConfig()
	assert.Equal(t, "test-custom-config", cfgShorthand.Name)

	// 3. Test fallback when file does not exist or flag is empty
	os.Args = []string{"cmd", "--config", "non_existent_file.yaml"}
	os.Setenv("DT_ENV", "local")
	pflag.CommandLine = pflag.NewFlagSet("cmd", pflag.ContinueOnError)
	viper.Reset()
	cfgFallback := LoadConfig()
	assert.NotEmpty(t, cfgFallback.Name)
}
