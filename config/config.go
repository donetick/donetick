package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/mcuadros/go-defaults"
	"github.com/spf13/viper"
)

var (
	Version   = "dev"
	Commit    = "dev"
	BuildDate = "dev"
)

type Config struct {
	Name                   string              `mapstructure:"name" yaml:"name"`
	Telegram               TelegramConfig      `mapstructure:"telegram" yaml:"telegram"`
	Pushover               PushoverConfig      `mapstructure:"pushover" yaml:"pushover"`
	Database               DatabaseConfig      `mapstructure:"database" yaml:"database"`
	Jwt                    JwtConfig           `mapstructure:"jwt" yaml:"jwt"`
	Server                 ServerConfig        `mapstructure:"server" yaml:"server"`
	SchedulerJobs          SchedulerConfig     `mapstructure:"scheduler_jobs" yaml:"scheduler_jobs"`
	EmailConfig            EmailConfig         `mapstructure:"email" yaml:"email"`
	StripeConfig           StripeConfig        `mapstructure:"stripe" yaml:"stripe"`
	RevenueCatConfig       RevenueCatConfig    `mapstructure:"revenuecat" yaml:"revenuecat"`
	IAPConfig              IAPConfig           `mapstructure:"iap" yaml:"iap"`
	OAuth2Config           OAuth2Config        `mapstructure:"oauth2" yaml:"oauth2"`
	WebhookConfig          WebhookConfig       `mapstructure:"webhook" yaml:"webhook"`
	RealTimeConfig         RealTimeConfig      `mapstructure:"realtime" yaml:"realtime"`
	MFAConfig              MFAConfig           `mapstructure:"mfa" yaml:"mfa"`
	Logging                LogConfig           `mapstructure:"logging" yaml:"logging"`
	IsDoneTickDotCom       bool                `mapstructure:"is_done_tick_dot_com" yaml:"is_done_tick_dot_com"`
	IsUserCreationDisabled bool                `mapstructure:"is_user_creation_disabled" yaml:"is_user_creation_disabled"`
	MinVersion             string              `mapstructure:"min_version" yaml:"min_version"`
	DonetickCloudConfig    DonetickCloudConfig `mapstructure:"donetick_cloud" yaml:"donetick_cloud"`
	FCM                    FCMConfig           `mapstructure:"fcm" yaml:"fcm"`
	FeatureLimits          FeatureLimitsConfig `mapstructure:"feature_limits" yaml:"feature_limits"`
	Storage                StorageConfig       `mapstructure:"storage" yaml:"storage"`
	Info                   Info
}

type Info struct {
	Version   string
	Commit    string
	BuildDate string
}
type StorageConfig struct {
	Mode           string      `mapstructure:"mode" yaml:"mode"`
	PublicHost     string      `mapstructure:"public_host" yaml:"public_host"`
	MaxUserStorage int         `mapstructure:"max_user_storage" yaml:"max_user_storage"`
	MaxFileSize    int64       `mapstructure:"max_file_size" yaml:"max_file_size"`
	AWS            *AWSStorage `mapstructure:"aws" yaml:"aws"`
}

type AWSStorage struct {
	StorageType string `mapstructure:"storage_type" yaml:"storage_type"`
	// CloudStorage:
	BucketName string `mapstructure:"bucket_name" yaml:"bucket_name"`
	Region     string `mapstructure:"region" yaml:"region"`
	BasePath   string `mapstructure:"base_path" yaml:"base_path"`
	AccessKey  string `mapstructure:"access_key" yaml:"access_key"`
	SecretKey  string `mapstructure:"secret_key" yaml:"secret_key"`
	Endpoint   string `mapstructure:"endpoint" yaml:"endpoint"`
}

type DonetickCloudConfig struct {
	GoogleClientID        string `mapstructure:"google_client_id" yaml:"google_client_id"`
	GoogleAndroidClientID string `mapstructure:"google_android_client_id" yaml:"google_android_client_id"`
	GoogleIOSClientID     string `mapstructure:"google_ios_client_id" yaml:"google_ios_client_id"`
	AppleClientID         string `mapstructure:"apple_client_id" yaml:"apple_client_id"`
}

type FeatureLimitsConfig struct {
	MaxCircleMembers     int `mapstructure:"max_circle_members" yaml:"max_circle_members" default:"2"`
	PlusCircleMaxMembers int `mapstructure:"plus_circle_max_members" yaml:"plus_circle_max_members" default:"6"`
	MaxSubaccounts       int `mapstructure:"max_subaccounts" yaml:"max_subaccounts" default:"1"`
	PlusMaxSubaccounts   int `mapstructure:"plus_max_subaccounts" yaml:"plus_max_subaccounts" default:"5"`
}

type TelegramConfig struct {
	Token string `mapstructure:"token" yaml:"token"`
}

