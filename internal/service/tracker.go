package service

import (
	"encoding/json"
	"time"

	"search-trends/internal/domain"
	"search-trends/internal/metrics"
	"search-trends/internal/storage"
)

type Tracker struct {
	store           *storage.Store
	metrics         *metrics.Metrics
	maxPayloadBytes int
}

func NewTracker(store *storage.Store, metrics *metrics.Metrics, maxPayloadBytes int) *Tracker {
	if maxPayloadBytes <= 0 {
		maxPayloadBytes = 4096
	}

	return &Tracker{
		store:           store,
		metrics:         metrics,
		maxPayloadBytes: maxPayloadBytes,
	}
}

func (t *Tracker) HandleMessage(payload []byte) {
	t.metrics.EventsReceived.Inc()

	if len(payload) == 0 || len(payload) > t.maxPayloadBytes {
		t.metrics.BadMessages.Inc()
		return
	}

	var event domain.SearchEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		t.metrics.BadMessages.Inc()
		return
	}

	status := t.store.Add(event, time.Now().UTC())
	switch status {
	case storage.StatusAccepted:
		t.metrics.EventsAccepted.Inc()
	case storage.StatusInvalid:
		t.metrics.EventsInvalid.Inc()
	case storage.StatusOld:
		t.metrics.EventsOld.Inc()
	case storage.StatusDuplicate:
		t.metrics.EventsDuplicate.Inc()
	case storage.StatusStopword:
		t.metrics.EventsStopword.Inc()
	case storage.StatusAbuse:
		t.metrics.EventsAbuse.Inc()
	}
}
