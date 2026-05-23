package metrics

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"search-trends/internal/storage"
)

type Counter struct {
	value atomic.Uint64
}

func (c *Counter) Inc() {
	c.value.Add(1)
}

func (c *Counter) Add(value uint64) {
	c.value.Add(value)
}

func (c *Counter) Get() uint64 {
	return c.value.Load()
}

type Metrics struct {
	EventsReceived  Counter
	EventsAccepted  Counter
	EventsInvalid   Counter
	EventsOld       Counter
	EventsDuplicate Counter
	EventsStopword  Counter
	EventsAbuse     Counter
	BadMessages     Counter

	TopRequests Counter

	TopDurationCount  Counter
	TopDurationSumNS  Counter
	TopDurationBucket [9]Counter
}

func New() *Metrics {
	return &Metrics{}
}

func (m *Metrics) ObserveTopDuration(duration time.Duration) {
	m.TopDurationCount.Inc()
	m.TopDurationSumNS.Add(uint64(duration.Nanoseconds()))

	seconds := duration.Seconds()
	buckets := []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1}
	for i, bucket := range buckets {
		if seconds <= bucket {
			m.TopDurationBucket[i].Inc()
		}
	}
}

func (m *Metrics) WritePrometheus(w io.Writer, snapshot storage.Snapshot, ready bool) {
	writeCounter(w, "search_events_received_total", "Total broker messages received", m.EventsReceived.Get())
	writeCounter(w, "search_events_accepted_total", "Total accepted search events", m.EventsAccepted.Get())
	writeCounter(w, "search_events_invalid_total", "Total invalid search events", m.EventsInvalid.Get())
	writeCounter(w, "search_events_old_total", "Total events rejected because timestamp is outside the sliding window", m.EventsOld.Get())
	writeCounter(w, "search_events_duplicate_total", "Total duplicate events rejected by event_id", m.EventsDuplicate.Get())
	writeCounter(w, "search_events_stopword_total", "Total events rejected by stop-list", m.EventsStopword.Get())
	writeCounter(w, "search_events_abuse_total", "Total events rejected by anti-abuse rules", m.EventsAbuse.Get())
	writeCounter(w, "search_bad_messages_total", "Total broker messages that could not be parsed", m.BadMessages.Get())
	writeCounter(w, "search_top_requests_total", "Total /top requests", m.TopRequests.Get())

	writeGauge(w, "search_ready", "Readiness status of the service", boolToFloat(ready))
	writeGauge(w, "search_active_queries", "Number of unique queries in current window", float64(snapshot.ActiveQueries))
	writeGauge(w, "search_window_events", "Number of events in current window", float64(snapshot.WindowEvents))
	writeGauge(w, "search_stopwords", "Number of configured stopwords", float64(snapshot.Stopwords))

	fmt.Fprintln(w, "# HELP search_top_request_duration_seconds Duration of /top requests")
	fmt.Fprintln(w, "# TYPE search_top_request_duration_seconds histogram")

	buckets := []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1}
	for i, bucket := range buckets {
		fmt.Fprintf(w, "search_top_request_duration_seconds_bucket{le=\"%.3f\"} %d\n", bucket, m.TopDurationBucket[i].Get())
	}
	fmt.Fprintf(w, "search_top_request_duration_seconds_bucket{le=\"+Inf\"} %d\n", m.TopDurationCount.Get())
	fmt.Fprintf(w, "search_top_request_duration_seconds_sum %.9f\n", float64(m.TopDurationSumNS.Get())/1e9)
	fmt.Fprintf(w, "search_top_request_duration_seconds_count %d\n", m.TopDurationCount.Get())
}

func writeCounter(w io.Writer, name string, help string, value uint64) {
	fmt.Fprintf(w, "# HELP %s %s\n", name, help)
	fmt.Fprintf(w, "# TYPE %s counter\n", name)
	fmt.Fprintf(w, "%s %d\n", name, value)
}

func writeGauge(w io.Writer, name string, help string, value float64) {
	fmt.Fprintf(w, "# HELP %s %s\n", name, help)
	fmt.Fprintf(w, "# TYPE %s gauge\n", name)
	fmt.Fprintf(w, "%s %.0f\n", name, value)
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
