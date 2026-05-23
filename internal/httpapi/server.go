package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"search-trends/internal/metrics"
	"search-trends/internal/storage"
)

type Server struct {
	store       *storage.Store
	metrics     *metrics.Metrics
	ready       func() bool
	maxTopLimit int
	adminToken  string
}

func NewServer(store *storage.Store, metrics *metrics.Metrics, ready func() bool, maxTopLimit int, adminToken string) *Server {
	if maxTopLimit <= 0 {
		maxTopLimit = 100
	}

	return &Server{
		store:       store,
		metrics:     metrics,
		ready:       ready,
		maxTopLimit: maxTopLimit,
		adminToken:  adminToken,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/readyz", s.readyz)
	mux.HandleFunc("/top", s.top)
	mux.HandleFunc("/stopwords", s.stopwords)
	mux.HandleFunc("/stopwords/", s.stopwordByWord)
	mux.HandleFunc("/metrics", s.prometheusMetrics)
	return mux
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if s.ready != nil && !s.ready() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ready": false})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ready": true})
}

func (s *Server) top(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	startedAt := time.Now()
	s.metrics.TopRequests.Inc()
	defer func() {
		s.metrics.ObserveTopDuration(time.Since(startedAt))
	}()

	limit := 10
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be positive integer")
			return
		}
		limit = parsed
	}

	if limit > s.maxTopLimit {
		limit = s.maxTopLimit
	}

	items := s.store.Top(limit, time.Now().UTC())
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) stopwords(w http.ResponseWriter, r *http.Request) {
	if !s.checkAdmin(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"items": s.store.Stopwords()})

	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		var request struct {
			Word string `json:"word"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "bad json")
			return
		}

		if !s.store.AddStopword(request.Word) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "already_exists_or_empty"})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"status": "added"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) stopwordByWord(w http.ResponseWriter, r *http.Request) {
	if !s.checkAdmin(w, r) {
		return
	}

	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	word := strings.TrimPrefix(r.URL.Path, "/stopwords/")
	word, _ = url.PathUnescape(word)
	if word == "" {
		writeError(w, http.StatusBadRequest, "word is required")
		return
	}

	if !s.store.DeleteStopword(word) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "not_found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) checkAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.adminToken == "" {
		return true
	}

	if r.Header.Get("X-Admin-Token") == s.adminToken {
		return true
	}

	writeError(w, http.StatusUnauthorized, "admin token is required")
	return false
}

func (s *Server) prometheusMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	snapshot := s.store.Snapshot(time.Now().UTC())
	ready := s.ready == nil || s.ready()
	s.metrics.WritePrometheus(w, snapshot, ready)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
