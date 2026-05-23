package domain

import "time"

type SearchEvent struct {
	EventID   string    `json:"event_id"`
	Query     string    `json:"query"`
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id,omitempty"`
	IPHash    string    `json:"ip_hash,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type TopItem struct {
	Query string `json:"query"`
	Count int64  `json:"count"`
}