type PushoverConfig struct {
	Token string `mapstructure:"token" yaml:"token"`
}

type DatabaseConfig struct {
	Type      string `mapstructure:"type" yaml:"type"`
	Host      string `mapstructure:"host" yaml:"host"`
	Port      int    `mapstructure:"port" yaml:"port"`
	User      string `mapstructure:"user" yaml:"user"`
	Password  string `mapstructure:"password" yaml:"password"`
	Name      string `mapstructure:"name" yaml:"name"`
	Migration bool   `mapstructure:"migration" yaml:"migration" default:"true"`
	LogLevel  int    `mapstructure:"logger" yaml:"logger"`
}

type JwtConfig struct {
	Secret      string        `mapstructure:"secret" yaml:"secret"`
	SessionTime time.Duration `mapstructure:"session_time" yaml:"session_time"`
	MaxRefresh  time.Duration `mapstructure:"max_refresh" yaml:"max_refresh"`
}

type ServerConfig struct {
	Port             int           `mapstructure:"port" yaml:"port"`
	RatePeriod       time.Duration `mapstructure:"rate_period" yaml:"rate_period"`
	RateLimit        int           `mapstructure:"rate_limit" yaml:"rate_limit"`
	ReadTimeout      time.Duration `mapstructure:"read_timeout" yaml:"read_timeout"`
	WriteTimeout     time.Duration `mapstructure:"write_timeout" yaml:"write_timeout"`
	WebhookTimeout   time.Duration `mapstructure:"webhook_timeout" yaml:"webhook_timeout"`
	CorsAllowOrigins []string      `mapstructure:"cors_allow_origins" yaml:"cors_allow_origins"`
	ServeFrontend    bool          `mapstructure:"serve_frontend" yaml:"serve_frontend"`
}

type SchedulerConfig struct {
	DueJob     time.Duration `mapstructure:"due_job" yaml:"due_job"`
	OverdueJob time.Duration `mapstructure:"overdue_job" yaml:"overdue_job"`
	PreDueJob  time.Duration `mapstructure:"pre_due_job" yaml:"pre_due_job"`
}

type StripeConfig struct {
	APIKey         string         `mapstructure:"api_key"`
	WhitelistedIPs []string       `mapstructure:"whitelisted_ips"`
	Prices         []StripePrices `mapstructure:"prices"`
	SuccessURL     string         `mapstructure:"success_url"`
	CancelURL      string         `mapstructure:"cancel_url"`
}

type StripePrices struct {
	PriceID string `mapstructure:"id"`
	Name    string `mapstructure:"name"`
}

type RevenueCatConfig struct {
	AuthSecret       string   `mapstructure:"auth_secret" yaml:"auth_secret"`
	WhitelistedIPs   []string `mapstructure:"whitelisted_ips" yaml:"whitelisted_ips"`
	SharedSecret     string   `mapstructure:"shared_secret" yaml:"shared_secret"`
	EnableValidation bool     `mapstructure:"enable_validation" yaml:"enable_validation"`
}

type IAPConfig struct {
	Apple  AppleIAPConfig  `mapstructure:"apple" yaml:"apple"`
	Google GoogleIAPConfig `mapstructure:"google" yaml:"google"`
}

type AppleIAPConfig struct {
	BundleID     string `mapstructure:"bundle_id" yaml:"bundle_id"`
	SharedSecret string `mapstructure:"shared_secret" yaml:"shared_secret"`
	Sandbox      bool   `mapstructure:"sandbox" yaml:"sandbox"`
	KeyID        string `mapstructure:"key_id" yaml:"key_id"`
	IssuerID     string `mapstructure:"issuer_id" yaml:"issuer_id"`
	PrivateKey   string `mapstructure:"private_key" yaml:"private_key"`
}

type GoogleIAPConfig struct {
	PackageName        string `mapstructure:"package_name" yaml:"package_name"`
	ServiceAccountJSON string `mapstructure:"service_account_json" yaml:"service_account_json"`
}

type FCMConfig struct {
	CredentialsPath string `json:"credentials_path" mapstructure:"credentials_path"`
	ProjectID       string `json:"project_id" mapstructure:"project_id"`
}
type EmailConfig struct {
	Email   string `mapstructure:"email"`
	Key     string `mapstructure:"key"`
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	AppHost string `mapstructure:"appHost"`
}

type OAuth2Config struct {
	ClientID     string   `mapstructure:"client_id" yaml:"client_id"`
	ClientSecret string   `mapstructure:"client_secret" yaml:"client_secret"`
	RedirectURL  string   `mapstructure:"redirect_url" yaml:"redirect_url"`
	Scopes       []string `mapstructure:"scopes" yaml:"scopes"`
	AuthURL      string   `mapstructure:"auth_url" yaml:"auth_url"`
	TokenURL     string   `mapstructure:"token_url" yaml:"token_url"`
	UserInfoURL  string   `mapstructure:"user_info_url" yaml:"user_info_url"`
	Name         string   `mapstructure:"name" yaml:"name"`
}

