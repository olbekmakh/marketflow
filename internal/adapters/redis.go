// internal/adapters/redis.go
package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"marketflow/internal/config"
	"marketflow/internal/domain"
)

type RedisAdapter struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisAdapter(cfg config.RedisConfig) (*RedisAdapter, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisAdapter{
		client: client,
		ttl:    cfg.TTL,
	}, nil
}

func (r *RedisAdapter) SetLatestPrice(ctx context.Context, exchange, symbol string, price float64) error {
	key := fmt.Sprintf("latest:%s:%s", exchange, symbol)
	return r.client.Set(ctx, key, price, r.ttl).Err()
}

func (r *RedisAdapter) GetLatestPrice(ctx context.Context, exchange, symbol string) (float64, error) {
	key := fmt.Sprintf("latest:%s:%s", exchange, symbol)
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(val, 64)
}

func (r *RedisAdapter) StoreRecentPrice(ctx context.Context, exchange, symbol string, price float64, timestamp time.Time) error {
	key := fmt.Sprintf("recent:%s:%s", exchange, symbol)
	
	priceUpdate := domain.PriceUpdate{
		Exchange:  exchange,
		Symbol:    symbol,
		Price:     price,
		Timestamp: timestamp,
	}

	data, err := json.Marshal(priceUpdate)
	if err != nil {
		return err
	}

	score := float64(timestamp.Unix())
	return r.client.ZAdd(ctx, key, redis.Z{Score: score, Member: data}).Err()
}

func (r *RedisAdapter) GetRecentPrices(ctx context.Context, exchange, symbol string, since time.Time) ([]domain.PriceUpdate, error) {
	key := fmt.Sprintf("recent:%s:%s", exchange, symbol)
	
	minScore := strconv.FormatInt(since.Unix(), 10)
	maxScore := "+inf"

	results, err := r.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: minScore,
		Max: maxScore,
	}).Result()
	
	if err != nil {
		return nil, err
	}

	var prices []domain.PriceUpdate
	for _, result := range results {
		var price domain.PriceUpdate
		if err := json.Unmarshal([]byte(result), &price); err == nil {
			prices = append(prices, price)
		}
	}

	return prices, nil
}

func (r *RedisAdapter) CleanExpiredData(ctx context.Context) error {
	cutoff := time.Now().Add(-2 * time.Minute).Unix()
	
	keys, err := r.client.Keys(ctx, "recent:*").Result()
	if err != nil {
		return err
	}

	for _, key := range keys {
		r.client.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(cutoff, 10))
	}

	return nil
}

func (r *RedisAdapter) Close() error {
	return r.client.Close()
}
