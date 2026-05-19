package authclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExpiresIn(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		unixSeconds int64
		want        func(*testing.T, time.Duration)
	}{
		"happy_path": {
			unixSeconds: time.Now().Add(time.Hour).Unix(),
			want: func(t *testing.T, d time.Duration) {
				// Allow ±1s slack for execution time.
				assert.InDelta(t, time.Hour.Seconds(), d.Seconds(), 1.0, "expiresIn(now+1h) should be ≈1h")
			},
		},
		"zero_returns_zero": {
			unixSeconds: 0,
			want: func(t *testing.T, d time.Duration) {
				assert.Equal(t, time.Duration(0), d)
			},
		},
		"negative_returns_zero": {
			unixSeconds: -1,
			want: func(t *testing.T, d time.Duration) {
				assert.Equal(t, time.Duration(0), d)
			},
		},
		"past_timestamp_returns_negative": {
			unixSeconds: time.Now().Add(-time.Hour).Unix(),
			want: func(t *testing.T, d time.Duration) {
				assert.Less(t, d, time.Duration(0), "past timestamp must yield negative duration")
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := expiresIn(tc.unixSeconds)
			tc.want(t, got)
		})
	}
}
