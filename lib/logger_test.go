package authclient

import (
	"context"
	"errors"
	"testing"
)

func TestNoopLogger_Smoke(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		run func(l Logger)
	}{
		"info_happy_path": {
			run: func(l Logger) { l.Info(context.Background(), "msg", "k", "v") },
		},
		"info_no_panic_with_nil_ctx": {
			run: func(l Logger) {
				var nilCtx context.Context
				l.Info(nilCtx, "msg")
			},
		},
		"info_no_panic_with_empty_fields": {
			run: func(l Logger) { l.Info(context.Background(), "msg") },
		},
		"warn_happy_path": {
			run: func(l Logger) { l.Warn(context.Background(), "msg", "k", "v") },
		},
		"warn_no_panic_with_nil_ctx": {
			run: func(l Logger) {
				var nilCtx context.Context
				l.Warn(nilCtx, "msg")
			},
		},
		"warn_no_panic_with_empty_fields": {
			run: func(l Logger) { l.Warn(context.Background(), "msg") },
		},
		"error_happy_path": {
			run: func(l Logger) { l.Error(context.Background(), "boom", errors.New("x"), "k", "v") },
		},
		"error_no_panic_with_nil_err": {
			run: func(l Logger) { l.Error(context.Background(), "msg", nil) },
		},
		"error_no_panic_with_nil_ctx_and_nil_err": {
			run: func(l Logger) {
				var nilCtx context.Context
				l.Error(nilCtx, "msg", nil)
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tc.run(NoopLogger{})
		})
	}
}
