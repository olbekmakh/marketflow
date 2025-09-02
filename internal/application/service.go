package application

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"marketflow/internal/adapters"
	"marketflow/internal/config"
	"marketflow/internal/domain"
)

type Application struct {
	db          domain.PriceRepository
	cache       domain.CacheRepository
	exchanges   []domain.ExchangeClient
	testGen     *TestDataGenerator
	mode        domain.DataMode
	config      *config.Config
	workers     []*Worker
	fanIn       *FanIn
	batcher     *Batcher
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	httpServer  *HTTPServer
	mu          sync.RWMutex
}

func NewApplication(db domain.PriceRepository, cache domain.CacheRepository, cfg *config.Config) *Application {
	ctx, cancel := context.WithCancel(context.Background())

	var exchanges []domain.ExchangeClient
	for _, exchCfg := range cfg.Exchanges {
		exchange := adapters.NewExchangeAdapter(exchCfg.Name, exchCfg.Host, exchCfg.Port)
		exchanges = append(exchanges, exchange)
	}

	app := &Application{
		db:        db,
		cache:     cache,
		exchanges: exchanges,
		testGen:   NewTestDataGenerator(),
		mode:      domain.TestMode,
		config:    cfg,
		ctx:       ctx,
		cancel:    cancel,
		batcher:   NewBatcher(db),
	}

	app.httpServer = NewHTTPServer(cfg.Server.Port, app)
	return app
}

func (a *Application) Start(ctx context.Context) error {
	slog.Info("Starting MarketFlow application", 
		"mode", a.mode,
		"workers_per_exchange", 5,
		"total_exchanges", len(a.config.Exchanges))

	// Запуск батчера
	a.startBatcher()

	// Запуск воркеров - 5 на каждую биржу
	a.startWorkers()

	// Запуск агрегатора данных
	a.startDataAggregator()

	// Запуск очистки кэша
	a.startCacheCleanup()

	// Запуск HTTP сервера
	go func() {
		slog.Info("Starting HTTP server", "addr", fmt.Sprintf(":%d", a.config.Server.Port))
		if err := a.httpServer.Start(); err != nil {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	// Запуск в тестовом режиме по умолчанию
	return a.startTestMode()
}

func (a *Application) startBatcher() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.batcher.Start(a.ctx)
	}()
}

func (a *Application) startWorkers() {
	// 5 воркеров на каждую биржу
	numWorkers := len(a.config.Exchanges) * 5
	for i := 0; i < numWorkers; i++ {
		worker := NewWorker(i, a.cache, a.batcher)
		a.workers = append(a.workers, worker)
		
		a.wg.Add(1)
		go func(w *Worker) {
			defer a.wg.Done()
			w.Start(a.ctx)
		}(worker)
	}

	slog.Info("Started worker pool", 
		"total_workers", len(a.workers),
		"workers_per_exchange", 5)
}

func (a *Application) startLiveMode() error {
	slog.Info("Attempting to start Live Mode")
	symbols := []string{"BTCUSDT", "DOGEUSDT", "TONUSDT", "SOLUSDT", "ETHUSDT"}
	
	var channels []<-chan domain.PriceUpdate
	connectedExchanges := 0

	for _, exchange := range a.exchanges {
		maxRetries := 3
		for retry := 0; retry < maxRetries; retry++ {
			if err := exchange.Connect(a.ctx); err != nil {
				slog.Warn("Failed to connect to exchange", 
					"exchange", fmt.Sprintf("%T", exchange), 
					"error", err,
					"retry", retry+1,
					"max_retries", maxRetries)
				if retry == maxRetries-1 {
					break
				}
				time.Sleep(time.Duration(retry+1) * time.Second)
				continue
			}

			ch, err := exchange.Subscribe(symbols)
			if err != nil {
				slog.Error("Failed to subscribe to exchange", "error", err)
				break
			}

			channels = append(channels, ch)
			connectedExchanges++
			break
		}
	}

	if connectedExchanges == 0 {
		slog.Warn("No exchanges connected, falling back to test mode")
		a.mu.Lock()
		a.mode = domain.TestMode
		a.mu.Unlock()
		return a.startTestMode()
	}

	slog.Info("Live mode started successfully", "connected_exchanges", connectedExchanges)
	
	// Fan-In: объединяем все каналы
	a.fanIn = NewFanIn(channels)
	
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.distributeUpdates()
	}()

	return nil
}

func (a *Application) startTestMode() error {
	slog.Info("Starting Test Mode with synthetic data generator")
	
	ch := a.testGen.Start(a.ctx)
	a.fanIn = NewFanIn([]<-chan domain.PriceUpdate{ch})
	
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.distributeUpdates()
	}()

	return nil
}

