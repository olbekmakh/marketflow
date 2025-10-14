package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	currentMode = "test"
	prices      = map[string]float64{
		"BTCUSDT":  50000.0,
		"ETHUSDT":  3000.0,
		"DOGEUSDT": 0.25,
		"TONUSDT":  5.5,
		"SOLUSDT":  150.0,
	}
	server *http.Server
	
	// Регулярное выражение для валидации периода
	periodRegex = regexp.MustCompile(`^([1-9][0-9]*)(s|m|h)$`)
)

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

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

	if *port <= 0 || *port > 65535 {
		fmt.Fprintf(os.Stderr, "Error: Invalid port %d\n", *port)
		os.Exit(1)
	}

	go startPriceGenerator()

	http.HandleFunc("/health", corsWrapper(healthHandler))
	http.HandleFunc("/mode/test", corsWrapper(modeHandler))
	http.HandleFunc("/mode/live", corsWrapper(modeHandler))
	http.HandleFunc("/prices/", corsWrapper(pricesHandler))

	server = &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println(" Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf(" Server shutdown error: %v", err)
		}
		log.Println(" Server stopped")
		os.Exit(0)
	}()

	log.Printf(" MarketFlow started on http://localhost:%d", *port)
	log.Printf(" Mode: %s", currentMode)
	log.Printf("  Symbols: %s", strings.Join(getSymbols(), ", "))
	log.Printf("  Endpoints:")
	log.Printf("   GET  /health")
	log.Printf("   POST /mode/test")
	log.Printf("   POST /mode/live")
	log.Printf("   GET  /prices/latest/{symbol}")
	log.Printf("   GET  /prices/latest/{exchange}/{symbol}")
	log.Printf("   GET  /prices/highest/{symbol}[?period=1m]")
	log.Printf("   GET  /prices/highest/{exchange}/{symbol}[?period=1m]")
	log.Printf("   GET  /prices/lowest/{symbol}[?period=1m]")
	log.Printf("   GET  /prices/lowest/{exchange}/{symbol}[?period=1m]")
	log.Printf("   GET  /prices/average/{symbol}[?period=1m]")
	log.Printf("   GET  /prices/average/{exchange}/{symbol}[?period=1m]")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf(" Server error: %v", err)
	}
}

func startPriceGenerator() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	log.Println(" Price generator started")

	for {
		select {
		case <-ticker.C:
			for symbol := range prices {
				change := (rand.Float64() - 0.5) * 0.02
				newPrice := prices[symbol] * (1 + change)
				if newPrice > 0 {
					prices[symbol] = newPrice
				}
			}
		}
	}
}

func corsWrapper(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		start := time.Now()
		handler(w, r)
		log.Printf(" %s %s - %v", r.Method, r.URL.Path, time.Since(start))
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"status": fmt.Sprintf("healthy (%s mode)", currentMode),
			"connections": map[string]string{
				"test_generator": "active",
				"database":       "simulated",
				"cache":          "simulated",
			},
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	writeJSON(w, http.StatusOK, response)
}

func modeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed. Use POST method.")
		return
	}

	var newMode string
	if strings.Contains(r.URL.Path, "/mode/test") {
		newMode = "test"
	} else if strings.Contains(r.URL.Path, "/mode/live") {
		newMode = "live"
	} else {
		writeError(w, http.StatusNotFound, "Invalid mode endpoint. Use /mode/test or /mode/live")
		return
	}

	currentMode = newMode
	log.Printf(" Mode switched to: %s", newMode)

	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"mode":      newMode,
			"status":    "switched",
			"message":   fmt.Sprintf("Successfully switched to %s mode", newMode),
			"timestamp": time.Now().Format(time.RFC3339),
		},
	}

	writeJSON(w, http.StatusOK, response)
}

func pricesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed. Use GET method.")
		return
	}

	// Парсим URL
	path := strings.TrimPrefix(r.URL.Path, "/prices/")
	parts := strings.Split(path, "/")

	var priceType, exchange, symbol string

	if len(parts) == 2 {
		priceType = parts[0]
		symbol = strings.ToUpper(parts[1])
		exchange = ""
	} else if len(parts) == 3 {
		priceType = parts[0]
		exchange = parts[1]
		symbol = strings.ToUpper(parts[2])
	} else {
		writeError(w, http.StatusBadRequest,
			"Invalid URL format. Expected: /prices/{type}/{symbol} or /prices/{type}/{exchange}/{symbol}")
		return
	}

	// Валидация символа
	if !isValidSymbol(symbol) {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("Invalid symbol: %s. Valid symbols: BTCUSDT, ETHUSDT, DOGEUSDT, TONUSDT, SOLUSDT", symbol))
		return
	}

	// Валидация биржи (если указана)
	if exchange != "" && !isValidExchange(exchange) {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("Invalid exchange: %s. Valid exchanges: exchange1, exchange2, exchange3, test", exchange))
		return
	}

	// Получаем текущую цену
	basePrice, exists := prices[symbol]
	if !exists {
		writeError(w, http.StatusNotFound,
			fmt.Sprintf("Price not found for symbol: %s", symbol))
		return
	}

	// Парсим и валидируем период
	periodStr := r.URL.Query().Get("period")
	if periodStr == "" {
		periodStr = "5m" // Значение по умолчанию
	}

	// Валидация периода
	if err := validatePeriod(periodStr); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Маршрутизация по типу цены
	switch priceType {
	case "latest":
		handleLatestPrice(w, symbol, exchange, basePrice)
	case "highest":
		handleHighestPrice(w, symbol, exchange, basePrice, periodStr)
	case "lowest":
		handleLowestPrice(w, symbol, exchange, basePrice, periodStr)
	case "average":
		handleAveragePrice(w, symbol, exchange, basePrice, periodStr)
	default:
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("Invalid price type: %s. Valid types: latest, highest, lowest, average", priceType))
	}
}

