package cache

import (
	"context"
	"testing"
	"time"
)

func BenchmarkValidateCache_Hit(b *testing.B) {
	c := New(context.Background(), time.Minute)
	c.Set(context.Background(), "k", "user-payload", time.Minute)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(context.Background(), "k")
	}
}

func BenchmarkMemoryCache_Set(b *testing.B) {
	c := New(context.Background(), time.Minute)
	payload := "user-payload"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(context.Background(), "k", payload, time.Minute)
	}
}
