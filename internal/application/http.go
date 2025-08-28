// internal/application/http.go
package application

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"log/slog"
	"marketflow/internal/domain"
)

type HTTPServer struct {
	server *http.Server
	app    *Application
}

func NewHTTPServer(port int, app *Application) *HTTPServer {
	router := mux.NewRouter()

	httpServer := &HTTPServer{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: router,
		},
		app: app,
	}

	httpServer.setupRoutes(router)
	return httpServer
}

func (h *HTTPServer) setupRoutes(router *mux.Router) {
	// Price endpoints
	router.HandleFunc("/prices/latest/{symbol}", h.getLatestPrice).Methods("GET")
	router.HandleFunc("/prices/latest/{exchange}/{symbol}", h.getLatestPriceByExchange).Methods("GET")

	router.HandleFunc("/prices/highest/{symbol}", h.getHighestPrice).Methods("GET")
	router.HandleFunc("/prices/highest/{exchange}/{symbol}", h.getHighestPriceByExchange).Methods("GET")

	router.HandleFunc("/prices/lowest/{symbol}", h.getLowestPrice).Methods("GET")
	router.HandleFunc("/prices/lowest/{exchange}/{symbol}", h.getLowestPriceByExchange).Methods("GET")

	router.HandleFunc("/prices/average/{symbol}", h.getAveragePrice).Methods("GET")
	router.HandleFunc("/prices/average/{exchange}/{symbol}", h.getAveragePriceByExchange).Methods("GET")

	// Mode endpoints
	router.HandleFunc("/mode/test", h.switchToTestMode).Methods("POST")
	router.HandleFunc("/mode/live", h.switchToLiveMode).Methods("POST")

	// Health endpoint
	router.HandleFunc("/health", h.getHealth).Methods("GET")
}

func (h *HTTPServer) getLatestPrice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]

	// Получаем цену от любой биржи (берем первую найденную)
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

func (h *HTTPServer) getLatestPriceByExchange(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exchange := vars["exchange"]
	symbol := vars["symbol"]

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

func (h *HTTPServer) getHighestPrice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]
	period := h.parsePeriod(r.URL.Query().Get("period"))

	for _, exchCfg := range h.app.config.Exchanges {
		price, err := h.app.GetHighestPrice(exchCfg.Name, symbol, period)
		if err == nil {
			h.writeJSON(w, http.StatusOK, price)
			return
		}
	}

	h.writeError(w, http.StatusNotFound, "Price not found")
}

func (h *HTTPServer) getHighestPriceByExchange(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exchange := vars["exchange"]
	symbol := vars["symbol"]
	period := h.parsePeriod(r.URL.Query().Get("period"))

	price, err := h.app.GetHighestPrice(exchange, symbol, period)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "Price not found")
		return
	}

	h.writeJSON(w, http.StatusOK, price)
}

func (h *HTTPServer) getLowestPrice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]
	period := h.parsePeriod(r.URL.Query().Get("period"))

	for _, exchCfg := range h.app.config.Exchanges {
		price, err := h.app.GetLowestPrice(exchCfg.Name, symbol, period)
		if err == nil {
			h.writeJSON(w, http.StatusOK, price)
			return
		}
	}

	h.writeError(w, http.StatusNotFound, "Price not found")
}

func (h *HTTPServer) getLowestPriceByExchange(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exchange := vars["exchange"]
	symbol := vars["symbol"]
	period := h.parsePeriod(r.URL.Query().Get("period"))

	price, err := h.app.GetLowestPrice(exchange, symbol, period)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "Price not found")
		return
	}

	h.writeJSON(w, http.StatusOK, price)
}

func (h *HTTPServer) getAveragePrice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]
	period := h.parsePeriod(r.URL.Query().Get("period"))

	for _, exchCfg := range h.app.config.Exchanges {
		price, err := h.app.GetAveragePrice(exchCfg.Name, symbol, period)
		if err == nil {
			h.writeJSON(w, http.StatusOK, price)
			return
		}
	}

	h.writeError(w, http.StatusNotFound, "Price not found")
}

func (h *HTTPServer) getAveragePriceByExchange(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exchange := vars["exchange"]
	symbol := vars["symbol"]
	period := h.parsePeriod(r.URL.Query().Get("period"))

	price, err := h.app.GetAveragePrice(exchange, symbol, period)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "Price not found")
		return
	}

	h.writeJSON(w, http.StatusOK, price)
}

func (h *HTTPServer) switchToTestMode(w http.ResponseWriter, r *http.Request) {
	if err := h.app.SwitchMode(domain.TestMode); err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to switch mode")
		return
	}

	response := map[string]string{"mode": "test", "status": "switched"}
	h.writeJSON(w, http.StatusOK, response)
}

func (h *HTTPServer) switchToLiveMode(w http.ResponseWriter, r *http.Request) {
	if err := h.app.SwitchMode(domain.LiveMode); err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to switch mode")
		return
	}

	response := map[string]string{"mode": "live", "status": "switched"}
	h.writeJSON(w, http.StatusOK, response)
}

func (h *HTTPServer) getHealth(w http.ResponseWriter, r *http.Request) {
	health := h.app.GetHealth()
	h.writeJSON(w, http.StatusOK, health)
}

func (h *HTTPServer) parsePeriod(period string) time.Duration {
	if period == "" {
		return 24 * time.Hour // по умолчанию 24 часа
	}

	if len(period) < 2 {
		return 24 * time.Hour
	}

	valueStr := period[:len(period)-1]
	unit := period[len(period)-1:]

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 24 * time.Hour
	}

	switch strings.ToLower(unit) {
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
	w.WriteHeader(status) // ✅ исправлено
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}

func (h *HTTPServer) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status) // ✅ правильно
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
