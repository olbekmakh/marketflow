package application

import (
	"context"
	"math/rand"
	"time"

	"marketflow/internal/domain"
)

type TestDataGenerator struct {
	symbols []string
	prices  map[string]float64
}

func NewTestDataGenerator() *TestDataGenerator {
	symbols := []string{"BTCUSDT", "DOGEUSDT", "TONUSDT", "SOLUSDT", "ETHUSDT"}
	prices := map[string]float64{
		"BTCUSDT":  50000.0,
		"DOGEUSDT": 0.25,
		"TONUSDT":  5.5,
		"SOLUSDT":  150.0,
		"ETHUSDT":  3000.0,
	}

	return &TestDataGenerator{
		symbols: symbols,
		prices:  prices,
	}
}

func (g *TestDataGenerator) Start(ctx context.Context) <-chan domain.PriceUpdate {
	output := make(chan domain.PriceUpdate, 100)

	go func() {
		defer close(output)
		
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Генерируем случайное обновление цены
				symbol := g.symbols[rand.Intn(len(g.symbols))]
				basePrice := g.prices[symbol]
				
				// Изменение цены на ±2%
				change := (rand.Float64() - 0.5) * 0.04
				newPrice := basePrice * (1 + change)
				g.prices[symbol] = newPrice

				update := domain.PriceUpdate{
					Exchange:  "test",
					Symbol:    symbol,
					Price:     newPrice,
					Timestamp: time.Now(),
				}

				select {
				case output <- update:
				case <-ctx.Done():
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	return output
}