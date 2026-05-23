package storage

import (
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"search-trends/internal/domain"
)

type Config struct {
	Window                         time.Duration
	MaxFutureSkew                  time.Duration
	DedupTTL                       time.Duration
	MaxEventsPerUserQueryPerMinute int
	MaxQueryLength                 int
	MaxUserIDLength                int
	MaxEventIDLength               int
	MaxStopwordLength              int
}

type AddStatus string

const (
	StatusAccepted  AddStatus = "accepted"
	StatusInvalid   AddStatus = "invalid"
	StatusOld       AddStatus = "old"
	StatusDuplicate AddStatus = "duplicate"
	StatusStopword  AddStatus = "stopword"
	StatusAbuse     AddStatus = "abuse"
)

type Snapshot struct {
	ActiveQueries int
	WindowEvents  int64
	Stopwords     int
}

type Store struct {
	mu sync.Mutex

	cfg Config

	buckets []bucket
	counts  map[string]int64

	stopwords map[string]struct{}
	dedup     map[string]int64
	abuse     map[string]abuseCounter

	cachedTop []domain.TopItem
	dirty     bool
	ops       uint64
}

type bucket struct {
	unixSec int64
	counts  map[string]int64
}

type abuseCounter struct {
	minute int64
	count  int
}

func NewStore(cfg Config) *Store {
	if cfg.Window <= 0 {
		cfg.Window = 5 * time.Minute
	}
	if cfg.MaxFutureSkew <= 0 {
		cfg.MaxFutureSkew = 10 * time.Second
	}
	if cfg.DedupTTL <= 0 {
		cfg.DedupTTL = 10 * time.Minute
	}
	if cfg.MaxEventsPerUserQueryPerMinute <= 0 {
		cfg.MaxEventsPerUserQueryPerMinute = 30
	}
	if cfg.MaxQueryLength <= 0 {
		cfg.MaxQueryLength = 120
	}
	if cfg.MaxUserIDLength <= 0 {
		cfg.MaxUserIDLength = 128
	}
	if cfg.MaxEventIDLength <= 0 {
		cfg.MaxEventIDLength = 128
	}
	if cfg.MaxStopwordLength <= 0 {
		cfg.MaxStopwordLength = 120
	}

	bucketCount := int(cfg.Window.Seconds()) + int(cfg.MaxFutureSkew.Seconds()) + 5
	if bucketCount < 2 {
		bucketCount = 2
	}

	return &Store{
		cfg:       cfg,
		buckets:   make([]bucket, bucketCount),
		counts:    make(map[string]int64),
		stopwords: make(map[string]struct{}),
		dedup:     make(map[string]int64),
		abuse:     make(map[string]abuseCounter),
		cachedTop: make([]domain.TopItem, 0),
		dirty:     true,
	}
}

func (s *Store) Add(event domain.SearchEvent, now time.Time) AddStatus {
	query := Normalize(event.Query)
	if query == "" || tooLong(query, s.cfg.MaxQueryLength) {
		return StatusInvalid
	}

	if event.EventID == "" || tooLong(event.EventID, s.cfg.MaxEventIDLength) {
		return StatusInvalid
	}

	if event.UserID == "" || tooLong(event.UserID, s.cfg.MaxUserIDLength) {
		return StatusInvalid
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = now
	}

	if event.Timestamp.Before(now.Add(-s.cfg.Window)) {
		return StatusOld
	}

	if event.Timestamp.After(now.Add(s.cfg.MaxFutureSkew)) {
		event.Timestamp = now
	}

	event.Query = query

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupLocked(now)
	s.ops++
	if s.ops%1000 == 0 {
		s.cleanupSmallMapsLocked(now)
	}

	if event.EventID != "" {
		expiresAt := s.dedup[event.EventID]
		if expiresAt > now.Unix() {
			return StatusDuplicate
		}
		s.dedup[event.EventID] = now.Add(s.cfg.DedupTTL).Unix()
	}

	if _, blocked := s.stopwords[query]; blocked {
		return StatusStopword
	}

	if s.isAbuseLocked(event, now) {
		return StatusAbuse
	}

	sec := event.Timestamp.Unix()
	idx := int(sec % int64(len(s.buckets)))
	if idx < 0 {
		idx = -idx
	}

	b := &s.buckets[idx]
	if b.unixSec != sec {
		s.dropBucketLocked(b)
		b.unixSec = sec
		b.counts = make(map[string]int64)
	}

	b.counts[query]++
	s.counts[query]++
	s.dirty = true

	return StatusAccepted
}