func (a *Application) distributeUpdates() {
	workerIdx := 0
	processedCount := 0
	
	for update := range a.fanIn.Output() {
		// Fan-Out: распределяем между воркерами round-robin
		worker := a.workers[workerIdx%len(a.workers)]
		
		select {
		case worker.Input() <- update:
			workerIdx++
			processedCount++
			
			if processedCount%1000 == 0 {
				slog.Info("Processed price updates", 
					"count", processedCount,
					"current_worker", workerIdx%len(a.workers))
			}
		case <-a.ctx.Done():
			slog.Info("Stopping update distribution", "total_processed", processedCount)
			return
		}
	}
}

func (a *Application) startDataAggregator() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		slog.Info("Started data aggregator", "interval", "1m")

		for {
			select {
			case <-ticker.C:
				start := time.Now()
				a.aggregateData()
				duration := time.Since(start)
				
				slog.Info("Data aggregation completed", 
					"duration", duration,
					"timestamp", time.Now().Format(time.RFC3339))
				
			case <-a.ctx.Done():
				slog.Info("Stopping data aggregator")
				return
			}
		}
	}()
}

func (a *Application) aggregateData() {
	symbols := []string{"BTCUSDT", "DOGEUSDT", "TONUSDT", "SOLUSDT", "ETHUSDT"}
	
	a.mu.RLock()
	mode := a.mode
	a.mu.RUnlock()

	if mode == domain.TestMode {
		// В тестовом режиме используем "test" как имя биржи
		for _, symbol := range symbols {
			a.aggregateForPair("test", symbol)
		}
	} else {
		// В режиме live используем настоящие имена бирж
		for _, exchCfg := range a.config.Exchanges {
			for _, symbol := range symbols {
				a.aggregateForPair(exchCfg.Name, symbol)
			}
		}
	}
}

func (a *Application) aggregateForPair(exchange, symbol string) {
	since := time.Now().Add(-1 * time.Minute)
	prices, err := a.cache.GetRecentPrices(a.ctx, exchange, symbol, since)
	if err != nil {
		slog.Debug("No recent prices found for aggregation", 
			"exchange", exchange, 
			"symbol", symbol, 
			"error", err)
		return
	}

	if len(prices) == 0 {
		return
	}

	var sum, min, max float64
	min = prices[0].Price
	max = prices[0].Price

	for _, price := range prices {
		sum += price.Price
		if price.Price < min {
			min = price.Price
		}
		if price.Price > max {
			max = price.Price
		}
	}

	avg := sum / float64(len(prices))

	aggregated := &domain.AggregatedPrice{
		PairName:     symbol,
		Exchange:     exchange,
		Timestamp:    time.Now(),
		AveragePrice: avg,
		MinPrice:     min,
		MaxPrice:     max,
	}

	// Добавляем в батчер вместо прямой записи
	a.batcher.Add(aggregated)

	slog.Debug("Aggregated price data", 
		"exchange", exchange, 
		"symbol", symbol, 
		"avg_price", avg,
		"min_price", min,
		"max_price", max,
		"data_points", len(prices))
}

func (a *Application) startCacheCleanup() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		slog.Info("Started cache cleanup service", "interval", "30s")

		for {
			select {
			case <-ticker.C:
				if err := a.cache.CleanExpiredData(a.ctx); err != nil {
					slog.Error("Failed to clean expired cache data", "error", err)
				} else {
					slog.Debug("Cache cleanup completed")
				}
			case <-a.ctx.Done():
				slog.Info("Stopping cache cleanup service")
				return
			}
		}
	}()
}

func (a *Application) SwitchMode(mode domain.DataMode) error {
	a.mu.Lock()
	oldMode := a.mode
	a.mode = mode
	a.mu.Unlock()
	
	slog.Info("Switching data mode", 
		"from", oldMode, 
		"to", mode,
		"timestamp", time.Now().Format(time.RFC3339))
	
	// Останавливаем текущие источники данных
	if a.fanIn != nil {
		// Отключаем текущие подключения
		for _, exchange := range a.exchanges {
			if exchange.IsConnected() {
				exchange.Disconnect()
			}
		}
	}
	
	// Запускаем новый режим
	if mode == domain.LiveMode {
		return a.startLiveMode()
	} else {
		return a.startTestMode()
	}
}

