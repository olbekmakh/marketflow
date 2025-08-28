package application

import (
	"context"
	"log/slog"

	"marketflow/internal/domain"
)

type Worker struct {
	id    int
	db    domain.PriceRepository
	cache domain.CacheRepository
	input chan domain.PriceUpdate
}

func NewWorker(id int, db domain.PriceRepository, cache domain.CacheRepository) *Worker {
	return &Worker{
		id:    id,
		db:    db,
		cache: cache,
		input: make(chan domain.PriceUpdate, 100),
	}
}

func (w *Worker) Start(ctx context.Context) {
	slog.Info("Starting worker", "id", w.id)
	
	for {
		select {
		case update := <-w.input:
			w.processUpdate(ctx, update)
		case <-ctx.Done():
			slog.Info("Stopping worker", "id", w.id)
			return
		}
	}
}

func (w *Worker) processUpdate(ctx context.Context, update domain.PriceUpdate) {
	// Сохраняем последнюю цену в кэш
	if err := w.cache.SetLatestPrice(ctx, update.Exchange, update.Symbol, update.Price); err != nil {
		slog.Error("Failed to cache latest price", "worker", w.id, "error", err)
	}

	// Сохраняем для агрегации
	if err := w.cache.StoreRecentPrice(ctx, update.Exchange, update.Symbol, update.Price, update.Timestamp); err != nil {
		slog.Error("Failed to store recent price", "worker", w.id, "error", err)
	}
}

func (w *Worker) Input() chan<- domain.PriceUpdate {
	return w.input
}
