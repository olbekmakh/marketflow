package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"marketflow/internal/adapters"
	"marketflow/internal/application"
	"marketflow/internal/config"
)

func main() {
	var (
		port = flag.Int("port", 8080, "Port number")
		help = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		fmt.Println("Usage:")
		fmt.Println("  marketflow [--port <N>]")
		fmt.Println("  marketflow --help")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --port N     Port number")
		return
	}

	// Валидация порта
	if *port <= 0 || *port > 65535 {
		fmt.Fprintf(os.Stderr, "Error: Invalid port number %d. Port must be between 1 and 65535\n", *port)
		os.Exit(1)
	}

	// Загрузка конфигурации
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Переопределяем порт из флага
	cfg.Server.Port = *port

	// Инициализация логгера с контекстной информацией
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		AddSource: true,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Инициализация адаптеров с обработкой ошибок
	dbAdapter, err := adapters.NewPostgresAdapter(cfg.Database)
	if err != nil {
		slog.Error("Failed to connect to database", 
			"error", err, 
			"host", cfg.Database.Host, 
			"port", cfg.Database.Port)
		os.Exit(1)
	}
	defer func() {
		if err := dbAdapter.Close(); err != nil {
			slog.Error("Failed to close database connection", "error", err)
		}
	}()

	cacheAdapter, err := adapters.NewRedisAdapter(cfg.Redis)
	if err != nil {
		slog.Error("Failed to connect to Redis", 
			"error", err, 
			"host", cfg.Redis.Host, 
			"port", cfg.Redis.Port)
		os.Exit(1)
	}
	defer func() {
		if err := cacheAdapter.Close(); err != nil {
			slog.Error("Failed to close Redis connection", "error", err)
		}
	}()

	// Инициализация приложения
	app := application.NewApplication(dbAdapter, cacheAdapter, cfg)

	// Запуск системы
	if err := app.Start(ctx); err != nil {
		slog.Error("Failed to start application", "error", err)
		os.Exit(1)
	}

	slog.Info("MarketFlow started successfully", 
		"port", *port, 
		"pid", os.Getpid(),
		"timestamp", time.Now().Format(time.RFC3339))

	// Ожидание сигнала завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan

	slog.Info("Received shutdown signal", "signal", sig.String())
	
	// Graceful shutdown с таймаутом
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := app.Shutdown(shutdownCtx); err != nil {
		slog.Error("Error during shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("MarketFlow shut down successfully")
}