package domain

import (
	"context"
	"time"
)

type PriceRepository interface {
	Store(ctx context.Context, price *AggregatedPrice) error
	GetHighest(ctx context.Context, exchange, symbol string, period time.Duration) (*AggregatedPrice, error)
	GetLowest(ctx context.Context, exchange, symbol string, period time.Duration) (*AggregatedPrice, error)
	GetAverage(ctx context.Context, exchange, symbol string, period time.Duration) (*AggregatedPrice, error)
}

type CacheRepository interface {
	SetLatestPrice(ctx context.Context, exchange, symbol string, price float64) error
	GetLatestPrice(ctx context.Context, exchange, symbol string) (float64, error)
	StoreRecentPrice(ctx context.Context, exchange, symbol string, price float64, timestamp time.Time) error
	GetRecentPrices(ctx context.Context, exchange, symbol string, since time.Time) ([]PriceUpdate, error)
	CleanExpiredData(ctx context.Context) error
}

type ExchangeClient interface {
	Connect(ctx context.Context) error
	Disconnect() error
	Subscribe(symbols []string) (<-chan PriceUpdate, error)
	IsConnected() bool
}
