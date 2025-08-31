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
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	httpServer  *HTTPServer
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
		mode:      domain.TestMode, // Начинаем в тестовом режиме
		config:    cfg,
		ctx:       ctx,
		cancel:    cancel,
	}

	app.httpServer = NewHTTPServer(cfg.Server.Port, app)
	return app
}

func (a *Application) Start(ctx context.Context) error {
	slog.Info("Starting MarketFlow application in test mode")

	// Запуск воркеров
	a.startWorkers()

	// Запуск агрегатора данных
	a.startDataAggregator()

	// Запуск очистки кэша
	a.startCacheCleanup()

	// Запуск HTTP сервера
	go func() {
		if err := a.httpServer.Start(); err != nil {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	// Автоматически запускаем в тестовом режиме
	return a.startTestMode()
}

func (a *Application) startWorkers() {
	// Создаем воркеров - по 5 на каждую биржу
	numWorkers := 15 // 5 воркеров на 3 биржи
	for i := 0; i < numWorkers; i++ {
		worker := NewWorker(i, a.db, a.cache)
		a.workers = append(a.workers, worker)
		
		a.wg.Add(1)
		go func(w *Worker) {
			defer a.wg.Done()
			w.Start(a.ctx)
		}(worker)
	}

	slog.Info("Started workers", "count", len(a.workers))
}

func (a *Application) startDataCollection() error {
	if a.mode == domain.LiveMode {
		return a.startLiveMode()
	} else {
		return a.startTestMode()
	}
}

func (a *Application) startLiveMode() error {
	slog.Info("Attempting to start Live Mode")
	symbols := []string{"BTCUSDT", "DOGEUSDT", "TONUSDT", "SOLUSDT", "ETHUSDT"}
	
	var channels []<-chan domain.PriceUpdate
	connectedExchanges := 0

	for _, exchange := range a.exchanges {
		if err := exchange.Connect(a.ctx); err != nil {
			slog.Warn("Failed to connect to exchange", "exchange", fmt.Sprintf("%T", exchange), "error", err)
			continue
		}

		ch, err := exchange.Subscribe(symbols)
		if err != nil {
			slog.Error("Failed to subscribe to exchange", "error", err)
			continue
		}Code 

	if connectedExchanges == 0 {
		slog.Warn("No exchanges connected, falling back to test mode")
		a.mode = domain.TestMode
		return a.startTestMode()
	}

	slog.Info("Live mode started", "connected_exchanges", connectedExchanges)
	
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
	slog.Info("Starting Test Mode with synthetic data")
	
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
	
	for update := range a.fanIn.Output() {
		// Fan-Out: распределяем между воркерами
		worker := a.workers[workerIdx%len(a.workers)]
		
		select {
		case worker.Input() <- update:
			workerIdx++
		case <-a.ctx.Done():
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

		for {
			select {
			case <-ticker.C:
				a.aggregateData()
			case <-a.ctx.Done():
				return
			}
		}
	}()
}

func (a *Application) aggregateData() {
	symbols := []string{"BTCUSDT", "DOGEUSDT", "TONUSDT", "SOLUSDT", "ETHUSDT"}
	
	if a.mode == domain.TestMode {
		// В тестовом режиме используем "test" как имя биржи
		a.aggregateForPair("test", "BTCUSDT")
		a.aggregateForPair("test", "ETHUSDT") 
		a.aggregateForPair("test", "DOGEUSDT")
		a.aggregateForPair("test", "TONUSDT")
		a.aggregateForPair("test", "SOLUSDT")
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
	if err != nil || len(prices) == 0 {
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

	if err := a.db.Store(a.ctx, aggregated); err != nil {
		slog.Error("Failed to store aggregated data", "error", err)
	} else {
		slog.Info("Stored aggregated data", "exchange", exchange, "symbol", symbol, "avg_price", avg)
	}
}

func (a *Application) startCacheCleanup() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := a.cache.CleanExpiredData(a.ctx); err != nil {
					slog.Error("Failed to clean cache", "error", err)
				}
			case <-a.ctx.Done():
				return
			}
		}
	}()
}

func (a *Application) SwitchMode(mode domain.DataMode) error {
	oldMode := a.mode
	a.mode = mode
	
	slog.Info("Switching mode", "from", oldMode, "to", mode)
	
	if mode == domain.LiveMode {
		// Попробуем подключиться к биржам
		return a.startLiveMode()
	} else {
		// Переключаемся на тестовый режим
		return a.startTestMode() 
	}
}

func (a *Application) GetLatestPrice(exchange, symbol string) (float64, error) {
	// В тестовом режиме используем "test" как имя биржи
	if a.mode == domain.TestMode {
		exchange = "test"
	}
	return a.cache.GetLatestPrice(a.ctx, exchange, symbol)
}

func (a *Application) GetHighestPrice(exchange, symbol string, period time.Duration) (*domain.AggregatedPrice, error) {
	if a.mode == domain.TestMode {
		exchange = "test"
	}
	return a.db.GetHighest(a.ctx, exchange, symbol, period)
}

func (a *Application) GetLowestPrice(exchange, symbol string, period time.Duration) (*domain.AggregatedPrice, error) {
	if a.mode == domain.TestMode {
		exchange = "test" 
	}
	return a.db.GetLowest(a.ctx, exchange, symbol, period)
}

func (a *Application) GetAveragePrice(exchange, symbol string, period time.Duration) (*domain.AggregatedPrice, error) {
	if a.mode == domain.TestMode {
		exchange = "test"
	}
	return a.db.GetAverage(a.ctx, exchange, symbol, period)
}

func (a *Application) GetHealth() *domain.HealthStatus {
	connections := make(map[string]string)
	
	if a.mode == domain.TestMode {
		connections["test_generator"] = "active"
	} else {
		for i, exchange := range a.exchanges {
			name := fmt.Sprintf("exchange%d", i+1)
			if exchange.IsConnected() {
				connections[name] = "connected"
			} else {
				connections[name] = "disconnected" 
			}
		}
	}

	// Проверяем Redis
	_, err := a.cache.GetLatestPrice(a.ctx, "test", "BTCUSDT")
	if err != nil {
		connections["redis"] = "disconnected"
	} else {
		connections["redis"] = "connected"
	}

	status := "healthy"
	if a.mode == domain.TestMode {
		status = "healthy (test mode)"
	}

	return &domain.HealthStatus{
		Status:      status,
		Connections: connections,
		Timestamp:   time.Now(),
	}
}

func (a *Application) Shutdown(ctx context.Context) {
	slog.Info("Shutting down application")
	
	a.cancel()
	
	// Отключение от бирж
	for _, exchange := range a.exchanges {
		exchange.Disconnect()
	}
	
	// Остановка HTTP сервера
	a.httpServer.Shutdown(ctx)
	
	// Ожидание завершения горутин
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("Application stopped gracefully")
	case <-time.After(10 * time.Second):
		slog.Warn("Forced shutdown after timeout")
	}
}