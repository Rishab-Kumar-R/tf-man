package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/events"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/logging"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/secrets"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/shutdown"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/sqsconsumer"
	"github.com/Rishab-Kumar-R/tf-man/stampede/order-service/internal/sqlcgen"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

//go:embed db/schema.sql
var schemaFS embed.FS

var ErrInsufficientStock = errors.New("insufficient stock")

type inventoryClient struct {
	baseURL string
	client  *http.Client
}

func newInventoryClient(baseURL string) *inventoryClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 50
	transport.IdleConnTimeout = 90 * time.Second

	return &inventoryClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Second, Transport: transport},
	}
}

type reserveRequest struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
	OrderID  string `json:"order_id"`
}

func (c *inventoryClient) Reserve(ctx context.Context, itemID string, quantity int, orderID string) error {
	const maxAttempts = 3
	backoff := 100 * time.Millisecond

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		retryable, err := c.doReserve(ctx, itemID, quantity, orderID)
		if err == nil {
			return nil
		}

		lastErr = err
		if !retryable || attempt == maxAttempts {
			return lastErr
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}

	return lastErr
}

func (c *inventoryClient) doReserve(ctx context.Context, itemID string, quantity int, orderID string) (retryable bool, err error) {
	body, err := json.Marshal(reserveRequest{
		ItemID:   itemID,
		Quantity: quantity,
		OrderID:  orderID,
	})
	if err != nil {
		return false, fmt.Errorf("marshal reserve request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/reserve", bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("build reserve request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return true, fmt.Errorf("call inventory service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
		return false, nil
	case resp.StatusCode == http.StatusConflict:
		return false, ErrInsufficientStock
	case resp.StatusCode >= 500:
		return true, fmt.Errorf("inventory service returned status %d", resp.StatusCode)
	default:
		return false, fmt.Errorf("inventory service returned unexpected status %d", resp.StatusCode)
	}
}

type server struct {
	pool         *pgxpool.Pool
	queries      *sqlcgen.Queries
	relayPool    *pgxpool.Pool
	relayQueries *sqlcgen.Queries
	publisher    *events.Publisher
	inventory    *inventoryClient
	logger       *slog.Logger
}

type createOrderRequest struct {
	UserID         string `json:"user_id"`
	ItemID         string `json:"item_id"`
	Quantity       int    `json:"quantity"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type createOrderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

func (s *server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.ItemID == "" || req.Quantity <= 0 {
		http.Error(w, "user_id, item_id, and a positive quantity are required", http.StatusBadRequest)
		return
	}

	orderID := uuid.New()
	if req.IdempotencyKey != "" {
		parsed, err := uuid.Parse(req.IdempotencyKey)
		if err != nil {
			http.Error(w, "idempotency_key must be a valid UUID", http.StatusBadRequest)
			return
		}
		orderID = parsed
	}

	if err := s.inventory.Reserve(ctx, req.ItemID, req.Quantity, orderID.String()); err != nil {
		if errors.Is(err, ErrInsufficientStock) {
			http.Error(w, "insufficient stock", http.StatusConflict)
			return
		}

		s.logger.Error("reserve stock failed", "error", err, "order_id", orderID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	event := events.Event{
		Type:      events.TypeOrderPlaced,
		UserID:    req.UserID,
		OrderID:   orderID.String(),
		Timestamp: time.Now().UTC(),
		Metadata: map[string]any{
			"item_id":  req.ItemID,
			"quantity": req.Quantity,
		},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		s.logger.Error("marshal outbox event failed", "error", err, "order_id", orderID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		s.logger.Error("begin transaction failed", "error", err, "order_id", orderID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.queries.WithTx(tx)

	order, err := qtx.CreateOrder(ctx, sqlcgen.CreateOrderParams{
		ID:       orderID,
		UserID:   req.UserID,
		ItemID:   req.ItemID,
		Quantity: int32(req.Quantity),
		Status:   "pending_payment",
	})
	if err != nil {
		s.logger.Error("insert order failed", "error", err, "order_id", orderID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := qtx.InsertOutboxEvent(ctx, sqlcgen.InsertOutboxEventParams{
		ID:        orderID,
		EventType: string(events.TypeOrderPlaced),
		Payload:   payload,
	}); err != nil {
		s.logger.Error("insert outbox event failed", "error", err, "order_id", orderID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("commit transaction failed", "error", err, "order_id", orderID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createOrderResponse{OrderID: order.ID.String(), Status: order.Status}); err != nil {
		s.logger.Error("encode response failed", "error", err, "order_id", orderID)
	}
}

func (s *server) runOutboxRelay(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.publishOutboxBatch(ctx)
		}
	}
}

func (s *server) publishOutboxBatch(ctx context.Context) {
	for range 20 {
		hadWork, err := s.publishNextOutboxEvent(ctx)
		if err != nil {
			s.logger.Error("process outbox event failed", "error", err)
		}
		if !hadWork {
			return
		}
	}
}

func (s *server) publishNextOutboxEvent(ctx context.Context) (bool, error) {
	tx, err := s.relayPool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.relayQueries.WithTx(tx)

	rows, err := qtx.ListUnpublishedOutboxEvents(ctx, 1)
	if err != nil {
		return false, fmt.Errorf("list unpublished outbox events: %w", err)
	}
	if len(rows) == 0 {
		return false, nil
	}
	row := rows[0]

	var evt events.Event
	if err := json.Unmarshal(row.Payload, &evt); err != nil {
		return true, fmt.Errorf("unmarshal outbox payload for %s: %w", row.ID, err)
	}

	publishCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.publisher.Publish(publishCtx, evt); err != nil {
		return true, fmt.Errorf("publish outbox event %s: %w", row.ID, err)
	}

	if err := qtx.MarkOutboxEventPublished(ctx, row.ID); err != nil {
		return true, fmt.Errorf("mark outbox event %s published: %w", row.ID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("commit transaction for %s: %w", row.ID, err)
	}

	return true, nil
}

func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *server) handleStatusEvent(ctx context.Context, msg sqstypes.Message) error {
	var evt events.Event
	if err := json.Unmarshal([]byte(*msg.Body), &evt); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	var status string
	switch evt.Type {
	case events.TypeOrderConfirmed:
		status = "confirmed"
	case events.TypePaymentFailed:
		status = "payment_failed"
	default:
		return nil
	}

	orderID, err := uuid.Parse(evt.OrderID)
	if err != nil {
		return fmt.Errorf("parse order id %q: %w", evt.OrderID, err)
	}

	if err := s.queries.UpdateOrderStatus(ctx, sqlcgen.UpdateOrderStatusParams{
		ID:     orderID,
		Status: status,
	}); err != nil {
		return fmt.Errorf("update order status: %w", err)
	}

	s.logger.Info("order status updated", "order_id", evt.OrderID, "status", status)
	return nil
}

func main() {
	logger := logging.New("order-service")

	ctx, cancel := shutdown.Context()
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Error("load aws config failed", "error", err)
		os.Exit(1)
	}

	creds, err := secrets.FetchDBCredentials(ctx, secretsmanager.NewFromConfig(cfg), os.Getenv("DB_SECRET_ARN"))
	if err != nil {
		logger.Error("fetch db credentials failed", "error", err)
		os.Exit(1)
	}

	dbPoolMaxConns := os.Getenv("DB_POOL_MAX_CONNS")
	if dbPoolMaxConns == "" {
		dbPoolMaxConns = "20"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=require&pool_max_conns=%s&pool_min_conns=2",
		creds.Username, creds.Password, creds.Host, creds.Port, creds.DBName, dbPoolMaxConns)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Error("connect to database failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	relayDSN := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=require&pool_max_conns=5&pool_min_conns=1",
		creds.Username, creds.Password, creds.Host, creds.Port, creds.DBName)

	relayPool, err := pgxpool.New(ctx, relayDSN)
	if err != nil {
		logger.Error("connect relay pool to database failed", "error", err)
		os.Exit(1)
	}
	defer relayPool.Close()

	schemaSQL, err := schemaFS.ReadFile("db/schema.sql")
	if err != nil {
		logger.Error("read embedded schema failed", "error", err)
		os.Exit(1)
	}

	if _, err := pool.Exec(ctx, string(schemaSQL)); err != nil {
		logger.Error("ensure schema failed", "error", err)
		os.Exit(1)
	}

	s := &server{
		pool:         pool,
		queries:      sqlcgen.New(pool),
		relayPool:    relayPool,
		relayQueries: sqlcgen.New(relayPool),
		publisher:    events.NewPublisher(sns.NewFromConfig(cfg), os.Getenv("SNS_TOPIC_ARN")),
		inventory:    newInventoryClient(os.Getenv("INVENTORY_SERVICE_URL")),
		logger:       logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", s.handleCreateOrder)
	mux.HandleFunc("GET /healthz", s.handleHealthz)

	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	statusConsumer := sqsconsumer.New(
		sqs.NewFromConfig(cfg),
		os.Getenv("ORDER_STATUS_QUEUE_URL"),
		5,
		s.handleStatusEvent,
		logger,
	)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := statusConsumer.Run(ctx); err != nil && ctx.Err() == nil {
			return fmt.Errorf("status consumer stopped unexpectedly: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		s.runOutboxRelay(gCtx)
		return nil
	})

	g.Go(func() error {
		logger.Info("order-service listening", "addr", httpServer.Addr)
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

	logger.Info("order-service shut down cleanly")
}
