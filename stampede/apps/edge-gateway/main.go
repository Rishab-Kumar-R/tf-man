package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/logging"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/shutdown"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

type hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]chan []byte
	logger  *slog.Logger
}

func newHub(logger *slog.Logger) *hub {
	return &hub{
		clients: make(map[*websocket.Conn]chan []byte),
		logger:  logger,
	}
}

func (h *hub) register(conn *websocket.Conn) chan []byte {
	send := make(chan []byte, 16)

	h.mu.Lock()
	h.clients[conn] = send
	h.mu.Unlock()

	return send
}

func (h *hub) unregister(conn *websocket.Conn) {
	h.mu.Lock()

	if send, ok := h.clients[conn]; ok {
		close(send)
		delete(h.clients, conn)
	}

	h.mu.Unlock()
}

func (h *hub) broadcast(msg []byte) {
	h.mu.Lock()
	var slow []*websocket.Conn
	for conn, send := range h.clients {
		select {
		case send <- msg:
		default:
			slow = append(slow, conn)
		}
	}
	h.mu.Unlock()

	for _, conn := range slow {
		h.logger.Warn("dropping message for slow websocket client")
		_ = conn.Close()
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkOrigin,
}

func checkOrigin(r *http.Request) bool {
	allowed := os.Getenv("ALLOWED_ORIGIN")
	if allowed == "" {
		return true
	}
	return r.Header.Get("Origin") == allowed
}

func (h *hub) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	send := h.register(conn)

	go func() {
		defer func() {
			h.unregister(conn)
			_ = conn.Close()
		}()

		for msg := range send {
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	go func() {
		defer func() {
			h.unregister(conn)
			_ = conn.Close()
		}()

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (h *hub) subscribeStockUpdates(ctx context.Context, rdb *redis.Client) {
	for ctx.Err() == nil {
		if err := h.runSubscription(ctx, rdb); err != nil && ctx.Err() == nil {
			h.logger.Error("stock-updates subscription failed, retrying", "error", err)

			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (h *hub) runSubscription(ctx context.Context, rdb *redis.Client) error {
	pubsub := rdb.Subscribe(ctx, "stock-updates")
	defer func() { _ = pubsub.Close() }()

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return fmt.Errorf("stock-updates subscription channel closed")
			}
			h.broadcast([]byte(msg.Payload))
		}
	}
}

var proxyTransport = func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 200
	t.MaxIdleConnsPerHost = 50
	t.IdleConnTimeout = 90 * time.Second
	return t
}()

func newReverseProxy(targetBase string, rewritePath func(string) string, logger *slog.Logger) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(targetBase)
	if err != nil {
		return nil, fmt.Errorf("parse target url %q: %w", targetBase, err)
	}

	return &httputil.ReverseProxy{
		Transport: proxyTransport,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			if rewritePath != nil {
				pr.Out.URL.Path = rewritePath(pr.In.URL.Path)
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			logger.Error("backend proxy request failed", "error", err, "target", target.Host, "path", r.URL.Path)
			w.WriteHeader(http.StatusBadGateway)
		},
	}, nil
}

func main() {
	logger := logging.New("edge-gateway")

	ctx, cancel := shutdown.Context()
	defer cancel()

	redisClient := redis.NewClient(&redis.Options{
		Addr:      os.Getenv("REDIS_ADDR"),
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	})

	orderProxy, err := newReverseProxy(os.Getenv("ORDER_SERVICE_URL"), func(_ string) string {
		return "/orders"
	}, logger)
	if err != nil {
		logger.Error("configure order-service proxy failed", "error", err)
		os.Exit(1)
	}

	catalogProxy, err := newReverseProxy(os.Getenv("CATALOG_SERVICE_URL"), func(p string) string {
		return strings.Replace(p, "/catalog", "/products", 1)
	}, logger)
	if err != nil {
		logger.Error("configure catalog-service proxy failed", "error", err)
		os.Exit(1)
	}

	h := newHub(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", h.handleWS)
	mux.Handle("POST /checkout", orderProxy)
	mux.Handle("GET /catalog/search", catalogProxy)
	mux.Handle("GET /catalog/{id}", catalogProxy)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := redisClient.Ping(r.Context()).Err(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // must stay unbounded: this server also serves long-lived WebSocket connections
		IdleTimeout:  60 * time.Second,
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		h.subscribeStockUpdates(gCtx, redisClient)
		return nil
	})

	g.Go(func() error {
		logger.Info("edge-gateway listening", "addr", httpServer.Addr)
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

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("service exited with error", "error", err)
		os.Exit(1)
	}

	logger.Info("edge-gateway shut down cleanly")
}