func (a *Application) GetLatestPrice(exchange, symbol string) (float64, error) {
	a.mu.RLock()
	mode := a.mode
	a.mu.RUnlock()

	// В тестовом режиме используем "test" как имя биржи
	if mode == domain.TestMode {
		exchange = "test"
	}
	
	price, err := a.cache.GetLatestPrice(a.ctx, exchange, symbol)
	if err != nil {
		slog.Debug("Failed to get latest price from cache", 
			"exchange", exchange, 
			"symbol", symbol, 
			"error", err)
		
		// Fallback: пробуем получить из PostgreSQL если Redis недоступен
		if fallbackPrice, fallbackErr := a.getFallbackPrice(exchange, symbol); fallbackErr == nil {
			slog.Info("Used PostgreSQL fallback for latest price", 
				"exchange", exchange, 
				"symbol", symbol)
			return fallbackPrice, nil
		}
	}
	
	return price, err
}

func (a *Application) getFallbackPrice(exchange, symbol string) (float64, error) {
	// Получаем последнюю агрегированную цену из PostgreSQL
	recent, err := a.db.GetAverage(a.ctx, exchange, symbol, 5*time.Minute)
	if err != nil {
		return 0, err
	}
	return recent.AveragePrice, nil
}

// Остальные методы остаются такими же...
func (a *Application) GetHighestPrice(exchange, symbol string, period time.Duration) (*domain.AggregatedPrice, error) {
	a.mu.RLock()
	mode := a.mode
	a.mu.RUnlock()

	if mode == domain.TestMode {
		exchange = "test"
	}
	return a.db.GetHighest(a.ctx, exchange, symbol, period)
}

func (a *Application) GetLowestPrice(exchange, symbol string, period time.Duration) (*domain.AggregatedPrice, error) {
	a.mu.RLock()
	mode := a.mode
	a.mu.RUnlock()

	if mode == domain.TestMode {
		exchange = "test"
	}
	return a.db.GetLowest(a.ctx, exchange, symbol, period)
}

func (a *Application) GetAveragePrice(exchange, symbol string, period time.Duration) (*domain.AggregatedPrice, error) {
	a.mu.RLock()
	mode := a.mode
	a.mu.RUnlock()

	if mode == domain.TestMode {
		exchange = "test"
	}
	return a.db.GetAverage(a.ctx, exchange, symbol, period)
}

func (a *Application) GetHealth() *domain.HealthStatus {
	connections := make(map[string]string)
	
	a.mu.RLock()
	mode := a.mode
	a.mu.RUnlock()

	if mode == domain.TestMode {
		connections["test_generator"] = "active"
	} else {
		for i, exchange := range a.exchanges {
			name := a.config.Exchanges[i].Name
			if exchange.IsConnected() {
				connections[name] = "connected"
			} else {
				connections[name] = "disconnected"
			}
		}
	}

	// Проверяем Redis
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	if err := a.cache.SetLatestPrice(ctx, "health", "check", 1.0); err != nil {
		connections["redis"] = "disconnected"
	} else {
		connections["redis"] = "connected"
	}

	// Проверяем PostgreSQL
	if err := a.db.Store(ctx, &domain.AggregatedPrice{
		PairName: "HEALTH", Exchange: "check", Timestamp: time.Now(),
		AveragePrice: 1.0, MinPrice: 1.0, MaxPrice: 1.0,
	}); err != nil {
		connections["postgresql"] = "disconnected"
	} else {
		connections["postgresql"] = "connected"
	}

	status := "healthy"
	if mode == domain.TestMode {
		status = "healthy (test mode)"
	}

	// Проверяем, есть ли отключенные сервисы
	for _, conn := range connections {
		if conn == "disconnected" {
			status = "degraded"
			break
		}
	}

	return &domain.HealthStatus{
		Status:      status,
		Connections: connections,
		Timestamp:   time.Now(),
	}
}

func (a *Application) Shutdown(ctx context.Context) error {
	slog.Info("Initiating graceful shutdown")
	
	// Отменяем контекст приложения
	a.cancel()
	
	// Останавливаем HTTP сервер
	if err := a.httpServer.Shutdown(ctx); err != nil {
		slog.Error("Error shutting down HTTP server", "error", err)
	}
	
	// Отключение от бирж
	for i, exchange := range a.exchanges {
		if exchange.IsConnected() {
			if err := exchange.Disconnect(); err != nil {
				slog.Error("Error disconnecting from exchange", 
					"exchange", a.config.Exchanges[i].Name, 
					"error", err)
			}
		}
	}
	
	// Ожидание завершения всех горутин
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("All goroutines stopped successfully")
		return nil
	case <-ctx.Done():
		slog.Warn("Shutdown timeout exceeded, some goroutines may not have stopped gracefully")
		return ctx.Err()
	}
}
