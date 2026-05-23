package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"search-trends/internal/domain"
	"search-trends/internal/natsclient"
)

func main() {
	var (
		natsURL  = flag.String("nats-url", getenv("NATS_URL", "nats://localhost:4222"), "NATS url")
		subject  = flag.String("subject", getenv("NATS_SUBJECT", "search.events"), "NATS subject")
		once     = flag.Bool("once", false, "publish one event and exit")
		rate     = flag.Int("rate", 50, "events per second")
		duration = flag.Duration("duration", 0, "duration for loop mode, 0 means forever")
		query    = flag.String("query", "", "fixed query for generated events")
		userID   = flag.String("user", "", "fixed user_id for generated events")
		abuse    = flag.Bool("abuse", false, "generate repeated events from one user and one query")
		bad      = flag.Bool("bad", false, "publish a bad non-json message")
	)
	flag.Parse()

	ctx := context.Background()
	client, err := natsclient.Dial(ctx, *natsURL)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	if *bad {
		if err := client.Publish(*subject, []byte("not-json")); err != nil {
			log.Fatal(err)
		}
		_ = client.Flush()
		log.Println("published bad message")
		return
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	if *once {
		event := buildEvent(rng, *query, *userID, *abuse)
		if err := client.PublishJSON(*subject, event); err != nil {
			log.Fatal(err)
		}
		_ = client.Flush()
		log.Printf("published: %+v\n", event)
		return
	}

	if *rate <= 0 {
		*rate = 1
	}

	interval := time.Second / time.Duration(*rate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var deadline <-chan time.Time
	if *duration > 0 {
		deadline = time.After(*duration)
	}

	for {
		select {
		case <-deadline:
			_ = client.Flush()
			return
		case <-ticker.C:
			event := buildEvent(rng, *query, *userID, *abuse)
			if err := client.PublishJSON(*subject, event); err != nil {
				log.Println("publish error:", err)
				continue
			}
		}
	}
}

func buildEvent(rng *rand.Rand, fixedQuery string, fixedUser string, abuse bool) domain.SearchEvent {
	queries := []string{
		"кроссовки", "айфон", "платье", "рюкзак", "наушники",
		"куртка", "ноутбук", "пылесос", "детская коляска", "корм для кошек",
	}

	query := fixedQuery
	if query == "" {
		query = queries[rng.Intn(len(queries))]
	}

	userID := fixedUser
	if userID == "" {
		userID = fmt.Sprintf("user-%d", 1+rng.Intn(5000))
	}

	if abuse {
		query = "накрутка"
		userID = "bot-1"
	}

	now := time.Now().UTC()
	return domain.SearchEvent{
		EventID:   fmt.Sprintf("%d-%d", now.UnixNano(), rng.Int63()),
		Query:     query,
		UserID:    userID,
		SessionID: fmt.Sprintf("session-%d", 1+rng.Intn(10000)),
		IPHash:    fmt.Sprintf("iphash-%d", 1+rng.Intn(2000)),
		Timestamp: now,
	}
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
