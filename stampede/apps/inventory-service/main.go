package main

import (
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/logging"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/shutdown"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

//go:embed redis/reserve.lua
var reserveScriptSrc string

var reserveScript = redis.NewScript(reserveScriptSrc)

type server struct {
	redis  *redis.Client
	logger *slog.Logger
}

type reserveRequest struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
	OrderID  string `json:"order_id"`
}

func (s *server) handleReserve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req reserveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ItemID == "" || req.OrderID == "" || req.Quantity <= 0 {
		http.Error(w, "item_id, order_id, and a positive quantity are required", http.StatusBadRequest)
		return
	}

	reservationKey := "reservation:" + req.OrderID
	stockKey := "stock:" + req.ItemID

	result, err := reserveScript.Run(ctx, s.redis, []string{reservationKey, stockKey}, req.Quantity, req.ItemID).Int()
	if err != nil {
		s.logger.Error("reserve script failed", "error", err, "order_id", req.OrderID, "item_id", req.ItemID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if result == 0 {
		http.Error(w, "insufficient stock", http.StatusConflict)
		return
	}

	s.logger.Info("stock reserved", "item_id", req.ItemID, "quantity", req.Quantity, "order_id", req.OrderID)
	w.WriteHeader(http.StatusOK)
}

type setStockRequest struct {
	Quantity int `json:"quantity"`
}

func (s *server) handleSetStock(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemID")
	if itemID == "" {
		http.Error(w, "item id is required", http.StatusBadRequest)
		return
	}

	var req setStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Quantity < 0 {
		http.Error(w, "quantity must not be negative", http.StatusBadRequest)
		return
	}

	if err := s.redis.Set(r.Context(), "stock:"+itemID, req.Quantity, 0).Err(); err != nil {
		s.logger.Error("set stock failed", "error", err, "item_id", itemID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := s.redis.Ping(r.Context()).Err(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func main() {
	logger := logging.New("inventory-service")

	ctx, cancel := shutdown.Context()
	defer cancel()

	redisClient := redis.NewClient(&redis.Options{
		Addr:      os.Getenv("REDIS_ADDR"),
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	})

	s := &server{redis: redisClient, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /reserve", s.handleReserve)
	mux.HandleFunc("POST /items/{itemID}/stock", s.handleSetStock)
	mux.HandleFunc("GET /healthz", s.handleHealthz)

	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		logger.Info("inventory-service listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server failed: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	})

	if err := g.Wait(); err != nil {
		logger.Error("service exited with error", "error", err)
		os.Exit(1)
	}

	logger.Info("inventory-service shut down cleanly")
}
