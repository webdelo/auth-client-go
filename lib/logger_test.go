package authclient

import (
	"context"
	"errors"
	"testing"
)

func TestNoopLogger_Info(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		run func(l Logger)
	}{
		"happy_path": {
			run: func(l Logger) {
				l.Info(context.Background(), "msg", "k", "v")
			},
		},
		"no_panic_with_nil_ctx": {
			run: func(l Logger) {
				var nilCtx context.Context
				l.Info(nilCtx, "msg")
			},
		},
		"no_panic_with_empty_fields": {
			run: func(l Logger) {
				l.Info(context.Background(), "msg")
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

func TestNoopLogger_Warn(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		run func(l Logger)
	}{
		"happy_path": {
			run: func(l Logger) {
				l.Warn(context.Background(), "msg", "k", "v")
			},
		},
		"no_panic_with_nil_ctx": {
			run: func(l Logger) {
				var nilCtx context.Context
				l.Warn(nilCtx, "msg")
			},
		},
		"no_panic_with_empty_fields": {
			run: func(l Logger) {
				l.Warn(context.Background(), "msg")
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

func TestNoopLogger_Error(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		run func(l Logger)
	}{
		"happy_path": {
			run: func(l Logger) {
				l.Error(context.Background(), "boom", errors.New("x"), "k", "v")
			},
		},
		"no_panic_with_nil_err": {
			run: func(l Logger) {
				l.Error(context.Background(), "msg", nil)
			},
		},
		"no_panic_with_nil_ctx_and_nil_err": {
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
