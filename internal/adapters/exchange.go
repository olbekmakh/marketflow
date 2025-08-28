// internal/adapters/exchange.go
package adapters

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"marketflow/internal/domain"
)

type ExchangeAdapter struct {
	name string
	host string
	port int
	conn net.Conn
	ch   chan domain.PriceUpdate
}

func NewExchangeAdapter(name, host string, port int) *ExchangeAdapter {
	return &ExchangeAdapter{
		name: name,
		host: host,
		port: port,
		ch:   make(chan domain.PriceUpdate, 100),
	}
}

func (e *ExchangeAdapter) Connect(ctx context.Context) error {
	address := fmt.Sprintf("%s:%d", e.host, e.port)
	
	var err error
	for retries := 0; retries < 5; retries++ {
		e.conn, err = net.DialTimeout("tcp", address, 5*time.Second)
		if err == nil {
			slog.Info("Connected to exchange", "exchange", e.name, "address", address)
			return nil
		}
		
		slog.Warn("Failed to connect to exchange, retrying...", 
			"exchange", e.name, "error", err, "retry", retries+1)
		time.Sleep(time.Duration(retries+1) * time.Second)
	}
	
	return fmt.Errorf("failed to connect to exchange %s: %w", e.name, err)
}

func (e *ExchangeAdapter) Disconnect() error {
	if e.conn != nil {
		return e.conn.Close()
	}
	return nil
}

func (e *ExchangeAdapter) Subscribe(symbols []string) (<-chan domain.PriceUpdate, error) {
	if e.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	go e.readData()
	return e.ch, nil
}

func (e *ExchangeAdapter) readData() {
	defer close(e.ch)
	
	scanner := bufio.NewScanner(e.conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var update domain.PriceUpdate
		if err := json.Unmarshal([]byte(line), &update); err != nil {
			slog.Warn("Failed to parse price update", "exchange", e.name, "error", err)
			continue
		}

		update.Exchange = e.name
		update.Timestamp = time.Now()

		select {
		case e.ch <- update:
		default:
			slog.Warn("Channel full, dropping price update", "exchange", e.name)
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("Error reading from exchange", "exchange", e.name, "error", err)
	}
}

func (e *ExchangeAdapter) IsConnected() bool {
	return e.conn != nil
}