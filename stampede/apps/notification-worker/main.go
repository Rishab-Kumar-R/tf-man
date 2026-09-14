package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/dedup"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/events"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/health"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/logging"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/shutdown"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/sqsconsumer"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

type notifier struct {
	dedup  *dedup.Checker
	logger *slog.Logger
}

func (n *notifier) handle(ctx context.Context, msg types.Message) error {
	var evt events.Event
	if err := json.Unmarshal([]byte(*msg.Body), &evt); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	msgID := aws.ToString(msg.MessageId)
	seen, err := n.dedup.Seen(ctx, "notification:seen:"+msgID)
	if err != nil {
		return fmt.Errorf("dedup check failed: %w", err)
	}
	if seen {
		n.logger.Debug("skipping already-processed message", "message_id", msgID)
		return nil
	}

	switch evt.Type {
	case events.TypeOrderConfirmed:
		n.logger.Info("simulated notification: order confirmation email",
			"user_id", evt.UserID, "order_id", evt.OrderID)
	case events.TypeFraudFlagged:
		n.logger.Warn("simulated notification: fraud alert",
			"user_id", evt.UserID, "metadata", evt.Metadata)
	case events.TypePaymentFailed:
		n.logger.Warn("simulated notification: payment failed",
			"user_id", evt.UserID, "order_id", evt.OrderID)
	default:
		n.logger.Debug("ignoring unhandled event type", "event_type", evt.Type)
	}

	return nil
}

func main() {
	logger := logging.New("notification-worker")

	ctx, cancel := shutdown.Context()
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Error("load aws config failed", "error", err)
		os.Exit(1)
	}

	concurrency, err := strconv.Atoi(os.Getenv("CONCURRENCY"))
	if err != nil {
		concurrency = 10
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:      os.Getenv("REDIS_ADDR"),
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	})

	n := &notifier{
		dedup:  dedup.NewChecker(redisClient, 24*time.Hour),
		logger: logger,
	}

	tracker := health.NewTracker()

	consumer := sqsconsumer.New(
		sqs.NewFromConfig(cfg),
		os.Getenv("SQS_QUEUE_URL"),
		concurrency,
		n.handle,
		logger,
		sqsconsumer.WithHealthTracker(tracker),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", tracker.Handler(30*time.Second))

	healthServer := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		logger.Info("health server listening", "addr", healthServer.Addr)
		if err := healthServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("health server failed: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return healthServer.Shutdown(shutdownCtx)
	})

	g.Go(func() error {
		if err := consumer.Run(ctx); err != nil && ctx.Err() == nil {
			return fmt.Errorf("consumer stopped unexpectedly: %w", err)
		}
		return nil
	})

	logger.Info("notification-worker starting")

	if err := g.Wait(); err != nil {
		logger.Error("service exited with error", "error", err)
		os.Exit(1)
	}

	logger.Info("notification-worker shut down cleanly")
}
