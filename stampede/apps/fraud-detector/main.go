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
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

type detector struct {
	redis     *redis.Client
	dedup     *dedup.Checker
	publisher *events.Publisher
	threshold int64
	window    time.Duration
	logger    *slog.Logger
}

func (d *detector) handle(ctx context.Context, msg types.Message) error {
	var evt events.Event
	if err := json.Unmarshal([]byte(*msg.Body), &evt); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	if evt.Type == events.TypeFraudFlagged {
		return nil
	}

	msgID := aws.ToString(msg.MessageId)

	seen, err := d.dedup.Seen(ctx, "fraud:seen:"+msgID)
	if err != nil {
		return fmt.Errorf("dedup check failed: %w", err)
	}
	if seen {
		d.logger.Debug("skipping already-processed message", "message_id", msgID)
		return nil
	}

	key := "fraud:events:" + evt.UserID

	count, err := d.redis.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("incr redis counter: %w", err)
	}

	if count == 1 {
		if err := d.redis.Expire(ctx, key, d.window).Err(); err != nil {
			return fmt.Errorf("set expiry: %w", err)
		}
	}

	if count <= d.threshold {
		return nil
	}

	d.logger.Warn("fraud pattern detected", "user_id", evt.UserID, "count", count)

	return d.publisher.Publish(ctx, events.Event{
		Type:      events.TypeFraudFlagged,
		UserID:    evt.UserID,
		Timestamp: time.Now().UTC(),
		Metadata: map[string]any{
			"event_count":    count,
			"window_seconds": d.window.Seconds(),
		},
	})
}

func main() {
	logger := logging.New("fraud-detector")

	ctx, cancel := shutdown.Context()
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Error("load aws config failed", "error", err)
		os.Exit(1)
	}

	threshold, err := strconv.ParseInt(os.Getenv("FRAUD_THRESHOLD"), 10, 64)
	if err != nil {
		logger.Error("invalid FRAUD_THRESHOLD", "error", err)
		os.Exit(1)
	}

	windowSeconds, err := strconv.Atoi(os.Getenv("FRAUD_WINDOW_SECONDS"))
	if err != nil {
		logger.Error("invalid FRAUD_WINDOW_SECONDS", "error", err)
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

	d := &detector{
		redis:     redisClient,
		dedup:     dedup.NewChecker(redisClient, 24*time.Hour),
		publisher: events.NewPublisher(sns.NewFromConfig(cfg), os.Getenv("SNS_TOPIC_ARN")),
		threshold: threshold,
		window:    time.Duration(windowSeconds) * time.Second,
		logger:    logger,
	}

	tracker := health.NewTracker()

	consumer := sqsconsumer.New(
		sqs.NewFromConfig(cfg),
		os.Getenv("SQS_QUEUE_URL"),
		concurrency,
		d.handle,
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

	logger.Info("fraud-detector starting", "threshold", threshold, "window_seconds", windowSeconds)

	if err := g.Wait(); err != nil {
		logger.Error("service exited with error", "error", err)
		os.Exit(1)
	}

	logger.Info("fraud-detector shut down cleanly")
}
