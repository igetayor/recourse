package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"
)

type contextKey string

const (
	TraceIDKey contextKey = "trace_id"
	UserIDKey  contextKey = "user_id"
	RoleKey    contextKey = "role"
)

var defaultLogger *slog.Logger

// InitLogger configures the structured JSON logger.
func InitLogger(levelStr string, serviceName string) *slog.Logger {
	var level slog.Level
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN", "WARNING":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	jsonHandler := slog.NewJSONHandler(os.Stdout, opts)
	baseLogger := slog.New(jsonHandler).With(slog.String("service", serviceName))
	slog.SetDefault(baseLogger)
	defaultLogger = baseLogger
	return baseLogger
}

// Get returns the default logger instance.
func Get() *slog.Logger {
	if defaultLogger == nil {
		return slog.Default()
	}
	return defaultLogger
}

// WithTraceID injects trace_id into context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		traceID = uuid.New().String()
	}
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// TraceIDFromContext retrieves the trace ID from context.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if val, ok := ctx.Value(TraceIDKey).(string); ok {
		return val
	}
	return ""
}

// FromContext decorates logger with contextual attributes (trace_id, user_id, role).
func FromContext(ctx context.Context) *slog.Logger {
	l := Get()
	if ctx == nil {
		return l
	}

	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		l = l.With(slog.String("trace_id", traceID))
	}
	if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
		l = l.With(slog.String("user_id", userID))
	}
	if role, ok := ctx.Value(RoleKey).(string); ok && role != "" {
		l = l.With(slog.String("role", role))
	}
	return l
}
