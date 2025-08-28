// internal/adapters/postgres.go
package adapters

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"marketflow/internal/config"
	"marketflow/internal/domain"
)

type PostgresAdapter struct {
	db *sql.DB
}

func NewPostgresAdapter(cfg config.DatabaseConfig) (*PostgresAdapter, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	adapter := &PostgresAdapter{db: db}
	if err := adapter.createTables(); err != nil {
		return nil, err
	}

	return adapter, nil
}

func (p *PostgresAdapter) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS market_data (
		id SERIAL PRIMARY KEY,
		pair_name VARCHAR(20) NOT NULL,
		exchange VARCHAR(50) NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		average_price DECIMAL(20,8) NOT NULL,
		min_price DECIMAL(20,8) NOT NULL,
		max_price DECIMAL(20,8) NOT NULL,
		INDEX(pair_name, exchange, timestamp)
	)`

	_, err := p.db.Exec(query)
	return err
}

func (p *PostgresAdapter) Store(ctx context.Context, price *domain.AggregatedPrice) error {
	query := `
	INSERT INTO market_data (pair_name, exchange, timestamp, average_price, min_price, max_price)
	VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := p.db.ExecContext(ctx, query,
		price.PairName, price.Exchange, price.Timestamp,
		price.AveragePrice, price.MinPrice, price.MaxPrice)
	return err
}

func (p *PostgresAdapter) GetHighest(ctx context.Context, exchange, symbol string, period time.Duration) (*domain.AggregatedPrice, error) {
	query := `
	SELECT pair_name, exchange, timestamp, average_price, min_price, max_price
	FROM market_data
	WHERE pair_name = $1 AND exchange = $2 AND timestamp >= $3
	ORDER BY max_price DESC
	LIMIT 1`

	since := time.Now().Add(-period)
	row := p.db.QueryRowContext(ctx, query, symbol, exchange, since)

	var price domain.AggregatedPrice
	err := row.Scan(&price.PairName, &price.Exchange, &price.Timestamp,
		&price.AveragePrice, &price.MinPrice, &price.MaxPrice)
	if err != nil {
		return nil, err
	}

	return &price, nil
}

func (p *PostgresAdapter) GetLowest(ctx context.Context, exchange, symbol string, period time.Duration) (*domain.AggregatedPrice, error) {
	query := `
	SELECT pair_name, exchange, timestamp, average_price, min_price, max_price
	FROM market_data
	WHERE pair_name = $1 AND exchange = $2 AND timestamp >= $3
	ORDER BY min_price ASC
	LIMIT 1`

	since := time.Now().Add(-period)
	row := p.db.QueryRowContext(ctx, query, symbol, exchange, since)

	var price domain.AggregatedPrice
	err := row.Scan(&price.PairName, &price.Exchange, &price.Timestamp,
		&price.AveragePrice, &price.MinPrice, &price.MaxPrice)
	if err != nil {
		return nil, err
	}

	return &price, nil
}

func (p *PostgresAdapter) GetAverage(ctx context.Context, exchange, symbol string, period time.Duration) (*domain.AggregatedPrice, error) {
	query := `
	SELECT AVG(average_price) as avg_price
	FROM market_data
	WHERE pair_name = $1 AND exchange = $2 AND timestamp >= $3`

	since := time.Now().Add(-period)
	row := p.db.QueryRowContext(ctx, query, symbol, exchange, since)

	var avgPrice float64
	if err := row.Scan(&avgPrice); err != nil {
		return nil, err
	}

	return &domain.AggregatedPrice{
		PairName:     symbol,
		Exchange:     exchange,
		Timestamp:    time.Now(),
		AveragePrice: avgPrice,
	}, nil
}

func (p *PostgresAdapter) Close() error {
	return p.db.Close()
}