package config

import (
	"os"
	"strconv"
	"time"

	"search-trends/internal/storage"
)

type Config struct {
	HTTPAddr        string
	AdminToken      string
	NATSURL         string
	NATSSubject     string
	NATSQueue       string
	MaxTopLimit     int
	MaxPayloadBytes int
	RefreshEvery    time.Duration
	Store           storage.Config
}

func Load() Config {
	windowSeconds := getInt("WINDOW_SECONDS", 300)

	return Config{
		HTTPAddr:        getString("HTTP_ADDR", ":8080"),
		AdminToken:      getString("ADMIN_TOKEN", ""),
		NATSURL:         getString("NATS_URL", "nats://localhost:4222"),
		NATSSubject:     getString("NATS_SUBJECT", "search.events"),
		NATSQueue:       getString("NATS_QUEUE", "search-trends"),
		MaxTopLimit:     getInt("MAX_TOP_LIMIT", 100),
		MaxPayloadBytes: getInt("MAX_PAYLOAD_BYTES", 4096),
		RefreshEvery:    time.Duration(getInt("REFRESH_INTERVAL_MS", 500)) * time.Millisecond,
		Store: storage.Config{
			Window:                         time.Duration(windowSeconds) * time.Second,
			MaxFutureSkew:                  time.Duration(getInt("MAX_FUTURE_SKEW_SECONDS", 10)) * time.Second,
			DedupTTL:                       time.Duration(getInt("DEDUP_TTL_SECONDS", 600)) * time.Second,
			MaxEventsPerUserQueryPerMinute: getInt("MAX_EVENTS_PER_USER_QUERY_PER_MINUTE", 30),
			MaxQueryLength:                 getInt("MAX_QUERY_LENGTH", 120),
			MaxUserIDLength:                getInt("MAX_USER_ID_LENGTH", 128),
			MaxEventIDLength:               getInt("MAX_EVENT_ID_LENGTH", 128),
			MaxStopwordLength:              getInt("MAX_STOPWORD_LENGTH", 120),
		},
	}
}

func getString(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
