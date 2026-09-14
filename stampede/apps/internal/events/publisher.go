package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
)

type Publisher struct {
	client   *sns.Client
	topicArn string
}

func NewPublisher(client *sns.Client, topicArn string) *Publisher {
	return &Publisher{
		client:   client,
		topicArn: topicArn,
	}
}

func (p *Publisher) Publish(ctx context.Context, event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	_, err = p.client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(p.topicArn),
		Message:  aws.String(string(body)),
		MessageAttributes: map[string]snstypes.MessageAttributeValue{
			"event_type": {
				DataType:    aws.String("String"),
				StringValue: aws.String(string(event.Type)),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("publish event: %w", err)
	}

	return nil
}
