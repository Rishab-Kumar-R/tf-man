package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
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
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

type worker struct {
	dedup       *dedup.Checker
	publisher   *events.Publisher
	logger      *slog.Logger
	failureRate float64
}

func (w *worker) handle(ctx context.Context, msg types.Message) error {
	var evt events.Event
	if err := json.Unmarshal([]byte(*msg.Body), &evt); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	if evt.Type != events.TypeOrderPlaced {
		return nil
	}

	msgID := aws.ToString(msg.MessageId)
	dedupKey := "payment:seen:" + msgID

	seen, err := w.dedup.Seen(ctx, dedupKey)
	if err != nil {
		return fmt.Errorf("dedup check failed: %w", err)
	}
	if seen {
		w.logger.Debug("skipping already-processed message", "message_id", msgID)
		return nil
	}

	var publishErr error
	if rand.Float64() < w.failureRate {
		w.logger.Warn("simulated payment failure", "order_id", evt.OrderID, "user_id", evt.UserID)
		publishErr = w.publisher.Publish(ctx, events.Event{
			Type:      events.TypePaymentFailed,
			UserID:    evt.UserID,
			OrderID:   evt.OrderID,
			Timestamp: time.Now().UTC(),
		})
	} else {
		w.logger.Info("payment processed", "order_id", evt.OrderID, "user_id", evt.UserID)
		publishErr = w.publisher.Publish(ctx, events.Event{
			Type:      events.TypeOrderConfirmed,
			UserID:    evt.UserID,
			OrderID:   evt.OrderID,
			Timestamp: time.Now().UTC(),
		})
	}

	if publishErr != nil {
		if err := w.dedup.Unclaim(ctx, dedupKey); err != nil {
			w.logger.Error("failed to release dedup claim after publish failure", "error", err, "message_id", msgID)
		}
		return publishErr
	}

	return nil
}

func main() {
	logger := logging.New("payment-worker")

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

	failureRate, err := strconv.ParseFloat(os.Getenv("SIMULATED_FAILURE_RATE"), 64)
	if err != nil {
		failureRate = 0.0
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:      os.Getenv("REDIS_ADDR"),
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	})

	w := &worker{
		dedup:       dedup.NewChecker(redisClient, 24*time.Hour),
		publisher:   events.NewPublisher(sns.NewFromConfig(cfg), os.Getenv("SNS_TOPIC_ARN")),
		logger:      logger,
		failureRate: failureRate,
	}

	tracker := health.NewTracker()

	consumer := sqsconsumer.New(
		sqs.NewFromConfig(cfg),
		os.Getenv("SQS_QUEUE_URL"),
		concurrency,
		w.handle,
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

	logger.Info("payment-worker starting", "simulated_failure_rate", failureRate)

	if err := g.Wait(); err != nil {
		logger.Error("service exited with error", "error", err)
		os.Exit(1)
	}

	logger.Info("payment-worker shut down cleanly")
}