type WebhookConfig struct {
	Timeout   time.Duration `mapstructure:"timeout" yaml:"timeout" default:"5s"`
	QueueSize int           `mapstructure:"queue_size" yaml:"queue_size" default:"100"`
}

type RealTimeConfig struct {
	Enabled               bool          `mapstructure:"enabled" yaml:"enabled" default:"true"`
	WebSocketEnabled      bool          `mapstructure:"websocket_enabled" yaml:"websocket_enabled" default:"true"`
	SSEEnabled            bool          `mapstructure:"sse_enabled" yaml:"sse_enabled" default:"true"`
	HeartbeatInterval     time.Duration `mapstructure:"heartbeat_interval" yaml:"heartbeat_interval" default:"30s"`
	ConnectionTimeout     time.Duration `mapstructure:"connection_timeout" yaml:"connection_timeout" default:"60s"`
	MinConnectionInterval time.Duration `mapstructure:"min_connection_interval" yaml:"min_connection_interval" default:"5s"`
	MaxConnections        int           `mapstructure:"max_connections" yaml:"max_connections" default:"1000"`
	MaxConnectionsPerUser int           `mapstructure:"max_connections_per_user" yaml:"max_connections_per_user" default:"5"`
	EventQueueSize        int           `mapstructure:"event_queue_size" yaml:"event_queue_size" default:"2048"`
	CleanupInterval       time.Duration `mapstructure:"cleanup_interval" yaml:"cleanup_interval" default:"2m"`
	StaleThreshold        time.Duration `mapstructure:"stale_threshold" yaml:"stale_threshold" default:"5m"`
	EnableCompression     bool          `mapstructure:"enable_compression" yaml:"enable_compression" default:"true"`
	EnableStats           bool          `mapstructure:"enable_stats" yaml:"enable_stats" default:"true"`
	AllowedOrigins        []string      `mapstructure:"allowed_origins" yaml:"allowed_origins"`
}

type MFAConfig struct {
	Enabled                 bool          `mapstructure:"enabled" yaml:"enabled" default:"true"`
	SessionTimeoutMinutes   int           `mapstructure:"session_timeout_minutes" yaml:"session_timeout_minutes" default:"15"`
	BackupCodeCount         int           `mapstructure:"backup_code_count" yaml:"backup_code_count" default:"10"`
	MaxVerificationAttempts int           `mapstructure:"max_verification_attempts" yaml:"max_verification_attempts" default:"5"`
	RateLimitWindow         time.Duration `mapstructure:"rate_limit_window" yaml:"rate_limit_window" default:"5m"`
}

type LogConfig struct {
	Level       string `mapstructure:"level" yaml:"level" default:"info"`
	Encoding    string `mapstructure:"encoding" yaml:"encoding" default:"console"`
	Development bool   `mapstructure:"development" yaml:"development" default:"false"`
}

func NewConfig() *Config {
	// Generate a secure secret instead of using a weak default
	secureSecret, err := generateSecureSecret()
	if err != nil {
		// If we can't generate a secure secret, panic - this is a security requirement
		panic(fmt.Sprintf("Failed to generate secure JWT secret: %v", err))
	}

	config := &Config{
		Telegram: TelegramConfig{
			Token: "",
		},
		Database: DatabaseConfig{
			Type:      "sqlite",
			Migration: true,
		},
		Jwt: JwtConfig{
			Secret:      secureSecret,
			SessionTime: 15 * time.Minute,
			MaxRefresh:  7 * 24 * time.Hour,
		},
		RealTimeConfig: RealTimeConfig{
			Enabled:               true,
			WebSocketEnabled:      false,
			SSEEnabled:            true,
			HeartbeatInterval:     30 * time.Second,
			ConnectionTimeout:     60 * time.Second,
			MinConnectionInterval: 5 * time.Second,
			MaxConnections:        1000,
			MaxConnectionsPerUser: 5,
			EventQueueSize:        2048,
			CleanupInterval:       2 * time.Minute,
			StaleThreshold:        5 * time.Minute,
			EnableCompression:     true,
			EnableStats:           true,
			AllowedOrigins:        []string{"*"},
		},
		Logging: LogConfig{
			Level:       "info",
			Encoding:    "console",
			Development: false,
		},
	}

	// Apply default values for fields with default tags
	defaults.SetDefaults(config)

	return config
}

