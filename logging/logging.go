package logging

import (
	"context"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey = string

const loggerKey = contextKey("logger")

var (
	defaultLogger     *zap.SugaredLogger
	defaultLoggerOnce sync.Once
)

var conf = &Config{
	Encoding:    "console",
	Level:       zapcore.InfoLevel,
	Development: true,
}

type Config struct {
	Encoding    string
	Level       zapcore.Level
	Development bool
}

// SetConfig sets given logging configs for DefaultLogger's logger.
// Must set configs before calling DefaultLogger()
func SetConfig(c *Config) {
	conf = &Config{
		Encoding:    c.Encoding,
		Level:       c.Level,
		Development: c.Development,
	}
}

// SetConfigFromAppConfig sets logging configuration from application config
func SetConfigFromAppConfig(level string, encoding string, development bool) {
	logLevel := parseLogLevel(level)
	conf = &Config{
		Encoding:    encoding,
		Level:       logLevel,
		Development: development,
	}
}

// parseLogLevel converts a string log level to zapcore.Level
func parseLogLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
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

func SetLevel(l zapcore.Level) {
	conf.Level = l
}

// NewLogger creates a new logger with the given log level
func NewLogger(conf *Config) *zap.SugaredLogger {
	ec := zap.NewProductionEncoderConfig()
	ec.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg := zap.Config{
		Encoding:         conf.Encoding,
		EncoderConfig:    ec,
		Level:            zap.NewAtomicLevelAt(conf.Level),
		Development:      conf.Development,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger, err := cfg.Build()
	if err != nil {
		logger = zap.NewNop()
	}
	return logger.Sugar()
}

// DefaultLogger returns the default logger for the package.
func DefaultLogger() *zap.SugaredLogger {
	defaultLoggerOnce.Do(func() {
		defaultLogger = NewLogger(conf)
	})
	return defaultLogger
}

// WithLogger creates a new context with the provided logger attached.
func WithLogger(ctx context.Context, logger *zap.SugaredLogger) context.Context {
	if gCtx, ok := ctx.(*gin.Context); ok {
		ctx = gCtx.Request.Context()
	}
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext returns the logger stored in the context. If no such logger
// exists, a default logger is returned.
func FromContext(ctx context.Context) *zap.SugaredLogger {
	if ctx == nil {
		return DefaultLogger()
	}
	if gCtx, ok := ctx.(*gin.Context); ok && gCtx != nil {
		ctx = gCtx.Request.Context()
	}
	if logger, ok := ctx.Value(loggerKey).(*zap.SugaredLogger); ok {
		return logger
	}
	return DefaultLogger()
}