// validatePeriod проверяет корректность формата периода
func validatePeriod(period string) error {
	// Проверка с помощью регулярного выражения
	if !periodRegex.MatchString(period) {
		return fmt.Errorf("Invalid period format: '%s'. Expected format: positive number + unit (s/m). Examples: 1s, 5m. Invalid: 0s, -1m, 5, 10x", period)
	}

	// Дополнительная проверка: извлекаем число и проверяем что оно положительное
	matches := periodRegex.FindStringSubmatch(period)
	if len(matches) != 3 {
		return fmt.Errorf("Invalid period format: '%s'", period)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return fmt.Errorf("Invalid period value: '%s'", period)
	}

	if value <= 0 {
		return fmt.Errorf("Period must be a positive number, got: %d", value)
	}

	// Проверка разумных ограничений (опционально)
	unit := matches[2]
	maxValues := map[string]int{
		"s": 3600,  // Максимум 3600 секунд (1 час)
		"m": 1440,  // Максимум 1440 минут (24 часа)
	}

	if max, ok := maxValues[unit]; ok && value > max {
		return fmt.Errorf("Period too large: %s. Maximum allowed: %d%s", period, max, unit)
	}

	return nil
}

func handleLatestPrice(w http.ResponseWriter, symbol, exchange string, price float64) {
	if exchange == "" {
		exchange = getExchangeName()
	}

	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"symbol":    symbol,
			"price":     price,
			"exchange":  exchange,
			"timestamp": time.Now().Format(time.RFC3339),
		},
	}
	writeJSON(w, http.StatusOK, response)
}

func handleHighestPrice(w http.ResponseWriter, symbol, exchange string, basePrice float64, period string) {
	if exchange == "" {
		exchange = getExchangeName()
	}

	maxPrice := basePrice * 1.05

	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"pair_name":     symbol,
			"exchange":      exchange,
			"timestamp":     time.Now().Format(time.RFC3339),
			"period":        period,
			"average_price": basePrice,
			"min_price":     basePrice * 0.95,
			"max_price":     maxPrice,
		},
	}
	writeJSON(w, http.StatusOK, response)
}

func handleLowestPrice(w http.ResponseWriter, symbol, exchange string, basePrice float64, period string) {
	if exchange == "" {
		exchange = getExchangeName()
	}

	minPrice := basePrice * 0.95

	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"pair_name":     symbol,
			"exchange":      exchange,
			"timestamp":     time.Now().Format(time.RFC3339),
			"period":        period,
			"average_price": basePrice,
			"min_price":     minPrice,
			"max_price":     basePrice * 1.05,
		},
	}
	writeJSON(w, http.StatusOK, response)
}

func handleAveragePrice(w http.ResponseWriter, symbol, exchange string, basePrice float64, period string) {
	if exchange == "" {
		exchange = getExchangeName()
	}

	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"pair_name":     symbol,
			"exchange":      exchange,
			"timestamp":     time.Now().Format(time.RFC3339),
			"period":        period,
			"average_price": basePrice,
			"min_price":     basePrice * 0.95,
			"max_price":     basePrice * 1.05,
		},
	}
	writeJSON(w, http.StatusOK, response)
}

func getExchangeName() string {
	if currentMode == "test" {
		return "test"
	}
	return "live"
}

func isValidSymbol(symbol string) bool {
	validSymbols := []string{"BTCUSDT", "ETHUSDT", "DOGEUSDT", "TONUSDT", "SOLUSDT"}
	for _, valid := range validSymbols {
		if symbol == valid {
			return true
		}
	}
	return false
}

func isValidExchange(exchange string) bool {
	validExchanges := []string{"exchange1", "exchange2", "exchange3", "test", "live"}
	for _, valid := range validExchanges {
		if exchange == valid {
			return true
		}
	}
	return false
}

func getSymbols() []string {
	symbols := make([]string, 0, len(prices))
	for symbol := range prices {
		symbols = append(symbols, symbol)
	}
	return symbols
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf(" JSON encoding error: %v", err)
	}
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	response := APIResponse{
		Success: false,
		Error:   message,
	}
	writeJSON(w, statusCode, response)
}