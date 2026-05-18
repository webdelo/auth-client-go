package authclient

import "context"

// Logger is the adapter interface used by the SDK for diagnostic logging.
//
// Methods follow slog-style variadic fields: callers pass key-value pairs
// (e.g., "user_uuid", uuid, "method", "Validate"). The SDK does not depend
// on any concrete logging library (slog, zap, logrus, etc.) — to plug in a
// concrete logger, wrap it in a small adapter that satisfies this interface
// and pass it via Config.Logger or WithLogger.
type Logger interface {
	Info(ctx context.Context, msg string, fields ...any)
	Warn(ctx context.Context, msg string, fields ...any)
	Error(ctx context.Context, msg string, err error, fields ...any)
}

// NoopLogger is the silent default Logger. It is used when Config.Logger is
// nil so that the SDK avoids per-call-site nil-checks at every logging call site.
type NoopLogger struct{}

// Info implements Logger. No-op.
func (NoopLogger) Info(context.Context, string, ...any) {}

// Warn implements Logger. No-op.
func (NoopLogger) Warn(context.Context, string, ...any) {}

// Error implements Logger. No-op.
func (NoopLogger) Error(context.Context, string, error, ...any) {}

var _ Logger = NoopLogger{}
