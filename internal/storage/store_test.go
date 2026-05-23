package storage

import (
	"testing"
	"time"

	"search-trends/internal/domain"
)

func TestTopReturnsMostPopularQueries(t *testing.T) {
	now := time.Now().UTC()
	store := NewStore(Config{Window: 5 * time.Minute, MaxEventsPerUserQueryPerMinute: 100})

	store.Add(domain.SearchEvent{EventID: "1", Query: "Айфон", UserID: "u1", Timestamp: now}, now)
	store.Add(domain.SearchEvent{EventID: "2", Query: "кроссовки", UserID: "u2", Timestamp: now}, now)
	store.Add(domain.SearchEvent{EventID: "3", Query: "айфон", UserID: "u3", Timestamp: now}, now)

	top := store.Top(10, now)
	if len(top) != 2 {
		t.Fatalf("expected 2 items, got %d", len(top))
	}
	if top[0].Query != "айфон" || top[0].Count != 2 {
		t.Fatalf("unexpected first item: %+v", top[0])
	}
}

func TestOldEventsAreRemoved(t *testing.T) {
	now := time.Now().UTC()
	store := NewStore(Config{Window: 5 * time.Minute, MaxEventsPerUserQueryPerMinute: 100})

	status := store.Add(domain.SearchEvent{EventID: "1", Query: "old", UserID: "u1", Timestamp: now.Add(-10 * time.Minute)}, now)
	if status != StatusOld {
		t.Fatalf("expected old status, got %s", status)
	}

	store.Add(domain.SearchEvent{EventID: "2", Query: "new", UserID: "u1", Timestamp: now}, now)
	top := store.Top(10, now)

	if len(top) != 1 || top[0].Query != "new" {
		t.Fatalf("unexpected top: %+v", top)
	}
}

func TestStopwordsHideQuery(t *testing.T) {
	now := time.Now().UTC()
	store := NewStore(Config{Window: 5 * time.Minute, MaxEventsPerUserQueryPerMinute: 100})

	store.Add(domain.SearchEvent{EventID: "1", Query: "айфон", UserID: "u1", Timestamp: now}, now)
	store.Add(domain.SearchEvent{EventID: "2", Query: "кроссовки", UserID: "u2", Timestamp: now}, now)
	store.AddStopword("айфон")

	top := store.Top(10, now)
	if len(top) != 1 || top[0].Query != "кроссовки" {
		t.Fatalf("unexpected top: %+v", top)
	}
}

func TestDedupByEventID(t *testing.T) {
	now := time.Now().UTC()
	store := NewStore(Config{Window: 5 * time.Minute, MaxEventsPerUserQueryPerMinute: 100, DedupTTL: 10 * time.Minute})

	first := store.Add(domain.SearchEvent{EventID: "same", Query: "айфон", UserID: "u1", Timestamp: now}, now)
	second := store.Add(domain.SearchEvent{EventID: "same", Query: "айфон", UserID: "u1", Timestamp: now}, now)

	if first != StatusAccepted || second != StatusDuplicate {
		t.Fatalf("unexpected statuses: %s, %s", first, second)
	}

	top := store.Top(10, now)
	if len(top) != 1 || top[0].Count != 1 {
		t.Fatalf("unexpected top: %+v", top)
	}
}

func TestAntiAbuse(t *testing.T) {
	now := time.Now().UTC()
	store := NewStore(Config{Window: 5 * time.Minute, MaxEventsPerUserQueryPerMinute: 2})

	statuses := []AddStatus{
		store.Add(domain.SearchEvent{EventID: "1", Query: "бот", UserID: "u1", Timestamp: now}, now),
		store.Add(domain.SearchEvent{EventID: "2", Query: "бот", UserID: "u1", Timestamp: now}, now),
		store.Add(domain.SearchEvent{EventID: "3", Query: "бот", UserID: "u1", Timestamp: now}, now),
	}

	if statuses[0] != StatusAccepted || statuses[1] != StatusAccepted || statuses[2] != StatusAbuse {
		t.Fatalf("unexpected statuses: %+v", statuses)
	}

	top := store.Top(10, now)
	if len(top) != 1 || top[0].Count != 2 {
		t.Fatalf("unexpected top: %+v", top)
	}
}

func TestNormalize(t *testing.T) {
	got := Normalize("  АйФон   15  ")
	if got != "айфон 15" {
		t.Fatalf("unexpected normalize result: %q", got)
	}
}

func TestRejectsInvalidIdentityFields(t *testing.T) {
	now := time.Now().UTC()
	store := NewStore(Config{Window: 5 * time.Minute})

	cases := []domain.SearchEvent{
		{Query: "кроссовки", UserID: "u1", Timestamp: now},
		{EventID: "e1", Query: "кроссовки", Timestamp: now},
		{EventID: "e2", Query: "", UserID: "u2", Timestamp: now},
	}

	for _, item := range cases {
		if got := store.Add(item, now); got != StatusInvalid {
			t.Fatalf("expected invalid status, got %s", got)
		}
	}
}

func TestRejectsTooLongQuery(t *testing.T) {
	now := time.Now().UTC()
	store := NewStore(Config{Window: 5 * time.Minute, MaxQueryLength: 5})

	got := store.Add(domain.SearchEvent{
		EventID:   "e1",
		Query:     "очень длинный запрос",
		UserID:    "u1",
		Timestamp: now,
	}, now)

	if got != StatusInvalid {
		t.Fatalf("expected invalid status, got %s", got)
	}
}