func (s *Store) Refresh(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupLocked(now)
	if s.dirty {
		s.rebuildTopLocked()
	}
}

func (s *Store) Top(limit int, now time.Time) []domain.TopItem {
	if limit <= 0 {
		limit = 10
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupLocked(now)
	if s.dirty {
		s.rebuildTopLocked()
	}

	if limit > len(s.cachedTop) {
		limit = len(s.cachedTop)
	}

	result := make([]domain.TopItem, limit)
	copy(result, s.cachedTop[:limit])
	return result
}

func (s *Store) AddStopword(word string) bool {
	word = Normalize(word)
	if word == "" || tooLong(word, s.cfg.MaxStopwordLength) {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, existed := s.stopwords[word]
	s.stopwords[word] = struct{}{}
	s.dirty = true
	return !existed
}

func (s *Store) DeleteStopword(word string) bool {
	word = Normalize(word)
	if word == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, existed := s.stopwords[word]
	delete(s.stopwords, word)
	s.dirty = true
	return existed
}

func (s *Store) Stopwords() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]string, 0, len(s.stopwords))
	for word := range s.stopwords {
		items = append(items, word)
	}

	sort.Strings(items)
	return items
}

func (s *Store) Snapshot(now time.Time) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupLocked(now)

	var events int64
	for _, count := range s.counts {
		events += count
	}

	return Snapshot{
		ActiveQueries: len(s.counts),
		WindowEvents:  events,
		Stopwords:     len(s.stopwords),
	}
}

func (s *Store) cleanupLocked(now time.Time) {
	border := now.Add(-s.cfg.Window).Unix()
	for i := range s.buckets {
		if s.buckets[i].unixSec > 0 && s.buckets[i].unixSec < border {
			s.dropBucketLocked(&s.buckets[i])
		}
	}
}

func (s *Store) cleanupSmallMapsLocked(now time.Time) {
	nowUnix := now.Unix()
	for eventID, expiresAt := range s.dedup {
		if expiresAt <= nowUnix {
			delete(s.dedup, eventID)
		}
	}

	currentMinute := nowUnix / 60
	for key, item := range s.abuse {
		if item.minute < currentMinute-10 {
			delete(s.abuse, key)
		}
	}
}

func (s *Store) dropBucketLocked(b *bucket) {
	if b.counts == nil {
		b.unixSec = 0
		return
	}

	for query, count := range b.counts {
		s.counts[query] -= count
		if s.counts[query] <= 0 {
			delete(s.counts, query)
		}
	}

	b.unixSec = 0
	b.counts = nil
	s.dirty = true
}

func (s *Store) rebuildTopLocked() {
	items := make([]domain.TopItem, 0, len(s.counts))
	for query, count := range s.counts {
		if count <= 0 {
			continue
		}
		if _, blocked := s.stopwords[query]; blocked {
			continue
		}

		items = append(items, domain.TopItem{Query: query, Count: count})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Query < items[j].Query
		}
		return items[i].Count > items[j].Count
	})

	s.cachedTop = items
	s.dirty = false
}

func (s *Store) isAbuseLocked(event domain.SearchEvent, now time.Time) bool {
	if event.UserID == "" || s.cfg.MaxEventsPerUserQueryPerMinute <= 0 {
		return false
	}

	minute := now.Unix() / 60
	key := event.UserID + "\x00" + event.Query
	item := s.abuse[key]
	if item.minute != minute {
		item = abuseCounter{minute: minute, count: 0}
	}

	item.count++
	s.abuse[key] = item
	return item.count > s.cfg.MaxEventsPerUserQueryPerMinute
}

func tooLong(value string, max int) bool {
	return max > 0 && utf8.RuneCountInString(value) > max
}

func Normalize(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(value))

	lastSpace := false
	for _, r := range value {
		if unicode.IsSpace(r) {
			if !lastSpace {
				builder.WriteRune(' ')
				lastSpace = true
			}
			continue
		}

		builder.WriteRune(r)
		lastSpace = false
	}

	return strings.TrimSpace(builder.String())
}
