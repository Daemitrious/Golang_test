package consumer

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"search-trends/internal/natsclient"
)

type NATSConsumer struct {
	URL       string
	Subject   string
	Queue     string
	Connected atomic.Bool
}

func (c *NATSConsumer) Run(ctx context.Context, handler func([]byte)) {
	backoff := time.Second

	for {
		select {
		case <-ctx.Done():
			c.Connected.Store(false)
			return
		default:
		}

		client, err := natsclient.Dial(ctx, c.URL)
		if err != nil {
			c.Connected.Store(false)
			log.Println("nats connect error:", err)
			sleep(ctx, backoff)
			backoff = nextBackoff(backoff)
			continue
		}

		if err := client.Subscribe(c.Subject, c.Queue, "1"); err != nil {
			_ = client.Close()
			c.Connected.Store(false)
			log.Println("nats subscribe error:", err)
			sleep(ctx, backoff)
			backoff = nextBackoff(backoff)
			continue
		}

		c.Connected.Store(true)
		backoff = time.Second
		log.Println("nats consumer connected. subject:", c.Subject, "queue:", c.Queue)

		err = client.ReadLoop(ctx, handler)
		_ = client.Close()
		c.Connected.Store(false)

		if ctx.Err() != nil {
			return
		}

		log.Println("nats read loop stopped:", err)
		sleep(ctx, backoff)
		backoff = nextBackoff(backoff)
	}
}

func (c *NATSConsumer) IsConnected() bool {
	return c.Connected.Load()
}

func sleep(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func nextBackoff(current time.Duration) time.Duration {
	current *= 2
	if current > 10*time.Second {
		return 10 * time.Second
	}
	return current
}
