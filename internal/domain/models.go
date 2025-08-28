package domain

import (
	"time"
)

type PriceUpdate struct {
	Exchange  string    `json:"exchange"`
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

type AggregatedPrice struct {
	PairName     string    `json:"pair_name"`
	Exchange     string    `json:"exchange"`
	Timestamp    time.Time `json:"timestamp"`
	AveragePrice float64   `json:"average_price"`
	MinPrice     float64   `json:"min_price"`
	MaxPrice     float64   `json:"max_price"`
}

type HealthStatus struct {
	Status      string            `json:"status"`
	Connections map[string]string `json:"connections"`
	Timestamp   time.Time         `json:"timestamp"`
}

type DataMode string

const (
	LiveMode DataMode = "live"
	TestMode DataMode = "test"
)
