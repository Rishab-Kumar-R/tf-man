package sqsconsumer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/health"
)

type Handler func(ctx context.Context, msg types.Message) error

type Option func(*Consumer)

func WithHealthTracker(t *health.Tracker) Option {
	return func(c *Consumer) {
		c.health = t
	}
}

func WithHandlerTimeout(d time.Duration) Option {
	return func(c *Consumer) {
		c.handlerTimeout = d
	}
}

const defaultHandlerTimeout = 30 * time.Second

type Consumer struct {
	client         *sqs.Client
	queueURL       string
	concurrency    int
	handler        Handler
	logger         *slog.Logger
	health         *health.Tracker
	handlerTimeout time.Duration
}

func New(
	client *sqs.Client,
	queueURL string,
	concurrency int,
	handler Handler,
	logger *slog.Logger,
	opts ...Option,
) *Consumer {
	c := &Consumer{
		client:         client,
		queueURL:       queueURL,
		concurrency:    concurrency,
		handler:        handler,
		logger:         logger,
		handlerTimeout: defaultHandlerTimeout,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Consumer) Run(ctx context.Context) error {
	sem := make(chan struct{}, c.concurrency)
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		default:
		}

		out, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     10,
		})
		if err != nil {
			if ctx.Err() != nil {
				wg.Wait()
				return ctx.Err()
			}

			c.logger.Error("receive message failed", "error", err)

			select {
			case <-ctx.Done():
				wg.Wait()
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}

			continue
		}

		if c.health != nil {
			c.health.MarkHealthy()
		}

		for _, msg := range out.Messages {
			sem <- struct{}{}

			wg.Go(func() {
				defer func() { <-sem }()

				handlerCtx, cancel := context.WithTimeout(ctx, c.handlerTimeout)
				defer cancel()

				if err := c.handler(handlerCtx, msg); err != nil {
					c.logger.Error("message handler failed", "error", err, "message_id", aws.ToString(msg.MessageId))
					return
				}

				_, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(c.queueURL),
					ReceiptHandle: msg.ReceiptHandle,
				})
				if err != nil {
					c.logger.Error("delete message failed", "error", err, "message_id", aws.ToString(msg.MessageId))
				}
			})
		}
	}
}
