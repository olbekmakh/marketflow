package application

import (
	"context"
	"log/slog"
	"marketflow/internal/domain"
	"time"
)

type Worker struct {
	id      int
	cache   domain.CacheRepository
	batcher *Batcher
	input   chan domain.PriceUpdate
}

func NewWorker(id int, cache domain.CacheRepository, batcher *Batcher) *Worker {
	return &Worker{
		id:      id,
		cache:   cache,
		batcher: batcher,
		input:   make(chan domain.PriceUpdate, 100),
	}
}

func (w *Worker) Start(ctx context.Context) {
	slog.Info("Starting worker", "worker_id", w.id)

	processedCount := 0

	for {
		select {
		case update := <-w.input:
			start := time.Now()
			w.processUpdate(ctx, update)
			duration := time.Since(start)

			processedCount++

			if processedCount%100 == 0 {
				slog.Debug("Worker processing stats",
					"worker_id", w.id,
					"processed_count", processedCount,
					"last_duration", duration)
			}

		case <-ctx.Done():
			slog.Info("Stopping worker",
				"worker_id", w.id,
				"total_processed", processedCount)
			return
		}
	}
}

func (w *Worker) processUpdate(ctx context.Context, update domain.PriceUpdate) {
	for retry := 0; retry < 3; retry++ {
		if err := w.cache.SetLatestPrice(ctx, update.Exchange, update.Symbol, update.Price); err != nil {
			if retry == 2 {
				slog.Error("Failed to cache latest price after retries",
					"worker_id", w.id,
					"exchange", update.Exchange,
					"symbol", update.Symbol,
					"error", err)
			} else {
				slog.Warn("Retrying cache operation",
					"worker_id", w.id,
					"retry", retry+1,
					"error", err)
				time.Sleep(time.Duration(retry+1) * 100 * time.Millisecond)
			}
			continue
		}
		break
	}

	if err := w.cache.StoreRecentPrice(ctx, update.Exchange, update.Symbol, update.Price, update.Timestamp); err != nil {
		slog.Error("Failed to store recent price",
			"worker_id", w.id,
			"error", err)
	}
}

func (w *Worker) Input() chan<- domain.PriceUpdate {
	return w.input
}