func configEnvironmentOverrides(Config *Config) {
	if os.Getenv("DONETICK_TELEGRAM_TOKEN") != "" {
		Config.Telegram.Token = os.Getenv("DONETICK_TELEGRAM_TOKEN")
	}
	if os.Getenv("DONETICK_PUSHOVER_TOKEN") != "" {
		Config.Pushover.Token = os.Getenv("DONETICK_PUSHOVER_TOKEN")
	}
	if os.Getenv("DONETICK_DISABLE_SIGNUP") == "true" {
		Config.IsUserCreationDisabled = true
	}

	// Logging environment overrides
	if os.Getenv("DONETICK_LOG_LEVEL") != "" {
		Config.Logging.Level = os.Getenv("DONETICK_LOG_LEVEL")
	}
	if os.Getenv("DONETICK_LOG_ENCODING") != "" {
		Config.Logging.Encoding = os.Getenv("DONETICK_LOG_ENCODING")
	}
	if os.Getenv("DONETICK_LOG_DEVELOPMENT") == "true" {
		Config.Logging.Development = true
	}
}
func LoadConfig() *Config {
	// https://github.com/spf13/viper/issues/1895#issuecomment-3316091229
	viper.SetOptions(viper.ExperimentalBindStruct())

	// set the config name based on the environment:
	if os.Getenv("DT_ENV") == "local" {
		viper.SetConfigName("local")
	} else if os.Getenv("DT_ENV") == "prod" {
		viper.SetConfigName("prod")
	} else if os.Getenv("DT_ENV") == "selfhosted" {
		viper.SetConfigName("selfhosted")
	} else {
		viper.SetConfigName("local")
	}
	// get logger and log the current environment:
	fmt.Printf("--ConfigLoad config for environment: %s\n", os.Getenv("DT_ENV"))
	viper.SetEnvPrefix("DT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.AddConfigPath("./config")
	viper.SetConfigType("yaml")

	err := viper.ReadInConfig()
	// print a useful error:
	if err != nil {
		var configNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configNotFoundError) {
			fmt.Printf("Config file not found, using defaults and environment variables")
		} else {
			fmt.Printf("Error reading config file: %v", err)
			panic(err)
		}
	}
	// Override with environment variables if set:
	viper.AutomaticEnv()

	var config Config
	err = viper.Unmarshal(&config)
	if err != nil {
		panic(err)
	}

	// Apply default values for fields with default tags
	defaults.SetDefaults(&config)

	fmt.Printf("--ConfigLoad name : %s ", config.Name)

	configEnvironmentOverrides(&config)

	// Validate JWT secret strength
	if config.Name != "local" {
		validateJWTSecret(config.Jwt.Secret)
	}

	config.Info.Version = Version
	config.Info.Commit = Commit
	config.Info.BuildDate = BuildDate

	// set the timezone to UTC if not set:
	if os.Getenv("TZ") == "" {
		os.Setenv("TZ", "UTC")
	}
	time.Local, _ = time.LoadLocation(os.Getenv("TZ"))

	return &config

	// return LocalConfig()
}

// List of known weak secrets that should not be allowed
var weakSecrets = []string{
	"secret",
	"mysecret",
	"jwt_secret",
	"default",
	"password",
	"123456",
	"changeme",
	"donetick",
	"jwt",
	"token",
	"key",
	"secretkey",
	"change_this_to_a_secure_random_string_32_characters_long",
}

// validateJWTSecret validates that the JWT secret is strong enough
func validateJWTSecret(secret string) error {
	var err error
	if len(secret) < 32 {
		err = fmt.Errorf("JWT secret must be at least 32 characters long, got %d characters", len(secret))
	}

	// Check against known weak secrets (case-insensitive)
	secretLower := strings.ToLower(secret)
	for _, weak := range weakSecrets {
		if secretLower == weak {
			err = fmt.Errorf("JWT secret is too weak: '%s' is not allowed", weak)
			break
		}
	}
	if err != nil {
		fmt.Printf("\n\n🚨 SECURITY ERROR: %s\n", err.Error())
		fmt.Printf("\n💡 To fix this issue:\n")
		fmt.Printf("   1. Generate a secure JWT secret:\n")
		if secureSecret, genErr := generateSecureSecret(); genErr == nil {
			fmt.Printf("      Suggested secret: %s\n", secureSecret)
		}
		fmt.Printf("   2. Set it in your configuration:\n")
		fmt.Printf("      - YAML: jwt.secret: \"<your_secure_secret>\"\n")
		fmt.Printf("      - ENV:  export DT_JWT_SECRET=\"<your_secure_secret>\"\n")
		fmt.Printf("\n❌ Application will not start with weak JWT secrets for security reasons.\n\n")
		panic("Weak JWT secret detected - application startup aborted for security")
	}
	return nil
}

// generateSecureSecret generates a cryptographically secure random secret
func generateSecureSecret() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// ParseLogLevel converts a string log level to zapcore.Level
func (c *LogConfig) ParseLogLevel() zapcore.Level {
	switch strings.ToLower(c.Level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "dpanic":
		return zapcore.DPanicLevel
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}
