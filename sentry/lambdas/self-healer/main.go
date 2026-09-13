package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

func handler(ctx context.Context, snsEvent events.SNSEvent) error {
	cluster := os.Getenv("ECS_CLUSTER")
	service := os.Getenv("ECS_SERVICE")
	desiredCountStr := os.Getenv("DESIRED_COUNT")

	desiredCount, err := strconv.Atoi(desiredCountStr)
	if err != nil {
		return fmt.Errorf("invalid DESIRED_COUNT env var %q: %w", desiredCountStr, err)
	}

	for _, record := range snsEvent.Records {
		log.Printf("alarm message received: %s", record.SNS.Message)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load AWS config: %w", err)
	}

	client := ecs.NewFromConfig(cfg)

	describeOut, err := client.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []string{service},
	})
	if err != nil {
		return fmt.Errorf("describe services failed: %w", err)
	}

	if len(describeOut.Services) == 0 {
		return fmt.Errorf("service %s not found in cluster %s", service, cluster)
	}

	running := describeOut.Services[0].RunningCount
	log.Printf("current running count for %s: %d (target: %d)", service, running, desiredCount)

	if int(running) >= desiredCount {
		log.Printf("service already at or above desired count, no action taken")
		return nil
	}

	_, err = client.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      aws.String(cluster),
		Service:      aws.String(service),
		DesiredCount: aws.Int32(int32(desiredCount)),
	})
	if err != nil {
		return fmt.Errorf("update service failed: %w", err)
	}

	log.Printf("forced service %s in cluster %s back to desired count %d", service, cluster, desiredCount)
	return nil
}

func main() {
	lambda.Start(handler)
}
