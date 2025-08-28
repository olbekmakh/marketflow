package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"marketflow/internal/adapters"
	"marketflow/internal/application"
	"marketflow/internal/config"
	"os"
	"os/signal"
	"syscall"
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

	// Загрузка конфигурации
	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Переопределяем порт из флага
	cfg.Server.Port = *port

	// Инициализация логгера
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Инициализация адаптеров
	dbAdapter, err := adapters.NewPostgresAdapter(cfg.Database)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer dbAdapter.Close()

	cacheAdapter, err := adapters.NewRedisAdapter(cfg.Redis)
	if err != nil {
		slog.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer cacheAdapter.Close()

	// Инициализация приложения
	app := application.NewApplication(dbAdapter, cacheAdapter, cfg)

	// Запуск системы
	if err := app.Start(ctx); err != nil {
		slog.Error("Failed to start application", "error", err)
		os.Exit(1)
	}

	// Ожидание сигнала завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	slog.Info("Shutting down...")
	app.Shutdown(ctx)
}
