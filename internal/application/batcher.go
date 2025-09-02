package application

import (
	"context"
	"log/slog"
	"time"

	"marketflow/internal/domain"
)

type Batcher struct {
	db          domain.PriceRepository
	batchCh     chan *domain.AggregatedPrice
	batchSize   int
	flushPeriod time.Duration
}

func NewBatcher(db domain.PriceRepository) *Batcher {
	return &Batcher{
		db:          db,
		batchCh:     make(chan *domain.AggregatedPrice, 1000),
		batchSize:   100,
		flushPeriod: 10 * time.Second,
	}
}

func (b *Batcher) Add(price *domain.AggregatedPrice) {
	select {
	case b.batchCh <- price:
	default:
		slog.Warn("Batcher channel full, dropping price update")
	}
}

func (b *Batcher) Start(ctx context.Context) {
	batch := make([]*domain.AggregatedPrice, 0, b.batchSize)
	ticker := time.NewTicker(b.flushPeriod)
	defer ticker.Stop()

	slog.Info("Started batcher", 
		"batch_size", b.batchSize, 
		"flush_period", b.flushPeriod)

	for {
		select {
		case price := <-b.batchCh:
			batch = append(batch, price)
			
			if len(batch) >= b.batchSize {
				b.flush(ctx, batch)
				batch = batch[:0] // Clear batch
			}
			
		case <-ticker.C:
			if len(batch) > 0 {
				b.flush(ctx, batch)
				batch = batch[:0] // Clear batch
			}
			
		case <-ctx.Done():
			// Flush remaining items
			if len(batch) > 0 {
				b.flush(ctx, batch)
			}
			slog.Info("Batcher stopped")
			return
		}
	}
}

func (b *Batcher) flush(ctx context.Context, batch []*domain.AggregatedPrice) {
	start := time.Now()
	
	for _, price := range batch {
		if err := b.db.Store(ctx, price); err != nil {
			slog.Error("Failed to store batch item", 
				"exchange", price.Exchange,
				"symbol", price.PairName,
				"error", err)
		}
	}
	
	duration := time.Since(start)
	slog.Info("Batch flushed to database", 
		"count", len(batch),
		"duration", duration)
}