package storage

import (
	"fmt"
	"testing"
	"time"

	"search-trends/internal/domain"
)

func BenchmarkAddEvent(b *testing.B) {
	now := time.Now().UTC()
	store := NewStore(Config{Window: 5 * time.Minute, MaxEventsPerUserQueryPerMinute: 1000000})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Add(domain.SearchEvent{
			EventID:   fmt.Sprintf("event-%d", i),
			Query:     fmt.Sprintf("query-%d", i%100),
			UserID:    fmt.Sprintf("user-%d", i%10000),
			Timestamp: now,
		}, now)
	}
}

func BenchmarkTopCached(b *testing.B) {
	now := time.Now().UTC()
	store := NewStore(Config{Window: 5 * time.Minute, MaxEventsPerUserQueryPerMinute: 1000000})

	for i := 0; i < 10000; i++ {
		store.Add(domain.SearchEvent{
			EventID:   fmt.Sprintf("event-%d", i),
			Query:     fmt.Sprintf("query-%d", i%1000),
			UserID:    fmt.Sprintf("user-%d", i),
			Timestamp: now,
		}, now)
	}
	store.Refresh(now)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Top(10, now)
	}
}
