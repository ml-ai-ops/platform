package store

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

type EventPublisher interface {
	Publish(context.Context, string, any) error
}

type OutboxWorker struct {
	repository *Postgres
	publisher  EventPublisher
	interval   time.Duration
}

func NewOutboxWorker(repository *Postgres, publisher EventPublisher, interval time.Duration) *OutboxWorker {
	if interval <= 0 {
		interval = time.Second
	}
	return &OutboxWorker{repository: repository, publisher: publisher, interval: interval}
}

func (w *OutboxWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		if err := w.Flush(ctx); err != nil && ctx.Err() == nil {
			log.Printf("outbox flush failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *OutboxWorker) Flush(ctx context.Context) error {
	events, err := w.repository.PendingOutbox(ctx, 100)
	if err != nil {
		return err
	}
	for _, event := range events {
		var payload any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			_ = w.repository.MarkFailed(ctx, event.Sequence, err)
			continue
		}
		if err := w.publisher.Publish(ctx, event.Topic, payload); err != nil {
			_ = w.repository.MarkFailed(ctx, event.Sequence, err)
			continue
		}
		if err := w.repository.MarkPublished(ctx, event.Sequence); err != nil {
			return err
		}
	}
	return nil
}
