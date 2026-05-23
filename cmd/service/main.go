package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"search-trends/internal/config"
	"search-trends/internal/consumer"
	"search-trends/internal/httpapi"
	"search-trends/internal/metrics"
	"search-trends/internal/service"
	"search-trends/internal/storage"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := storage.NewStore(cfg.Store)
	appMetrics := metrics.New()
	tracker := service.NewTracker(store, appMetrics, cfg.MaxPayloadBytes)

	natsConsumer := &consumer.NATSConsumer{
		URL:     cfg.NATSURL,
		Subject: cfg.NATSSubject,
		Queue:   cfg.NATSQueue,
	}

	go natsConsumer.Run(ctx, tracker.HandleMessage)
	go refreshLoop(ctx, store, cfg.RefreshEvery)

	apiServer := httpapi.NewServer(store, appMetrics, natsConsumer.IsConnected, cfg.MaxTopLimit, cfg.AdminToken)
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           apiServer.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Println("http server started on", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Println("http shutdown error:", err)
	}
}

func refreshLoop(ctx context.Context, store *storage.Store, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			store.Refresh(now.UTC())
		}
	}
}
