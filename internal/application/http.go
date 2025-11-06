package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"marketflow/internal/domain"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type HTTPServer struct {
	server *http.Server
	app    *Application
}

func NewHTTPServer(port int, app *Application) *HTTPServer {
	mux := http.NewServeMux()

	httpServer := &HTTPServer{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
		app: app,
	}

	httpServer.setupRoutes(mux)
	return httpServer
}

func (h *HTTPServer) setupRoutes(mux *http.ServeMux) {
	// Health endpoint
	mux.HandleFunc("/health", h.getHealth)

	// Mode endpoints
	mux.HandleFunc("/mode/test", h.switchToTestMode)
	mux.HandleFunc("/mode/live", h.switchToLiveMode)

	// Price endpoints - все обрабатываются одной функцией
	mux.HandleFunc("/prices/", h.handlePrices)
}

// handlePrices - единый обработчик для всех /prices/* endpoints
func (h *HTTPServer) handlePrices(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Парсим путь: /prices/{type}/{exchange}/{symbol} или /prices/{type}/{symbol}
	path := strings.TrimPrefix(r.URL.Path, "/prices/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		h.writeError(w, http.StatusBadRequest, "Invalid URL format")
		return
	}

	priceType := parts[0] // latest, highest, lowest, average
	var exchange, symbol string

	if len(parts) == 2 {
		// Формат: /prices/{type}/{symbol}
		symbol = strings.ToUpper(parts[1])
		exchange = "" // Будет выбрана первая доступная биржа
	} else if len(parts) == 3 {
		// Формат: /prices/{type}/{exchange}/{symbol}
		exchange = parts[1]
		symbol = strings.ToUpper(parts[2])
	} else {
		h.writeError(w, http.StatusBadRequest, "Invalid URL format")
		return
	}

	// Парсим период для historical данных
	period := h.parsePeriod(r.URL.Query().Get("period"))

	// Маршрутизация по типу цены
	switch priceType {
	case "latest":
		if exchange == "" {
			h.getLatestPrice(w, symbol)
		} else {
			h.getLatestPriceByExchange(w, exchange, symbol)
		}
	case "highest":
		if exchange == "" {
			h.getHighestPrice(w, symbol, period)
		} else {
			h.getHighestPriceByExchange(w, exchange, symbol, period)
		}
	case "lowest":
		if exchange == "" {
			h.getLowestPrice(w, symbol, period)
		} else {
			h.getLowestPriceByExchange(w, exchange, symbol, period)
		}
	case "average":
		if exchange == "" {
			h.getAveragePrice(w, symbol, period)
		} else {
			h.getAveragePriceByExchange(w, exchange, symbol, period)
		}
	default:
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid price type: %s", priceType))
	}
}

// Latest price handlers
func (h *HTTPServer) getLatestPrice(w http.ResponseWriter, symbol string) {
	// Пробуем получить от любой биржи
	for _, exchCfg := range h.app.config.Exchanges {
		price, err := h.app.GetLatestPrice(exchCfg.Name, symbol)
		if err == nil {
			response := map[string]interface{}{
				"symbol":   symbol,
				"price":    price,
				"exchange": exchCfg.Name,
			}
			h.writeJSON(w, http.StatusOK, response)
			return
		}
	}
	h.writeError(w, http.StatusNotFound, "Price not found")
}

func (h *HTTPServer) getLatestPriceByExchange(w http.ResponseWriter, exchange, symbol string) {
	price, err := h.app.GetLatestPrice(exchange, symbol)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "Price not found")
		return
	}

	response := map[string]interface{}{
		"symbol":   symbol,
		"exchange": exchange,
		"price":    price,
	}
	h.writeJSON(w, http.StatusOK, response)
}

// Highest price handlers
func (h *HTTPServer) getHighestPrice(w http.ResponseWriter, symbol string, period time.Duration) {
	for _, exchCfg := range h.app.config.Exchanges {
		price, err := h.app.GetHighestPrice(exchCfg.Name, symbol, period)
		if err == nil {
			h.writeJSON(w, http.StatusOK, price)
			return
		}
	}
	h.writeError(w, http.StatusNotFound, "Price not found")
}

func (h *HTTPServer) getHighestPriceByExchange(w http.ResponseWriter, exchange, symbol string, period time.Duration) {
	price, err := h.app.GetHighestPrice(exchange, symbol, period)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "Price not found")
		return
	}
	h.writeJSON(w, http.StatusOK, price)
}

// Lowest price handlers
func (h *HTTPServer) getLowestPrice(w http.ResponseWriter, symbol string, period time.Duration) {
	for _, exchCfg := range h.app.config.Exchanges {
		price, err := h.app.GetLowestPrice(exchCfg.Name, symbol, period)
		if err == nil {
			h.writeJSON(w, http.StatusOK, price)
			return
		}
	}
	h.writeError(w, http.StatusNotFound, "Price not found")
}

func (h *HTTPServer) getLowestPriceByExchange(w http.ResponseWriter, exchange, symbol string, period time.Duration) {
	price, err := h.app.GetLowestPrice(exchange, symbol, period)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "Price not found")
		return
	}
	h.writeJSON(w, http.StatusOK, price)
}

// Average price handlers
func (h *HTTPServer) getAveragePrice(w http.ResponseWriter, symbol string, period time.Duration) {
	for _, exchCfg := range h.app.config.Exchanges {
		price, err := h.app.GetAveragePrice(exchCfg.Name, symbol, period)
		if err == nil {
			h.writeJSON(w, http.StatusOK, price)
			return
		}
	}
	h.writeError(w, http.StatusNotFound, "Price not found")
}

func (h *HTTPServer) getAveragePriceByExchange(w http.ResponseWriter, exchange, symbol string, period time.Duration) {
	price, err := h.app.GetAveragePrice(exchange, symbol, period)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "Price not found")
		return
	}
	h.writeJSON(w, http.StatusOK, price)
}

// Mode handlers
func (h *HTTPServer) switchToTestMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if err := h.app.SwitchMode(domain.TestMode); err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to switch mode")
		return
	}

	response := map[string]string{"mode": "test", "status": "switched"}
	h.writeJSON(w, http.StatusOK, response)
}

func (h *HTTPServer) switchToLiveMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if err := h.app.SwitchMode(domain.LiveMode); err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to switch mode")
		return
	}

	response := map[string]string{"mode": "live", "status": "switched"}
	h.writeJSON(w, http.StatusOK, response)
}

// Health handler
func (h *HTTPServer) getHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	health := h.app.GetHealth()
	h.writeJSON(w, http.StatusOK, health)
}

// Helper functions
func (h *HTTPServer) parsePeriod(period string) time.Duration {
	if period == "" {
		return 24 * time.Hour // Default
	}

	if len(period) < 2 {
		return 24 * time.Hour
	}

	valueStr := period[:len(period)-1]
	unit := strings.ToLower(period[len(period)-1:])

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 24 * time.Hour
	}

	switch unit {
	case "s":
		return time.Duration(value) * time.Second
	case "m":
		return time.Duration(value) * time.Minute
	case "h":
		return time.Duration(value) * time.Hour
	default:
		return 24 * time.Hour
	}
}

func (h *HTTPServer) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}

func (h *HTTPServer) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := map[string]string{"error": message}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode error response", "error", err)
	}
}

func (h *HTTPServer) Start() error {
	slog.Info("Starting HTTP server", "addr", h.server.Addr)
	return h.server.ListenAndServe()
}

func (h *HTTPServer) Shutdown(ctx context.Context) error {
	slog.Info("Shutting down HTTP server")
	return h.server.Shutdown(ctx)
}
